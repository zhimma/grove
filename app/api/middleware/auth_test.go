package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/zhimma/grove/pkg/auth"
)

func TestUserAuthRejectsConsoleTokenOnAPISurface(t *testing.T) {
	gin.SetMode(gin.TestMode)
	manager, err := auth.NewManager(auth.Config{Secret: "test-secret", Issuer: "test-issuer", AccessExpiry: time.Hour})
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	pair, err := manager.GenerateAdminTokenPair("admin-1", "session-1", auth.UserTypeConsole)
	if err != nil {
		t.Fatalf("issue console token: %v", err)
	}

	engine := gin.New()
	engine.GET("/private", NewUserAuthSet(manager).Required(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/private", nil)
	req.Header.Set("Authorization", "Bearer "+pair.AccessToken)
	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, req)
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", resp.Code, resp.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["message"] != "访问令牌无效" {
		t.Fatalf("unexpected response: %#v", payload)
	}
}

func TestUserAuthOptionalDoesNotAuthenticateInvalidSurfaceToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	manager, err := auth.NewManager(auth.Config{Secret: "test-secret", Issuer: "test-issuer", AccessExpiry: time.Hour})
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	pair, err := manager.GenerateAdminTokenPair("admin-1", "session-1", auth.UserTypeConsole)
	if err != nil {
		t.Fatalf("issue console token: %v", err)
	}

	engine := gin.New()
	engine.GET("/optional", NewUserAuthSet(manager).Optional(), func(c *gin.Context) {
		if got := c.GetString("user_id"); got != "" {
			t.Errorf("unexpected user identity %q", got)
		}
		c.Status(http.StatusNoContent)
	})
	req := httptest.NewRequest(http.MethodGet, "/optional", nil)
	req.Header.Set("Authorization", "Bearer "+pair.AccessToken)
	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, req)
	if resp.Code != http.StatusNoContent {
		t.Fatalf("expected optional request to continue, got %d", resp.Code)
	}
}
