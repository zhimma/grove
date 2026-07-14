package service

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/zhimma/grove/internal/model"
	"github.com/zhimma/grove/pkg/auth"
	"github.com/zhimma/grove/pkg/database"
	"github.com/zhimma/grove/pkg/errx"
	"github.com/zhimma/grove/pkg/ratelimit"
	"github.com/zhimma/grove/pkg/request"
)

func TestLoginUsesSameErrorForMissingAccountAndWrongPassword(t *testing.T) {
	service := newLoginTestService(t, 5)

	missingErr := loginWithIP(service, "missing", "wrong", "192.0.2.1")
	wrongErr := loginWithIP(service, "admin", "wrong", "192.0.2.2")
	missing := errx.Normalize(missingErr)
	wrong := errx.Normalize(wrongErr)
	if missing.HTTPStatus != http.StatusUnauthorized || missing.Code != "invalid_credentials" {
		t.Fatalf("unexpected missing-account error: %#v", missing)
	}
	if wrong.HTTPStatus != missing.HTTPStatus || wrong.Code != missing.Code || wrong.Message != missing.Message {
		t.Fatalf("credential failures must use identical semantics: missing=%#v wrong=%#v", missing, wrong)
	}
}

func TestInvalidLoginPasswordHashUsesProductionCost(t *testing.T) {
	cost, err := bcrypt.Cost(invalidLoginPasswordHash)
	if err != nil {
		t.Fatalf("invalid-login password hash is malformed: %v", err)
	}
	if cost != bcrypt.DefaultCost {
		t.Fatalf("invalid-login password hash cost = %d, expected %d", cost, bcrypt.DefaultCost)
	}
}

func TestLoginLocksAfterFailureLimit(t *testing.T) {
	service := newLoginTestService(t, 2)

	if err := loginWithIP(service, "admin", "wrong", "192.0.2.1"); errx.Normalize(err).Code != "invalid_credentials" {
		t.Fatalf("first failure should remain a credential error: %v", err)
	}
	locked := errx.Normalize(loginWithIP(service, "admin", "wrong", "192.0.2.1"))
	if locked.HTTPStatus != http.StatusTooManyRequests || locked.Code != "login_locked" {
		t.Fatalf("second failure should lock login: %#v", locked)
	}
	if retry, ok := locked.Data["retry_after"].(int64); !ok || retry < 1 {
		t.Fatalf("locked error must include retry_after seconds: %#v", locked.Data)
	}
}

func TestSuccessfulLoginClearsFailureState(t *testing.T) {
	service := newLoginTestService(t, 2)

	if err := loginWithIP(service, "admin", "wrong", "192.0.2.1"); errx.Normalize(err).Code != "invalid_credentials" {
		t.Fatalf("first failure should remain a credential error: %v", err)
	}
	ctx := request.WithRequestMeta(context.Background(), request.RequestMeta{ClientIP: "192.0.2.1"})
	if _, err := service.Login(ctx, LoginInput{Account: "admin", Password: "correct-password"}); err != nil {
		t.Fatalf("successful login: %v", err)
	}
	if err := loginWithIP(service, "admin", "wrong", "192.0.2.1"); errx.Normalize(err).Code != "invalid_credentials" {
		t.Fatalf("failure after successful login must start a new sequence: %v", err)
	}
}

func TestLoginFailureStateIsIsolatedByAccountAndIP(t *testing.T) {
	service := newLoginTestService(t, 1)

	locked := errx.Normalize(loginWithIP(service, "missing", "wrong", "192.0.2.1"))
	if locked.Code != "login_locked" {
		t.Fatalf("expected missing account key to lock: %#v", locked)
	}

	for _, clientIP := range []string{"192.0.2.1", "192.0.2.2"} {
		ctx := request.WithRequestMeta(context.Background(), request.RequestMeta{ClientIP: clientIP})
		if _, err := service.Login(ctx, LoginInput{Account: "admin", Password: "correct-password"}); err != nil {
			t.Fatalf("admin login from %s must not share missing-account lock: %v", clientIP, err)
		}
	}
}

func newLoginTestService(t *testing.T, failureLimit int) *AuthService {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/auth-login.db"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&model.ConsoleRole{},
		&model.ConsoleAdmin{},
		&model.ConsoleSession{},
		&model.ConsoleLoginLog{},
	); err != nil {
		t.Fatalf("migrate login models: %v", err)
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	admin := model.ConsoleAdmin{
		Base:     model.Base{ID: "console-admin-login"},
		Account:  "admin",
		Password: string(hashed),
		Status:   model.ConsoleAdminStatusActive,
	}
	if err := db.Create(&admin).Error; err != nil {
		t.Fatalf("create admin: %v", err)
	}
	manager, err := auth.NewManager("test-secret", "test-issuer", time.Hour, 24*time.Hour)
	if err != nil {
		t.Fatalf("new token manager: %v", err)
	}
	guard := ratelimit.NewLocalLoginGuard(ratelimit.LoginConfig{
		AttemptsPerMinute: 600,
		Burst:             100,
		FailureLimit:      failureLimit,
		LockDuration:      time.Minute,
	})
	return NewAuthService(database.NewRepoWithConnections(db, nil), nil, manager, guard)
}

func loginWithIP(service *AuthService, account, password, clientIP string) error {
	ctx := request.WithRequestMeta(context.Background(), request.RequestMeta{ClientIP: clientIP})
	_, err := service.Login(ctx, LoginInput{Account: account, Password: password})
	return err
}
