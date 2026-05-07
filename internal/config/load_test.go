package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadWithOptionsExpandsEnv(t *testing.T) {
	t.Setenv("APP_PORT", "9090")
	t.Setenv("JWT_SECRET", "test-secret")

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(configPath, []byte(`
app:
  name: demo
  env: development
port: ${APP_PORT:8080}
jwt:
  secret: ${JWT_SECRET:change-me}
  issuer: demo
  access_expiry_hours: 24
api:
  prefix: /api/v1
`), 0o600)
	if err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadWithOptions(LoadOptions{ConfigFile: configPath, Service: "api"})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.Port != "9090" {
		t.Fatalf("expected port 9090, got %s", cfg.Port)
	}
	if cfg.JWT.Secret != "test-secret" {
		t.Fatalf("expected secret to be expanded")
	}
}
