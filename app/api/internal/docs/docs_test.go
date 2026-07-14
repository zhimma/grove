package docs

import (
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/zhimma/grove/app/api/internal/router"
	"github.com/zhimma/grove/internal/config"
	"github.com/zhimma/grove/internal/docsui"
	"github.com/zhimma/grove/internal/provider"
)

func TestSpecOnlyIncludesDemoPathsWhenExplicitlyEnabledOutsideProduction(t *testing.T) {
	cfg := newContractTestConfig(t, "development")
	assertDemoPathPresence(t, spec(cfg), false)

	cfg.Demo.Enabled = true
	assertDemoPathPresence(t, spec(cfg), true)

	cfg.App.Env = "production"
	assertDemoPathPresence(t, spec(cfg), false)
}

func TestSpecUsesRegisteredAPIPrefixAsServerURL(t *testing.T) {
	cfg := newContractTestConfig(t, "test")
	cfg.API.Prefix = "/custom/v2"
	cfg.Docs.BasePath = "/stale/v1"
	document := spec(cfg)
	if len(document.Servers) != 1 || document.Servers[0].URL != "/custom/v2" {
		t.Fatalf("expected API prefix server URL, got %#v", document.Servers)
	}
}

func TestAPIRouteContractMatchesGinRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, testCase := range []struct {
		name        string
		env         string
		demoEnabled bool
	}{
		{name: "demo disabled", env: "test", demoEnabled: false},
		{name: "demo enabled", env: "test", demoEnabled: true},
		{name: "production ignores demo", env: "production", demoEnabled: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := newContractTestConfig(t, testCase.env)
			cfg.Demo.Enabled = testCase.demoEnabled
			p, err := provider.New(cfg, "api-contract", provider.WithAuth())
			if err != nil {
				t.Fatalf("new provider: %v", err)
			}
			t.Cleanup(func() { _ = p.Close() })

			engine := gin.New()
			router.New(cfg, p).InstallToEngine(engine)
			if err := docsui.CompareRoutes(engine.Routes(), spec(cfg), cfg.API.Prefix); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func assertDemoPathPresence(t *testing.T, document docsui.Document, expected bool) {
	t.Helper()
	for _, path := range []string{"/ping", "/auth/access-token", "/profile", "/jobs/echo"} {
		_, exists := document.Paths[path]
		if exists != expected {
			t.Fatalf("path %s existence = %t, expected %t", path, exists, expected)
		}
	}
	if expected {
		operation := document.Paths["/auth/access-token"]["post"]
		if operation.ID != "issueAccessToken" || operation.RequestBody == nil {
			t.Fatalf("unexpected token operation: %#v", operation)
		}
		if _, ok := operation.Responses["default"]; !ok {
			t.Fatalf("token operation must document the error envelope")
		}
	}
}

func newContractTestConfig(t *testing.T, env string) *config.Config {
	t.Helper()
	return &config.Config{
		App:  config.AppConfig{Name: "grove", Env: env},
		Port: "8080",
		Log: config.LogConfig{
			Level:   "error",
			Path:    t.TempDir(),
			Console: false,
			Service: "api-contract-test",
		},
		JWT: config.JWTConfig{
			Secret:            "0123456789abcdef0123456789abcdef",
			Issuer:            "grove",
			AccessExpiryHours: 24,
		},
		Docs: config.DocsConfig{
			Title:       "Grove API",
			Description: "API contract",
			Version:     "1.0.0",
			BasePath:    "/api/v1",
		},
		API: config.APIConfig{Prefix: "/api/v1"},
	}
}
