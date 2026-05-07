package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/zhimma/grove/internal/config"
)

func TestNewCoreServerRequiresConfig(t *testing.T) {
	core, cleanup, err := NewCoreServer(nil, "api", "8080")
	if err == nil {
		t.Fatal("expected error when config is nil")
	}
	if core != nil {
		t.Fatal("expected nil core server when config is nil")
	}
	if cleanup != nil {
		t.Fatal("expected nil cleanup when config is nil")
	}
}

func TestNewCoreServerRegistersHealthCheck(t *testing.T) {
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
		Server: config.ServerConfig{
			ReadTimeout:     5,
			WriteTimeout:    5,
			ShutdownTimeout: 5,
			MaxHeaderBytes:  1 << 20,
		},
	}

	core, cleanup, err := NewCoreServer(cfg, "api", "8080")
	if err != nil {
		t.Fatalf("new core server: %v", err)
	}
	t.Cleanup(cleanup)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	resp := httptest.NewRecorder()
	core.Router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	var payload map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode health response: %v", err)
	}
	if payload["status"] != "ok" {
		t.Fatalf("unexpected status: %#v", payload["status"])
	}
	if payload["service"] != "api" {
		t.Fatalf("unexpected service: %#v", payload["service"])
	}
}
