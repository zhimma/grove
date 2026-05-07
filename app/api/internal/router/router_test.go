package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/zhimma/grove/internal/config"
	appmiddleware "github.com/zhimma/grove/internal/middleware"
	"github.com/zhimma/grove/internal/provider"
)

func TestRouterPingAndProfile(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		App:  config.AppConfig{Name: "grove", Env: "test"},
		Port: "8080",
		Log: config.LogConfig{
			Level:   "error",
			Path:    t.TempDir(),
			Console: false,
			Service: "api-test",
		},
		JWT: config.JWTConfig{
			Secret:            "test-secret",
			Issuer:            "grove",
			AccessExpiryHours: 24,
		},
		API: config.APIConfig{
			Prefix: "/api/v1",
		},
	}

	p, err := provider.New(cfg, "api", provider.WithAuth())
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	t.Cleanup(func() { _ = p.Close() })

	engine := gin.New()
	engine.Use(appmiddleware.RequestID(), appmiddleware.RequestMeta("api"), appmiddleware.Recovery())
	New(cfg, p).InstallToEngine(engine)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ping?name=codex", nil)
	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
	if !strings.Contains(resp.Body.String(), "pong, codex") {
		t.Fatalf("unexpected ping response: %s", resp.Body.String())
	}

	token, err := p.TokenManager.IssueAccessToken("api-user")
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/profile", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp = httptest.NewRecorder()
	engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 for profile, got %d body=%s", resp.Code, resp.Body.String())
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode profile response: %v", err)
	}
	if int(payload["code"].(float64)) != 0 {
		t.Fatalf("expected success response, got %v", payload)
	}
}
