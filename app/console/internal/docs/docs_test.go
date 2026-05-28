package docs

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

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

	engine := gin.New()
	RegisterDocs(engine, cfg)

	pageReq := httptest.NewRequest(http.MethodGet, "/console/docs", nil)
	pageResp := httptest.NewRecorder()
	engine.ServeHTTP(pageResp, pageReq)
	if pageResp.Code != http.StatusOK {
		t.Fatalf("expected docs page 200, got %d", pageResp.Code)
	}

	openapiReq := httptest.NewRequest(http.MethodGet, "/console/docs/openapi.json", nil)
	openapiResp := httptest.NewRecorder()
	engine.ServeHTTP(openapiResp, openapiReq)
	if openapiResp.Code != http.StatusOK {
		t.Fatalf("expected openapi 200, got %d", openapiResp.Code)
	}
}