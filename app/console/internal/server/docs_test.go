package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zhimma/grove/internal/config"
)

func TestConsoleDocsRoutes(t *testing.T) {
	cfg := &config.Config{
		App:         config.AppConfig{Name: "grove", Env: "test"},
		ConsolePort: "8082",
		Log: config.LogConfig{
			Level:   "error",
			Path:    t.TempDir(),
			Console: false,
			Service: "console-test",
		},
		JWT: config.JWTConfig{
			Secret:            "test-secret",
			Issuer:            "grove",
			AccessExpiryHours: 24,
		},
		Docs: config.DocsConfig{
			Enabled:     true,
			Title:       "Console Docs",
			Description: "Console docs",
			Version:     "1.0.0",
		},
	}

	app, cleanup, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("new console server: %v", err)
	}
	t.Cleanup(func() {
		if cleanup != nil {
			cleanup()
		}
	})

	pageReq := httptest.NewRequest(http.MethodGet, "/console/docs", nil)
	pageResp := httptest.NewRecorder()
	app.Router.ServeHTTP(pageResp, pageReq)
	if pageResp.Code != http.StatusOK {
		t.Fatalf("expected docs page 200, got %d", pageResp.Code)
	}

	openapiReq := httptest.NewRequest(http.MethodGet, "/console/docs/openapi.json", nil)
	openapiResp := httptest.NewRecorder()
	app.Router.ServeHTTP(openapiResp, openapiReq)
	if openapiResp.Code != http.StatusOK {
		t.Fatalf("expected openapi 200, got %d", openapiResp.Code)
	}
}
