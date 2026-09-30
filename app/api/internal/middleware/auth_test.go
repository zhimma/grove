package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/zhimma/grove/pkg/auth"
	"github.com/zhimma/grove/pkg/request"
)

func TestUserAuthRequiresBearerScheme(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tokens, err := auth.NewTokens(auth.Config{Secret: "test-secret", Issuer: "test-issuer", AccessExpiry: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	token, err := tokens.IssueAccessToken("user-1")
	if err != nil {
		t.Fatal(err)
	}
	engine := gin.New()
	engine.GET("/private", NewUserAuthSet(tokens).Required(), func(c *gin.Context) {
		if request.UserID(c) != "user-1" {
			t.Error("authenticated identity was not preserved")
		}
		c.Status(http.StatusNoContent)
	})
	engine.GET("/optional", NewUserAuthSet(tokens).Optional(), func(c *gin.Context) {
		if request.UserID(c) != "" {
			t.Error("invalid scheme authenticated an optional request")
		}
		c.Status(http.StatusNoContent)
	})
	for _, tc := range []struct {
		path, header string
		status       int
	}{
		{"/private", "bEaReR " + token, http.StatusNoContent},
		{"/private", token, http.StatusUnauthorized},
		{"/private", "Basic " + token, http.StatusUnauthorized},
		{"/private", "Bearer ", http.StatusUnauthorized},
		{"/optional", "Basic " + token, http.StatusNoContent},
	} {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		req.Header.Set("Authorization", tc.header)
		resp := httptest.NewRecorder()
		engine.ServeHTTP(resp, req)
		if resp.Code != tc.status {
			t.Fatalf("%s expected %d, got %d", tc.path, tc.status, resp.Code)
		}
	}
}

func TestUserAuthRejectsConsoleTokenOnAPISurface(t *testing.T) {
	gin.SetMode(gin.TestMode)
	manager, err := auth.NewTokens(auth.Config{Secret: "test-secret", Issuer: "test-issuer", AccessExpiry: time.Hour})
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
	manager, err := auth.NewTokens(auth.Config{Secret: "test-secret", Issuer: "test-issuer", AccessExpiry: time.Hour})
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
