package docs

import (
	"testing"

	"github.com/zhimma/grove/internal/config"
)

func TestSpecOnlyIncludesDemoPathsWhenExplicitlyEnabledOutsideProduction(t *testing.T) {
	cfg := &config.Config{
		App:  config.AppConfig{Env: "development"},
		Docs: config.DocsConfig{BasePath: "/api/v1"},
	}
	assertDemoPathPresence(t, spec(cfg), false)

	cfg.Demo.Enabled = true
	assertDemoPathPresence(t, spec(cfg), true)

	cfg.App.Env = "production"
	assertDemoPathPresence(t, spec(cfg), false)
}

func assertDemoPathPresence(t *testing.T, document map[string]any, expected bool) {
	t.Helper()
	paths, ok := document["paths"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected paths payload: %#v", document["paths"])
	}
	for _, path := range []string{"/ping", "/auth/access-token", "/profile", "/jobs/echo"} {
		_, exists := paths[path]
		if exists != expected {
			t.Fatalf("path %s existence = %t, expected %t", path, exists, expected)
		}
	}
}
