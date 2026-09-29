package model

import (
	"context"
	"net/http"
	"testing"

	"github.com/zhimma/grove/internal/testkit"
	"github.com/zhimma/grove/pkg/errx"
)

func TestFindUserByIDRejectsDisabledUser(t *testing.T) {
	db := testkit.OpenDB(t, &User{})
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
