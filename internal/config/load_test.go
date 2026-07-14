package config

import (
	"os"
	"path/filepath"
	"strings"
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

func TestLoadWithOptionsValidatesProductionSecret(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(configPath, []byte(`
app:
  env: production
jwt:
  secret: change-me
storage:
  default: local
  disks:
    local:
      driver: local
      root: ./storage
`), 0o600)
	if err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err = LoadWithOptions(LoadOptions{ConfigFile: configPath, Service: "api"})
	if err == nil {
		t.Fatal("expected production weak jwt secret error")
	}
	if !strings.Contains(err.Error(), "jwt secret") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadWithOptionsDefaultsDebugByEnvironment(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(configPath, []byte(`
app:
  env: test
`), 0o600)
	if err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadWithOptions(LoadOptions{ConfigFile: configPath, Service: "api"})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if !cfg.App.Debug {
		t.Fatal("expected debug to default true outside production")
	}

	configPath = filepath.Join(dir, "production.yaml")
	err = os.WriteFile(configPath, []byte(`
app:
  env: production
jwt:
  secret: 12345678901234567890123456789012
cors:
  allowed_origins: [https://console.example.com]
`), 0o600)
	if err != nil {
		t.Fatalf("write production config: %v", err)
	}
	cfg, err = LoadWithOptions(LoadOptions{ConfigFile: configPath, Service: "api"})
	if err != nil {
		t.Fatalf("load production config: %v", err)
	}
	if cfg.App.Debug {
		t.Fatal("expected debug to default false in production")
	}
}

func TestLoadWithOptionsOverridesDebugFromEnv(t *testing.T) {
	t.Setenv("APP_DEBUG", "false")

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(configPath, []byte(`
app:
  env: development
`), 0o600)
	if err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadWithOptions(LoadOptions{ConfigFile: configPath, Service: "api"})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.App.Debug {
		t.Fatal("expected APP_DEBUG=false to disable debug")
	}

	t.Setenv("APP_DEBUG", "yes")
	cfg, err = LoadWithOptions(LoadOptions{ConfigFile: configPath, Service: "api"})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if !cfg.App.Debug {
		t.Fatal("expected APP_DEBUG=yes to enable debug")
	}
}

func TestLoadWithOptionsAllowsDebugOverrideInProduction(t *testing.T) {
	t.Setenv("APP_DEBUG", "true")

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(configPath, []byte(`
app:
  env: production
jwt:
  secret: 12345678901234567890123456789012
cors:
  allowed_origins: [https://console.example.com]
`), 0o600)
	if err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadWithOptions(LoadOptions{ConfigFile: configPath, Service: "api"})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if !cfg.App.Debug {
		t.Fatal("expected APP_DEBUG=true to enable debug in production")
	}
}

func TestLoadWithOptionsAllowsExplicitConfigDebugInProduction(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(configPath, []byte(`
app:
  env: production
  debug: true
jwt:
  secret: 12345678901234567890123456789012
cors:
  allowed_origins: [https://console.example.com]
`), 0o600)
	if err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadWithOptions(LoadOptions{ConfigFile: configPath, Service: "api"})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if !cfg.App.Debug {
		t.Fatal("expected explicit config debug to enable debug in production")
	}
}

func TestLoadWithOptionsTreatsEmptyDebugPlaceholderAsDefault(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(configPath, []byte(`
app:
  env: development
  debug:
`), 0o600)
	if err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadWithOptions(LoadOptions{ConfigFile: configPath, Service: "api"})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if !cfg.App.Debug {
		t.Fatal("expected empty debug value to use development default")
	}
}

func TestValidateRejectsJobWithoutRedis(t *testing.T) {
	cfg := defaultConfig()
	cfg.Job.Enabled = true
	cfg.Redis.Enabled = false

	if err := cfg.Validate("worker"); err == nil {
		t.Fatal("expected job without redis validation error")
	}
}

func TestValidateRejectsCredentialsWithWildcardCORS(t *testing.T) {
	cfg := defaultConfig()
	cfg.CORS.AllowCredentials = true
	cfg.CORS.AllowedOrigins = []string{"*"}

	if err := cfg.Validate("api"); err == nil {
		t.Fatal("expected wildcard credentials validation error")
	}
}

func TestValidateRejectsInvalidPort(t *testing.T) {
	cfg := defaultConfig()
	cfg.Port = "99999"

	if err := cfg.Validate("api"); err == nil {
		t.Fatal("expected invalid port validation error")
	}
}

func TestLoadConfigExampleDefaultsDemoOff(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "config.example.yaml"))
	if err != nil {
		t.Fatalf("read config example: %v", err)
	}
	configPath := filepath.Join(t.TempDir(), "config.example.yaml")
	if err := os.WriteFile(configPath, raw, 0o600); err != nil {
		t.Fatalf("copy config example: %v", err)
	}
	t.Setenv("APP_ENV", "development")
	t.Setenv("DEMO_ENABLED", "")

	cfg, err := LoadWithOptions(LoadOptions{ConfigFile: configPath, Service: "api"})
	if err != nil {
		t.Fatalf("load config example: %v", err)
	}
	if cfg.Demo.Enabled {
		t.Fatal("demo must be disabled by default")
	}
}

func TestLoadWithOptionsReadsDemoEnabledOverride(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("demo:\n  enabled: false\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("APP_ENV", "development")
	t.Setenv("DEMO_ENABLED", "true")

	cfg, err := LoadWithOptions(LoadOptions{ConfigFile: configPath, Service: "api"})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if !cfg.Demo.Enabled {
		t.Fatal("expected DEMO_ENABLED to enable demo routes")
	}
}

func TestValidateProductionRejectsWildcardCORSAndBroadTrustedProxies(t *testing.T) {
	cfg := defaultConfig()
	cfg.App.Env = "production"
	cfg.JWT.Secret = "0123456789abcdef0123456789abcdef"
	cfg.CORS.AllowedOrigins = []string{"*"}
	if err := cfg.Validate("api"); err == nil {
		t.Fatal("production wildcard CORS must be rejected")
	}

	cfg.CORS.AllowedOrigins = []string{"https://console.example.com"}
	cfg.Security.TrustedProxies = []string{"0.0.0.0/0"}
	if err := cfg.Validate("api"); err == nil {
		t.Fatal("broad trusted proxy range must be rejected")
	}

	cfg.Security.TrustedProxies = []string{"10.0.0.0/8", "127.0.0.1"}
	if err := cfg.Validate("api"); err != nil {
		t.Fatalf("explicit trusted proxies should pass: %v", err)
	}
}

func TestLoadWithOptionsReadsLoginProtectionOverrides(t *testing.T) {
	t.Setenv("LOGIN_PROTECTION_ENABLED", "false")
	t.Setenv("LOGIN_ATTEMPTS_PER_MINUTE", "24")
	t.Setenv("LOGIN_BURST", "7")
	t.Setenv("LOGIN_FAILURE_LIMIT", "6")
	t.Setenv("LOGIN_LOCK_SECONDS", "120")

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("app:\n  env: test\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := LoadWithOptions(LoadOptions{ConfigFile: configPath, Service: "console"})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	login := cfg.Security.Login
	if login.Enabled {
		t.Fatal("expected LOGIN_PROTECTION_ENABLED=false")
	}
	if login.AttemptsPerMinute != 24 || login.Burst != 7 || login.FailureLimit != 6 || login.LockSeconds != 120 {
		t.Fatalf("unexpected login protection overrides: %#v", login)
	}
}
