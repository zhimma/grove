package model

import (
	"context"
	"net/http"
	"testing"

	"github.com/zhimma/grove/pkg/errx"
)

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
