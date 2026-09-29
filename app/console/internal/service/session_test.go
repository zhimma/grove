package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/zhimma/grove/internal/model"
	"github.com/zhimma/grove/internal/testkit"
	"github.com/zhimma/grove/pkg/auth"
	"github.com/zhimma/grove/pkg/database"
	"github.com/zhimma/grove/pkg/errx"
	"github.com/zhimma/grove/pkg/pagination"
)

func TestSessionCreateStoresOnlyRefreshTokenHash(t *testing.T) {
	service, db, manager := newSessionTestService(t)

	session, pair, err := service.Create(context.Background(), CreateSessionInput{
		AdminID:    "console-admin-session-test",
		DeviceName: "Chrome on macOS",
		ClientIP:   "127.0.0.1",
		UserAgent:  "test-agent",
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if session.RefreshTokenHash == pair.RefreshToken || session.RefreshTokenHash != auth.HashToken(pair.RefreshToken) {
		t.Fatalf("refresh token was not hashed correctly: %#v", session)
	}
	var persisted model.ConsoleSession
	if err := db.First(&persisted, "id = ?", session.ID).Error; err != nil {
		t.Fatalf("load session: %v", err)
	}
	if persisted.RefreshTokenHash != auth.HashToken(pair.RefreshToken) {
		t.Fatalf("unexpected persisted hash %q", persisted.RefreshTokenHash)
	}
	claims, err := manager.ParseAccessToken(pair.AccessToken)
	if err != nil {
		t.Fatalf("parse access token: %v", err)
	}
	if claims.SessionID != session.ID || claims.AdminID != session.AdminID {
		t.Fatalf("access token does not identify session: %+v", claims)
	}
}

func TestSessionRotateInvalidatesOldRefreshToken(t *testing.T) {
	service, _, _ := newSessionTestService(t)
	_, pair, err := service.Create(context.Background(), CreateSessionInput{AdminID: "console-admin-session-test"})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	rotated, err := service.Rotate(context.Background(), pair.RefreshToken)
	if err != nil {
		t.Fatalf("rotate refresh token: %v", err)
	}
	if rotated.RefreshToken == pair.RefreshToken {
		t.Fatal("refresh token was not rotated")
	}
	if _, err := service.Rotate(context.Background(), pair.RefreshToken); err == nil || errx.Normalize(err).Code != "invalid_refresh_token" {
		t.Fatalf("old refresh token must be rejected, got %v", err)
	}
}

func TestSessionConcurrentRotateOnlySucceedsOnce(t *testing.T) {
	service, _, _ := newSessionTestService(t)
	_, pair, err := service.Create(context.Background(), CreateSessionInput{AdminID: "console-admin-session-test"})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, rotateErr := service.Rotate(context.Background(), pair.RefreshToken)
			errs <- rotateErr
		}()
	}
	close(start)
	wg.Wait()
	close(errs)

	var success int
	for rotateErr := range errs {
		if rotateErr == nil {
			success++
			continue
		}
		if errx.Normalize(rotateErr).Code != "invalid_refresh_token" {
			t.Fatalf("unexpected rotate error: %v", rotateErr)
		}
	}
	if success != 1 {
		t.Fatalf("expected one successful rotation, got %d", success)
	}
}

func TestSessionRevokeInvalidatesAccessSession(t *testing.T) {
	service, _, _ := newSessionTestService(t)
	session, _, err := service.Create(context.Background(), CreateSessionInput{AdminID: "console-admin-session-test"})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if _, err := service.Validate(context.Background(), session.AdminID, session.ID); err != nil {
		t.Fatalf("validate active session: %v", err)
	}
	if err := service.Revoke(context.Background(), session.ID, "logout"); err != nil {
		t.Fatalf("revoke session: %v", err)
	}
	if _, err := service.Validate(context.Background(), session.AdminID, session.ID); err == nil {
		t.Fatal("revoked session must be rejected")
	}
}

func newSessionTestService(t *testing.T) (*SessionService, *gorm.DB, *auth.Tokens) {
	t.Helper()
	db := testkit.OpenDB(t, &model.ConsoleRole{}, &model.ConsoleAdmin{}, &model.ConsoleSession{})
	admin := model.ConsoleAdmin{
		Base:     model.Base{ID: "console-admin-session-test"},
		Account:  "session-test",
		Password: "unused",
		Status:   model.ConsoleAdminStatusActive,
	}
	if err := db.Create(&admin).Error; err != nil {
		t.Fatalf("create admin: %v", err)
	}
	manager, err := auth.NewTokens(auth.Config{Secret: "test-secret", Issuer: "test-issuer", AccessExpiry: time.Hour, RefreshExpiry: 24 * time.Hour})
	if err != nil {
		t.Fatalf("new token manager: %v", err)
	}
	return NewSessionService(database.NewConnectionsFromDBs(db, nil), manager, pagination.Policy{}), db, manager
}
