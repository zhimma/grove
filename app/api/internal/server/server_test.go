package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zhimma/grove/internal/config"
)

func TestAPIDocsRoutes(t *testing.T) {
	cfg := &config.Config{
		App:  config.AppConfig{Name: "grove", Env: "test"},
		Port: "8081",
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
		Docs: config.DocsConfig{
			Enabled:     true,
			Title:       "API Docs",
			Description: "API docs",
			Version:     "1.0.0",
			BasePath:    "/api/v1",
		},
		API: config.APIConfig{
			Prefix: "/api/v1",
		},
		Storage: config.StorageConfig{
			Default: "local",
			Disks: map[string]config.StorageDiskConfig{
				"local": {
					Driver:  "local",
					Root:    t.TempDir(),
					BaseURL: "/storage",
				},
			},
		},
	}

	app, cleanup, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("new api server: %v", err)
	}
	t.Cleanup(func() {
		if cleanup != nil {
			cleanup()
		}
	})

	pageReq := httptest.NewRequest(http.MethodGet, "/docs", nil)
	pageResp := httptest.NewRecorder()
	app.Router.ServeHTTP(pageResp, pageReq)
	if pageResp.Code != http.StatusOK {
		t.Fatalf("expected docs page 200, got %d", pageResp.Code)
	}

	openapiReq := httptest.NewRequest(http.MethodGet, "/docs/openapi.json", nil)
	openapiResp := httptest.NewRecorder()
	app.Router.ServeHTTP(openapiResp, openapiReq)
	if openapiResp.Code != http.StatusOK {
		t.Fatalf("expected openapi 200, got %d", openapiResp.Code)
	}
}