//go:build integration

package service

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/zhimma/grove/internal/model"
	"github.com/zhimma/grove/pkg/auth"
	"github.com/zhimma/grove/pkg/database"
	"github.com/zhimma/grove/pkg/errx"
)

func TestPostgresConcurrentRefreshOnlySucceedsOnce(t *testing.T) {
	dsn := os.Getenv("GROVE_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("GROVE_TEST_POSTGRES_DSN is not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open PostgreSQL: %v", err)
	}
	if err := db.AutoMigrate(&model.ConsoleRole{}, &model.ConsoleAdmin{}, &model.ConsoleSession{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Exec("DROP TABLE IF EXISTS console_sessions").Error
		_ = db.Exec("DROP TABLE IF EXISTS console_admins").Error
		_ = db.Exec("DROP TABLE IF EXISTS console_roles").Error
	})
	role := model.ConsoleRole{
		Base: model.Base{ID: "role-postgres-session"}, Name: "Session Test", Code: "session-test", Status: model.ConsoleRoleStatusActive,
	}
	if err := db.Create(&role).Error; err != nil {
		t.Fatalf("create role: %v", err)
	}
	admin := model.ConsoleAdmin{
		Base: model.Base{ID: "admin-postgres-session"}, Account: "postgres-session", Password: "unused", RoleID: role.ID, Status: model.ConsoleAdminStatusActive,
	}
	if err := db.Create(&admin).Error; err != nil {
		t.Fatalf("create admin: %v", err)
	}
	manager, err := auth.NewManager("test-secret", "test-issuer", time.Hour, 24*time.Hour)
	if err != nil {
		t.Fatalf("new token manager: %v", err)
	}
	sessions := NewSessionService(database.NewConnectionsFromDBs(db, nil), manager)
	_, pair, err := sessions.Create(context.Background(), CreateSessionInput{AdminID: admin.ID})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	const workers = 8
	start := make(chan struct{})
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, rotateErr := sessions.Rotate(context.Background(), pair.RefreshToken)
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
			t.Fatalf("unexpected refresh error: %v", rotateErr)
		}
	}
	if success != 1 {
		t.Fatalf("expected one successful refresh, got %d", success)
	}
}
