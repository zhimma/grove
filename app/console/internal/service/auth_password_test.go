package service

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/zhimma/grove/internal/model"
	"github.com/zhimma/grove/pkg/database"
)

func TestChangePasswordClearsMustChangePassword(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/auth-password.db"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.ConsoleAdmin{}, &model.ConsoleSession{}); err != nil {
		t.Fatalf("migrate console admin: %v", err)
	}

	oldHash, err := bcrypt.GenerateFromPassword([]byte("old-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash old password: %v", err)
	}
	admin := model.ConsoleAdmin{
		Base:               model.Base{ID: "console-admin-root"},
		Account:            "root",
		Password:           string(oldHash),
		MustChangePassword: true,
		Status:             model.ConsoleAdminStatusActive,
	}
	if err := db.Create(&admin).Error; err != nil {
		t.Fatalf("create admin: %v", err)
	}
	now := time.Now()
	session := model.ConsoleSession{
		AuditBase:        model.AuditBase{ID: "console-session-password-change"},
		AdminID:          admin.ID,
		RefreshTokenHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		LastActiveAt:     now,
		ExpiresAt:        now.Add(time.Hour),
	}
	if err := db.Create(&session).Error; err != nil {
		t.Fatalf("create session: %v", err)
	}

	repo := database.NewRepoWithConnections(db, nil)
	service := NewAuthService(repo, nil, nil)
	if err := service.ChangePassword(context.Background(), ChangePasswordInput{
		AdminID:     admin.ID,
		OldPassword: "old-password",
		NewPassword: "new-password",
	}); err != nil {
		t.Fatalf("change password: %v", err)
	}

	var updated model.ConsoleAdmin
	if err := db.First(&updated, "id = ?", admin.ID).Error; err != nil {
		t.Fatalf("load updated admin: %v", err)
	}
	if updated.MustChangePassword {
		t.Fatal("must_change_password should be cleared after self-service password change")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(updated.Password), []byte("new-password")); err != nil {
		t.Fatalf("new password was not persisted: %v", err)
	}
	if err := db.First(&session, "id = ?", session.ID).Error; err != nil {
		t.Fatalf("load session: %v", err)
	}
	if session.RevokedAt == nil || session.RevokeReason != "password_changed" {
		t.Fatalf("password change must revoke sessions: %#v", session)
	}
}

func TestResetPasswordRevokesSessions(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/admin-reset-password.db"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.ConsoleAdmin{}, &model.ConsoleSession{}); err != nil {
		t.Fatalf("migrate models: %v", err)
	}
	admin := model.ConsoleAdmin{
		Base: model.Base{ID: "console-admin-reset"}, Account: "reset", Password: "old", Status: model.ConsoleAdminStatusActive,
	}
	if err := db.Create(&admin).Error; err != nil {
		t.Fatalf("create admin: %v", err)
	}
	now := time.Now()
	session := model.ConsoleSession{
		AuditBase: model.AuditBase{ID: "console-session-password-reset"}, AdminID: admin.ID,
		RefreshTokenHash: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		LastActiveAt:     now, ExpiresAt: now.Add(time.Hour),
	}
	if err := db.Create(&session).Error; err != nil {
		t.Fatalf("create session: %v", err)
	}

	service := NewAdminService(database.NewRepoWithConnections(db, nil), nil)
	if err := service.ResetPassword(context.Background(), ResetAdminPasswordInput{AdminID: admin.ID, Password: "new-password"}); err != nil {
		t.Fatalf("reset password: %v", err)
	}
	if err := db.First(&session, "id = ?", session.ID).Error; err != nil {
		t.Fatalf("load session: %v", err)
	}
	if session.RevokedAt == nil || session.RevokeReason != "password_reset" {
		t.Fatalf("password reset must revoke sessions: %#v", session)
	}
}
