package service

import (
	"context"
	"net/http"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/zhimma/grove/internal/model"
	"github.com/zhimma/grove/pkg/database"
	"github.com/zhimma/grove/pkg/errx"
	"github.com/zhimma/grove/pkg/pagination"
	"github.com/zhimma/grove/pkg/secretbox"
)

func TestSystemConfigSecretLifecycleEncryptsMasksAndResolves(t *testing.T) {
	service, db := newSystemConfigSecretService(t, true)
	created, err := service.CreateConfig(context.Background(), CreateSystemConfigInput{
		ConfigGroup:  "integration",
		ConfigKey:    "webhook_token",
		Name:         "Webhook Token",
		ValueType:    "string",
		Value:        " secret-current ",
		DefaultValue: " secret-default ",
		IsEditable:   true,
		IsSecret:     true,
	})
	if err != nil {
		t.Fatalf("create secret config: %v", err)
	}
	assertMaskedSystemConfig(t, created)

	var persisted model.SystemConfig
	if err := db.First(&persisted, "id = ?", created.ID).Error; err != nil {
		t.Fatalf("load persisted config: %v", err)
	}
	if persisted.Value == " secret-current " || persisted.DefaultValue == " secret-default " {
		t.Fatalf("secret values were stored in plaintext: %#v", persisted)
	}
	if persisted.Value == SecretMask || persisted.DefaultValue == SecretMask {
		t.Fatal("mask must never be persisted")
	}

	resolved, err := service.ResolveEffectiveValue(context.Background(), "integration", "webhook_token")
	if err != nil || resolved != " secret-current " {
		t.Fatalf("resolve current secret: value=%q err=%v", resolved, err)
	}

	listed, err := service.ListConfigs(context.Background(), ListSystemConfigsInput{Request: pagination.Request{ListAll: true}})
	if err != nil {
		t.Fatalf("list configs: %v", err)
	}
	if len(listed.List) != 1 {
		t.Fatalf("expected one config, got %d", len(listed.List))
	}
	assertMaskedSystemConfig(t, &listed.List[0])

	originalCiphertext := persisted.Value
	kept, err := service.UpdateConfigByID(context.Background(), UpdateSystemConfigByIDInput{
		ID:         created.ID,
		Value:      "must-not-be-used",
		KeepSecret: true,
	})
	if err != nil {
		t.Fatalf("keep secret value: %v", err)
	}
	assertMaskedSystemConfig(t, kept)
	if err := db.First(&persisted, "id = ?", created.ID).Error; err != nil {
		t.Fatalf("reload kept config: %v", err)
	}
	if persisted.Value != originalCiphertext {
		t.Fatal("keep_secret changed persisted ciphertext")
	}

	updated, err := service.UpdateConfigByID(context.Background(), UpdateSystemConfigByIDInput{
		ID:    created.ID,
		Value: " secret-replaced ",
	})
	if err != nil {
		t.Fatalf("replace secret value: %v", err)
	}
	assertMaskedSystemConfig(t, updated)
	if resolved, err := service.ResolveEffectiveValue(context.Background(), "integration", "webhook_token"); err != nil || resolved != " secret-replaced " {
		t.Fatalf("resolve replaced secret: value=%q err=%v", resolved, err)
	}

	cleared, err := service.UpdateConfigByID(context.Background(), UpdateSystemConfigByIDInput{ID: created.ID, Value: ""})
	if err != nil {
		t.Fatalf("clear secret value: %v", err)
	}
	if cleared.Value != "" || cleared.DefaultValue != SecretMask {
		t.Fatalf("cleared config should expose only masked default: %#v", cleared)
	}
	if resolved, err := service.ResolveEffectiveValue(context.Background(), "integration", "webhook_token"); err != nil || resolved != " secret-default " {
		t.Fatalf("resolve default secret: value=%q err=%v", resolved, err)
	}
}

func TestSystemConfigSecretRequiresEncryptionKey(t *testing.T) {
	service, _ := newSystemConfigSecretService(t, false)
	_, err := service.CreateConfig(context.Background(), CreateSystemConfigInput{
		ConfigGroup: "integration",
		ConfigKey:   "token",
		Name:        "Token",
		Value:       "secret",
		IsSecret:    true,
	})
	httpErr := errx.Normalize(err)
	if httpErr.HTTPStatus != http.StatusServiceUnavailable || httpErr.Code != "config_encryption_unavailable" {
		t.Fatalf("unexpected missing-key error: %#v", httpErr)
	}
}

func TestSystemConfigRejectsInfrastructureSecrets(t *testing.T) {
	service, _ := newSystemConfigSecretService(t, true)
	for _, input := range []CreateSystemConfigInput{
		{ConfigGroup: "jwt", ConfigKey: "issuer_secret", Name: "JWT", Value: "secret", IsSecret: true},
		{ConfigGroup: "application", ConfigKey: "db_password", Name: "DB", Value: "secret", IsSecret: true},
	} {
		_, err := service.CreateConfig(context.Background(), input)
		if errx.Normalize(err).Code != "infrastructure_secret_not_allowed" {
			t.Fatalf("infrastructure secret must be rejected: input=%#v err=%v", input, err)
		}
	}
}

func TestSystemConfigNonSecretRemainsPlainWithoutEncryptionKey(t *testing.T) {
	service, db := newSystemConfigSecretService(t, false)
	created, err := service.CreateConfig(context.Background(), CreateSystemConfigInput{
		ConfigGroup: "platform",
		ConfigKey:   "site_name",
		Name:        "Site Name",
		Value:       "Grove",
		IsEditable:  true,
	})
	if err != nil {
		t.Fatalf("create plain config: %v", err)
	}
	if created.Value != "Grove" || created.IsSecret {
		t.Fatalf("plain config changed unexpectedly: %#v", created)
	}
	var persisted model.SystemConfig
	if err := db.First(&persisted, "id = ?", created.ID).Error; err != nil || persisted.Value != "Grove" {
		t.Fatalf("plain config persistence: %#v err=%v", persisted, err)
	}
}

func newSystemConfigSecretService(t *testing.T, withBox bool) (*SystemConfigService, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/system-config-secret.db"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.SystemConfig{}); err != nil {
		t.Fatalf("migrate system config: %v", err)
	}
	dbs := database.NewConnectionsFromDBs(db, nil)
	if !withBox {
		return NewSystemConfigService(dbs, nil, pagination.Policy{}), db
	}
	box, err := secretbox.New("0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("new secret box: %v", err)
	}
	return NewSystemConfigService(dbs, box, pagination.Policy{}), db
}

func assertMaskedSystemConfig(t *testing.T, config *model.SystemConfig) {
	t.Helper()
	if !config.IsSecret || config.Value != SecretMask || config.DefaultValue != SecretMask {
		t.Fatalf("secret config is not masked: %#v", config)
	}
}
