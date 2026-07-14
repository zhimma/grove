package docs

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/zhimma/grove/app/console/internal/router"
	"github.com/zhimma/grove/internal/config"
	"github.com/zhimma/grove/internal/docsui"
	"github.com/zhimma/grove/internal/provider"
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

func TestConsoleRouteContractMatchesGinRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := newConsoleContractTestConfig(t)
	p, err := provider.New(cfg, "console-contract", provider.WithAuth(), provider.WithStorage())
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	t.Cleanup(func() { _ = p.Close() })

	engine := gin.New()
	router.New(cfg, p).InstallToEngine(engine)
	document := spec(cfg)
	if err := docsui.CompareRoutes(engine.Routes(), document, "/console/v1"); err != nil {
		t.Fatal(err)
	}

	operation := document.Paths["/admins/{id}/status"]["put"]
	if operation.ID != "consoleUpdateAdminStatus" || operation.RequestBody == nil {
		t.Fatalf("unexpected admin status contract: %#v", operation)
	}
	if _, ok := operation.Responses["default"]; !ok {
		t.Fatal("console operation must document the error envelope")
	}
	upload := document.Paths["/storage/upload"]["post"]
	if _, ok := upload.RequestBody.Content["multipart/form-data"]; !ok {
		t.Fatalf("upload must document multipart body: %#v", upload.RequestBody)
	}
}

func newConsoleContractTestConfig(t *testing.T) *config.Config {
	t.Helper()
	return &config.Config{
		App:         config.AppConfig{Name: "grove", Env: "test"},
		ConsolePort: "8082",
		Log: config.LogConfig{
			Level:   "error",
			Path:    t.TempDir(),
			Console: false,
			Service: "console-contract-test",
		},
		JWT: config.JWTConfig{
			Secret:            "0123456789abcdef0123456789abcdef",
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
}
