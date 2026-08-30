package model

import (
	"context"
	"net/http"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/zhimma/grove/pkg/errx"
)

func TestFindUserByIDRejectsDisabledUser(t *testing.T) {
	db := openUserModelTestDB(t)
	user := User{Base: Base{ID: "disabled-user"}, Name: "Disabled", Email: "disabled@example.com", Status: UserStatusDisabled}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create disabled user: %v", err)
	}

	_, err := FindUserByID(context.Background(), db, user.ID)
	httpErr := errx.Normalize(err)
	if httpErr.HTTPStatus != http.StatusForbidden || httpErr.Code != "user_disabled" {
		t.Fatalf("unexpected disabled user error: %#v", httpErr)
	}
}

func openUserModelTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/user.db"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&User{}); err != nil {
		t.Fatalf("migrate user: %v", err)
	}
	return db
}

func TestFindUserByIDRequiresDatabase(t *testing.T) {
	user, err := FindUserByID(context.Background(), nil, "api-user")
	if user != nil {
		t.Fatalf("missing database must not return a virtual user: %#v", user)
	}
	httpErr := errx.Normalize(err)
	if httpErr.HTTPStatus != http.StatusServiceUnavailable {
		t.Fatalf("expected service unavailable, got %#v", httpErr)
	}
}
