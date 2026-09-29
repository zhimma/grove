package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
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

func TestConfigExampleUsesLiteralDefaults(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "config.example.yaml"))
	if err != nil {
		t.Fatalf("read config example: %v", err)
	}
	if envPattern.Match(content) {
		t.Fatal("config.example.yaml must not use uppercase environment placeholders for defaults")
	}
}

func TestExpandEnvPreservesLowercaseTemplates(t *testing.T) {
	got := expandEnv(`console/${user_id}/${APP_NAME:grove}`)
	if got != "console/${user_id}/grove" {
		t.Fatalf("lowercase template was changed unexpectedly: %q", got)
	}
}

func TestLoadWithOptionsSupportsMySQLDatabaseConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte(`databases:
  default:
    enabled: true
    driver: mysql
    host: 127.0.0.1
    user: root
    password: secret
    dbname: grove
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadWithOptions(LoadOptions{ConfigFile: configPath, Service: "api"})
	if err != nil {
		t.Fatalf("load mysql config: %v", err)
	}
	if cfg.Databases.Default.Driver != "mysql" {
		t.Fatalf("expected mysql driver, got %q", cfg.Databases.Default.Driver)
	}
	if cfg.Databases.Default.Port != "3306" {
		t.Fatalf("expected mysql default port, got %q", cfg.Databases.Default.Port)
	}
	if cfg.Databases.Default.Charset != "utf8mb4" || cfg.Databases.Default.Loc != "Local" {
		t.Fatalf("unexpected mysql defaults: %#v", cfg.Databases.Default)
	}
}

func TestLoadWithOptionsReadsInitialRootPassword(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte(`security:
  initial_root_password: root123456
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadWithOptions(LoadOptions{ConfigFile: configPath, Service: "api"})
	if err != nil {
		t.Fatalf("load root password config: %v", err)
	}
	if cfg.Security.InitialRootPassword != "root123456" {
		t.Fatalf("unexpected initial root password: %q", cfg.Security.InitialRootPassword)
	}
}

func TestLoadWithOptionsReadsDatabaseDriverOverrides(t *testing.T) {
	t.Setenv("DB_DRIVER", "mysql")
	t.Setenv("DB_PORT", "3307")
	t.Setenv("DB_CHARSET", "utf8mb4")
	t.Setenv("DB_PARSE_TIME", "true")
	t.Setenv("DB_LOC", "UTC")
	t.Setenv("DB_TLS", "true")

	cfg, err := LoadWithOptions(LoadOptions{ConfigFile: filepath.Join(t.TempDir(), "config.yaml"), Service: "api"})
	if err != nil {
		t.Fatalf("load database overrides: %v", err)
	}
	db := cfg.Databases.Default
	if db.Driver != "mysql" || db.Port != "3307" || db.Charset != "utf8mb4" || !db.ParseTime || db.Loc != "UTC" || !db.TLS {
		t.Fatalf("unexpected database overrides: %#v", db)
	}
}

func TestLoadWithOptionsReadsDatabaseConnectTimeoutOverride(t *testing.T) {
	t.Setenv("DB_CONNECT_TIMEOUT", "9")
	cfg, err := LoadWithOptions(LoadOptions{ConfigFile: filepath.Join(t.TempDir(), "config.yaml"), Service: "api"})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Databases.Default.ConnectTimeout != 9 {
		t.Fatalf("expected connect timeout override, got %d", cfg.Databases.Default.ConnectTimeout)
	}
}

func TestValidateRejectsNegativeDatabaseConnectTimeout(t *testing.T) {
	cfg := defaultConfig()
	cfg.Databases.Default.Enabled = true
	cfg.Databases.Default.Host = "127.0.0.1"
	cfg.Databases.Default.Port = "5432"
	cfg.Databases.Default.User = "grove"
	cfg.Databases.Default.DBName = "grove"
	cfg.Databases.Default.ConnectTimeout = -1
	if err := cfg.Validate("api"); err == nil || !strings.Contains(err.Error(), "connect_timeout") {
		t.Fatalf("expected negative connect timeout error, got %v", err)
	}
}

func TestLoadWithOptionsRejectsExplicitInvalidLimits(t *testing.T) {
	tests := map[string]string{
		"server zero timeout": "server:\n  idle_timeout: 0\n",
		"body limit":          "server:\n  max_body_bytes: 0\n",
		"pagination":          "api:\n  default_per_page: 101\n  max_per_page: 100\n",
		"log file size":       "log:\n  max_size_mb: 0\n",
		"log retention":       "log:\n  max_age_days: -1\n",
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
				t.Fatalf("write config: %v", err)
			}
			if _, err := LoadWithOptions(LoadOptions{ConfigFile: path, Service: "api"}); err == nil {
				t.Fatal("expected invalid explicit configuration to be rejected")
			}
		})
	}
}

// A config written before rotation existed must still load, rotate at a
// bounded size and expire old files, and the ignored log.service key must not
// trip strict decoding.
func TestLoadWithOptionsDefaultsLogRotation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("log:\n  level: info\n  service: legacy\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := LoadWithOptions(LoadOptions{ConfigFile: path, Service: "api"})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Log.MaxSizeMB != 100 || cfg.Log.MaxAgeDays != 14 {
		t.Fatalf("log rotation defaults = %d MB / %d days, want 100 / 14", cfg.Log.MaxSizeMB, cfg.Log.MaxAgeDays)
	}
}

func TestLoadWithOptionsHonorsExplicitMySQLParseTimeFalse(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte(`databases:
  default:
    enabled: true
    driver: mysql
    host: 127.0.0.1
    user: root
    dbname: grove
    parse_time: false
`), 0o600); err != nil {
		t.Fatalf("write mysql config: %v", err)
	}

	cfg, err := LoadWithOptions(LoadOptions{ConfigFile: configPath, Service: "api"})
	if err != nil {
		t.Fatalf("load mysql config: %v", err)
	}
	if cfg.Databases.Default.ParseTime {
		t.Fatal("explicit mysql parse_time=false must be preserved")
	}
}

func TestLoadWithOptionsPreservesExplicitMySQLPort(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte(`databases:
  default:
    enabled: true
    driver: mysql
    host: 127.0.0.1
    port: "5432"
    user: root
    dbname: grove
`), 0o600); err != nil {
		t.Fatalf("write mysql config: %v", err)
	}

	cfg, err := LoadWithOptions(LoadOptions{ConfigFile: configPath, Service: "api"})
	if err != nil {
		t.Fatalf("load mysql config: %v", err)
	}
	if cfg.Databases.Default.Port != "5432" {
		t.Fatalf("explicit mysql port must be preserved, got %q", cfg.Databases.Default.Port)
	}
}

func TestLoadWithOptionsRejectsUnknownFields(t *testing.T) {
	tests := map[string]string{
		"top level": "unknown_field: true\n",
		"nested":    "server:\n  read_timout: 30\n",
	}

	for name, raw := range tests {
		t.Run(name, func(t *testing.T) {
			configPath := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(configPath, []byte(raw), 0o600); err != nil {
				t.Fatalf("write config: %v", err)
			}

			_, err := LoadWithOptions(LoadOptions{ConfigFile: configPath, Service: "api"})
			if err == nil {
				t.Fatal("expected unknown field error")
			}
			if !strings.Contains(err.Error(), "field") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestLoadWithOptionsRejectsMultipleDocuments(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("app:\n  env: test\n---\napp:\n  name: second\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := LoadWithOptions(LoadOptions{ConfigFile: configPath, Service: "api"})
	if err == nil {
		t.Fatal("expected multiple document error")
	}
	if !strings.Contains(err.Error(), "multiple YAML documents") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadConfigExampleWithCleanEnvironment(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "config.example.yaml"))
	if err != nil {
		t.Fatalf("read config example: %v", err)
	}
	for _, match := range envPattern.FindAllStringSubmatch(string(raw), -1) {
		if len(match) > 1 {
			t.Setenv(match[1], "")
		}
	}

	configPath := filepath.Join(t.TempDir(), "config.example.yaml")
	if err := os.WriteFile(configPath, raw, 0o600); err != nil {
		t.Fatalf("copy config example: %v", err)
	}
	if _, err := LoadWithOptions(LoadOptions{ConfigFile: configPath, Service: "api"}); err != nil {
		t.Fatalf("load config example with clean environment: %v", err)
	}
}

func TestConfigExampleDoesNotContainStaticCredentials(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "config.example.yaml"))
	if err != nil {
		t.Fatalf("read config example: %v", err)
	}
	content := string(raw)
	var parsed Config
	if err := yaml.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("parse config example: %v", err)
	}
	for name, value := range map[string]string{
		"database password": parsed.Databases.Default.Password,
		"redis password":    parsed.Redis.Password,
		"jwt secret":        parsed.JWT.Secret,
		"root password":     parsed.Security.InitialRootPassword,
		"s3 access key":     parsed.Storage.Disks["s3"].AccessKey,
		"s3 secret key":     parsed.Storage.Disks["s3"].SecretKey,
	} {
		if strings.TrimSpace(value) != "" {
			t.Fatalf("config example contains a non-empty %s", name)
		}
	}
	for _, forbidden := range []string{
		"${DB_PASSWORD:postgres}",
		"password: postgres",
		"${JWT_SECRET:change-me}",
		"secret: change-me",
	} {
		if strings.Contains(content, forbidden) {
			t.Fatalf("config example contains static credential pattern %q", forbidden)
		}
	}
}

func TestValidateInitialRootPassword(t *testing.T) {
	for _, password := range []string{"", "short", "123456789012345", "replace-with-your-password"} {
		if err := ValidateInitialRootPassword(password); err == nil {
			t.Errorf("expected weak initial root password %q to be rejected", password)
		}
	}

	for _, password := range []string{
		"correct horse battery staple",
		"一段足够长的本地环境初始管理员密码",
		"N7!secure-root-password-2026",
	} {
		if err := ValidateInitialRootPassword(password); err != nil {
			t.Errorf("expected strong initial root password to pass: %v", err)
		}
	}
}

func TestValidateProductionRejectsWeakConfiguredInitialRootPassword(t *testing.T) {
	cfg := validProductionConfig()
	cfg.Security.InitialRootPassword = "short"
	if err := cfg.Validate("api"); err == nil || !strings.Contains(err.Error(), "initial root password") {
		t.Fatalf("expected weak production root password error, got %v", err)
	}

	cfg.Security.InitialRootPassword = "correct horse battery staple"
	if err := cfg.Validate("api"); err != nil {
		t.Fatalf("expected strong production root password to pass: %v", err)
	}
}

func TestValidateRejectsInvalidServerLimits(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*ServerConfig)
		field  string
	}{
		{name: "shutdown timeout", mutate: func(server *ServerConfig) { server.ShutdownTimeout = 0 }, field: "shutdown_timeout"},
		{name: "read timeout", mutate: func(server *ServerConfig) { server.ReadTimeout = -1 }, field: "read_timeout"},
		{name: "write timeout", mutate: func(server *ServerConfig) { server.WriteTimeout = 0 }, field: "write_timeout"},
		{name: "idle timeout", mutate: func(server *ServerConfig) { server.IdleTimeout = 0 }, field: "idle_timeout"},
		{name: "max header bytes", mutate: func(server *ServerConfig) { server.MaxHeaderBytes = 0 }, field: "max_header_bytes"},
		{name: "max body bytes", mutate: func(server *ServerConfig) { server.MaxBodyBytes = 0 }, field: "max_body_bytes"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := defaultConfig()
			tt.mutate(&cfg.Server)
			err := cfg.Validate("api")
			if err == nil || !strings.Contains(err.Error(), tt.field) {
				t.Fatalf("expected %s validation error, got %v", tt.field, err)
			}
		})
	}
}

func TestLoadDoesNotReadDotEnv(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("app:\n  env: development\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("DB_ENABLED=true\n"), 0o600); err != nil {
		t.Fatalf("write dot env: %v", err)
	}
	t.Setenv("DB_ENABLED", "")

	cfg, err := LoadWithOptions(LoadOptions{ConfigFile: configPath, Service: "api"})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Databases.Default.Enabled {
		t.Fatal(".env must not be loaded as a second configuration source")
	}
}

func TestObservabilityConfigValidation(t *testing.T) {
	cfg := defaultConfig()
	cfg.Observability.TraceSampleRatio = 1.1
	if err := cfg.Validate("api"); err == nil || !strings.Contains(err.Error(), "trace_sample_ratio") {
		t.Fatalf("expected trace sample validation error, got %v", err)
	}

	cfg = defaultConfig()
	cfg.Observability.MetricsPath = "/health/ready"
	if err := cfg.Validate("api"); err == nil || !strings.Contains(err.Error(), "conflicts") {
		t.Fatalf("expected metrics path conflict, got %v", err)
	}

	cfg = defaultConfig()
	cfg.Observability.OTLPTraceEndpoint = "javascript:alert(1)"
	if err := cfg.Validate("api"); err == nil || !strings.Contains(err.Error(), "otlp_trace_endpoint") {
		t.Fatalf("expected OTLP endpoint validation error, got %v", err)
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

func TestLoadWithOptionsRejectsDebugOverrideInProduction(t *testing.T) {
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

	if _, err := LoadWithOptions(LoadOptions{ConfigFile: configPath, Service: "api"}); err == nil || !strings.Contains(err.Error(), "app.debug") {
		t.Fatalf("expected production debug override to be rejected, got %v", err)
	}
}

func TestLoadWithOptionsRejectsExplicitConfigDebugInProduction(t *testing.T) {
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

	if _, err := LoadWithOptions(LoadOptions{ConfigFile: configPath, Service: "api"}); err == nil || !strings.Contains(err.Error(), "app.debug") {
		t.Fatalf("expected production debug configuration to be rejected, got %v", err)
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

func TestValidateRejectsUnknownService(t *testing.T) {
	cfg := defaultConfig()
	if err := cfg.Validate("ap1"); err == nil || !strings.Contains(err.Error(), "unknown service") {
		t.Fatalf("expected unknown service error, got %v", err)
	}
}

func TestValidateProductionConsoleRequiresDatabaseAndEnforcer(t *testing.T) {
	cfg := validProductionConfig()

	if err := cfg.Validate("console"); err == nil || !strings.Contains(err.Error(), "default database") {
		t.Fatalf("expected default database error, got %v", err)
	}

	cfg.Databases.Default = validDatabaseConfig()
	if err := cfg.Validate("console"); err == nil || !strings.Contains(err.Error(), "console casbin enforcer") {
		t.Fatalf("expected console enforcer error, got %v", err)
	}

	cfg.Casbin.Enforcers["console"] = CasbinEnforcerConfig{
		Enabled:  true,
		Database: "default",
		Mode:     "rbac",
	}
	if err := cfg.Validate("console"); err != nil {
		t.Fatalf("expected valid production console config, got %v", err)
	}
}

func TestValidateWorkerRequiresRunnableComponent(t *testing.T) {
	cfg := defaultConfig()
	if err := cfg.Validate("worker"); err == nil || !strings.Contains(err.Error(), "job or scheduler") {
		t.Fatalf("expected disabled worker error, got %v", err)
	}
}

func TestValidateCasbinRequiresEnabledDatabase(t *testing.T) {
	cfg := defaultConfig()
	cfg.Databases.Resources = map[string]DatabaseConfig{}
	cfg.Databases.Resources["reporting"] = DatabaseConfig{Enabled: false}
	cfg.Casbin.Enforcers["api"] = CasbinEnforcerConfig{
		Enabled:  true,
		Database: "reporting",
	}

	if err := cfg.Validate("api"); err == nil || !strings.Contains(err.Error(), "reporting") {
		t.Fatalf("expected disabled database reference error, got %v", err)
	}
}

func TestValidateEnabledDatabaseRequiresConnectionFields(t *testing.T) {
	cfg := defaultConfig()
	cfg.Databases.Default.Enabled = true
	cfg.Databases.Default.Host = ""

	if err := cfg.Validate("api"); err == nil || !strings.Contains(err.Error(), "host") {
		t.Fatalf("expected database host error, got %v", err)
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

func TestValidateRejectsInvalidPaginationPolicy(t *testing.T) {
	cfg := defaultConfig()
	cfg.API.DefaultPerPage = 101
	cfg.API.MaxPerPage = 100
	if err := cfg.Validate("api"); err == nil || !strings.Contains(err.Error(), "pagination") {
		t.Fatalf("expected pagination policy error, got %v", err)
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
	if cfg.Scheduler.Enabled || cfg.Scheduler.Timezone != "Local" {
		t.Fatalf("unexpected scheduler defaults: %#v", cfg.Scheduler)
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
	cfg.App.Debug = false
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

func TestLoadWithOptionsDefaultsSecureUploadPolicies(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("app:\n  env: test\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := LoadWithOptions(LoadOptions{ConfigFile: configPath, Service: "console"})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Server.MaxBodyBytes != 32*1024*1024 {
		t.Fatalf("unexpected max body bytes: %d", cfg.Server.MaxBodyBytes)
	}
	if cfg.Storage.DefaultUploadPolicy != "document" {
		t.Fatalf("unexpected default upload policy: %q", cfg.Storage.DefaultUploadPolicy)
	}
	for _, name := range []string{"avatar", "document"} {
		policy, ok := cfg.Storage.UploadPolicies[name]
		if !ok || policy.MaxBytes <= 0 || len(policy.Extensions) == 0 || len(policy.MIMETypes) == 0 {
			t.Fatalf("missing secure %s policy: %#v", name, policy)
		}
	}
}

func TestLoadWithOptionsReadsMaxBodyBytesOverride(t *testing.T) {
	t.Setenv("SERVER_MAX_BODY_BYTES", "4096")
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("app:\n  env: test\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := LoadWithOptions(LoadOptions{ConfigFile: configPath, Service: "api"})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Server.MaxBodyBytes != 4096 {
		t.Fatalf("expected max body override, got %d", cfg.Server.MaxBodyBytes)
	}
}

func TestLoadWithOptionsReadsConfigEncryptionKeyOverride(t *testing.T) {
	t.Setenv("CONFIG_ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef")
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("app:\n  env: test\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := LoadWithOptions(LoadOptions{ConfigFile: configPath, Service: "console"})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Security.ConfigEncryptionKey != "0123456789abcdef0123456789abcdef" {
		t.Fatal("expected config encryption key override")
	}
}

func TestLoadWithOptionsReadsSchedulerOverrides(t *testing.T) {
	t.Setenv("SCHEDULER_ENABLED", "true")
	t.Setenv("SCHEDULER_TIMEZONE", "Asia/Shanghai")
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("app:\n  env: test\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := LoadWithOptions(LoadOptions{ConfigFile: configPath, Service: "worker"})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if !cfg.Scheduler.Enabled || cfg.Scheduler.Timezone != "Asia/Shanghai" {
		t.Fatalf("unexpected scheduler config: %#v", cfg.Scheduler)
	}
}

func TestValidateRejectsInvalidSchedulerTimezone(t *testing.T) {
	cfg := defaultConfig()
	cfg.Scheduler.Enabled = true
	cfg.Scheduler.Timezone = "not/a-timezone"
	if err := cfg.Validate("worker"); err == nil {
		t.Fatal("expected invalid scheduler timezone error")
	}
}

func validProductionConfig() Config {
	cfg := defaultConfig()
	cfg.App.Env = "production"
	cfg.App.Debug = false
	cfg.JWT.Secret = "0123456789abcdef0123456789abcdef"
	cfg.CORS.AllowedOrigins = []string{"https://console.example.com"}
	return cfg
}

func validDatabaseConfig() DatabaseConfig {
	return DatabaseConfig{
		Enabled:        true,
		Driver:         "postgres",
		Host:           "127.0.0.1",
		Port:           "5432",
		User:           "grove",
		Password:       "secret",
		DBName:         "grove",
		SSLMode:        "disable",
		ConnectTimeout: 5,
	}
}
