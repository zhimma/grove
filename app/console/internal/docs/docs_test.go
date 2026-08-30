package docs

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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

	operationList := document.Paths["/logs/operations"]["get"]
	if operationList.ID != "consoleListOperationLogs" {
		t.Fatalf("unexpected operation log list operationId: %q", operationList.ID)
	}
	if _, ok := document.Paths["/logs/operations/{id}"]["get"]; !ok {
		t.Fatal("operation log detail route must be documented")
	}
	if _, ok := document.Paths["/logs/logins"]["get"]; !ok {
		t.Fatal("login log list route must be documented")
	}
	operationSchema := document.Components.Schemas["ConsoleListOperationLogsResponse"]
	if _, ok := operationSchema.Properties["list"]; !ok {
		t.Fatalf("operation log response must expose list: %#v", operationSchema)
	}
	if _, ok := operationSchema.Properties["meta"]; !ok {
		t.Fatalf("operation log response must expose meta: %#v", operationSchema)
	}
	seenSuccess := false
	for _, parameter := range operationList.Parameters {
		if parameter.Name == "success" {
			seenSuccess = true
			if parameter.Schema.Type != "boolean" {
				t.Fatalf("success filter must be boolean, got %#v", parameter.Schema)
			}
		}
		if parameter.Name == "status" {
			t.Fatal("log contract must not expose legacy status filter")
		}
	}
	if !seenSuccess {
		t.Fatal("operation log contract must expose success filter")
	}
}

func TestConsoleFrontendContractMatchesOpenAPI(t *testing.T) {
	frontendPath := filepath.Join("..", "..", "..", "..", "web", "admin-vben", "apps", "console", "src", "api", "console-contract.json")
	raw, err := os.ReadFile(frontendPath)
	if err != nil {
		t.Fatalf("read frontend API contract: %v", err)
	}
	var contract struct {
		BasePath   string `json:"basePath"`
		Operations []struct {
			OperationID string `json:"operationId"`
			Method      string `json:"method"`
			Path        string `json:"path"`
		} `json:"operations"`
	}
	if err := json.Unmarshal(raw, &contract); err != nil {
		t.Fatalf("decode frontend API contract: %v", err)
	}
	if contract.BasePath != "/console/v1" {
		t.Fatalf("frontend contract basePath = %q, want /console/v1", contract.BasePath)
	}
	if len(contract.Operations) == 0 {
		t.Fatal("frontend API contract must contain operations")
	}
	document := spec(newConsoleContractTestConfig(t))
	seen := make(map[string]struct{}, len(contract.Operations))
	for _, entry := range contract.Operations {
		if entry.OperationID == "" || entry.Path == "" || entry.Method == "" {
			t.Fatalf("frontend contract contains incomplete operation: %#v", entry)
		}
		key := strings.ToUpper(entry.Method) + " " + entry.Path
		if _, exists := seen[key]; exists {
			t.Fatalf("frontend contract contains duplicate operation: %s", key)
		}
		seen[key] = struct{}{}
		operation, ok := document.Paths[entry.Path][strings.ToLower(entry.Method)]
		if !ok {
			t.Fatalf("frontend operation %s is absent from OpenAPI", key)
		}
		if operation.ID != entry.OperationID {
			t.Fatalf("frontend operation %s has operationId %q, OpenAPI has %q", key, entry.OperationID, operation.ID)
		}
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
