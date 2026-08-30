package config

import (
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var envPattern = regexp.MustCompile(`\$\{([A-Z][A-Z0-9_]*)(?::([^}]*))?\}`)

func Load() (*Config, error) {
	return LoadWithOptions(LoadOptions{})
}

func LoadWithOptions(opts LoadOptions) (*Config, error) {
	cfg := defaultConfig()

	configFile := strings.TrimSpace(opts.ConfigFile)
	if configFile == "" {
		configFile = "config.yaml"
	}

	debugConfigured := false
	if raw, err := os.ReadFile(filepath.Clean(configFile)); err == nil {
		rawText := string(raw)
		debugConfigured = configHasAppDebug(rawText)
		expanded := expandEnv(rawText)
		decoder := yaml.NewDecoder(strings.NewReader(expanded))
		decoder.KnownFields(true)
		if err := decoder.Decode(&cfg); err != nil {
			return nil, fmt.Errorf("unmarshal config: %w", err)
		}
		var extra yaml.Node
		if err := decoder.Decode(&extra); err == nil {
			return nil, fmt.Errorf("unmarshal config: multiple YAML documents are not allowed")
		} else if err != io.EOF {
			return nil, fmt.Errorf("unmarshal config: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read config: %w", err)
	}

	applyEnvironmentOverrides(&cfg)
	cfg.normalize(opts.Service, debugConfigured || strings.TrimSpace(os.Getenv("APP_DEBUG")) != "")
	if err := cfg.Validate(opts.Service); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func defaultConfig() Config {
	return Config{
		App: AppConfig{
			Name:  "grove",
			Env:   "development",
			Debug: true,
		},
		Port:        "8080",
		ConsolePort: "8081",
		WorkerPort:  "8082",
		Server: ServerConfig{
			ShutdownTimeout: 30,
			ReadTimeout:     30,
			WriteTimeout:    30,
			IdleTimeout:     60,
			MaxHeaderBytes:  1 << 20,
			MaxBodyBytes:    32 * 1024 * 1024,
		},
		Log: LogConfig{
			Level:   "info",
			Path:    "./logs",
			Console: true,
			Service: "grove",
		},
		Databases: DatabasesConfig{
			Default: DatabaseConfig{
				Driver:          "postgres",
				SSLMode:         "disable",
				Charset:         "utf8mb4",
				ParseTime:       true,
				Loc:             "Local",
				MaxConnections:  20,
				MaxIdleConns:    10,
				ConnMaxLifetime: 3600,
				ConnectTimeout:  5,
			},
		},
		JWT: JWTConfig{
			Secret:            "change-me",
			Issuer:            "grove",
			AccessExpiryHours: 24,
		},
		Job: JobConfig{
			Concurrency: 10,
			Queues: map[string]int{
				"default":  5,
				"critical": 3,
				"low":      1,
			},
		},
		Scheduler: SchedulerConfig{
			Enabled:  false,
			Timezone: "Local",
		},
		Observability: ObservabilityConfig{
			Enabled:          true,
			MetricsEnabled:   true,
			MetricsPath:      "/metrics",
			ReadinessTimeout: 3,
			TraceSampleRatio: 0.1,
		},
		Casbin: CasbinConfig{
			Enforcers: map[string]CasbinEnforcerConfig{},
		},
		Storage: StorageConfig{
			Default:             "local",
			DefaultUploadPolicy: "document",
			UploadPolicies:      defaultUploadPolicies(),
			Disks: map[string]StorageDiskConfig{
				"local": {
					Driver:  "local",
					Root:    "./storage",
					BaseURL: "/storage",
					Secure:  false,
					Prefix:  "",
				},
			},
		},
		Docs: DocsConfig{
			Enabled:         true,
			Title:           "Grove API",
			Description:     "API framework scaffold for interface-driven services",
			Version:         "1.0.0",
			BasePath:        "/api/v1",
			Schemes:         []string{"http"},
			ScalarScriptURL: "https://cdn.jsdelivr.net/npm/@scalar/api-reference",
		},
		CORS: CORSConfig{
			Enabled: true,
			AllowedOrigins: []string{
				"*",
			},
			AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
			AllowedHeaders: []string{"Authorization", "Content-Type", "X-Request-Id"},
			MaxAge:         600,
		},
		API: APIConfig{
			Prefix:         "/api/v1",
			DefaultPerPage: 20,
			MaxPerPage:     100,
		},
		Security: SecurityConfig{
			TrustedProxies: []string{},
			Login: LoginProtectionConfig{
				Enabled:           true,
				AttemptsPerMinute: 10,
				Burst:             5,
				FailureLimit:      5,
				LockSeconds:       900,
			},
		},
	}
}

func configHasAppDebug(rawConfig string) bool {
	var root yaml.Node
	expanded := expandEnv(rawConfig)
	if err := yaml.Unmarshal([]byte(expanded), &root); err != nil || len(root.Content) == 0 {
		return false
	}
	doc := root.Content[0]
	if doc.Kind != yaml.MappingNode {
		return false
	}
	for i := 0; i+1 < len(doc.Content); i += 2 {
		if doc.Content[i].Value != "app" || doc.Content[i+1].Kind != yaml.MappingNode {
			continue
		}
		app := doc.Content[i+1]
		for j := 0; j+1 < len(app.Content); j += 2 {
			if app.Content[j].Value == "debug" {
				value := app.Content[j+1]
				return strings.TrimSpace(value.Value) != "" && value.Tag != "!!null"
			}
		}
	}
	return false
}

func expandEnv(input string) string {
	return envPattern.ReplaceAllStringFunc(input, func(match string) string {
		parts := envPattern.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		name := parts[1]
		def := ""
		if len(parts) > 2 {
			def = parts[2]
		}
		if value := os.Getenv(name); value != "" {
			return value
		}
		return def
	})
}

func applyEnvironmentOverrides(cfg *Config) {
	if value := os.Getenv("APP_ENV"); value != "" {
		cfg.App.Env = value
	}
	if value := os.Getenv("APP_DEBUG"); value != "" {
		cfg.App.Debug = parseBool(value)
	}
	if value := os.Getenv("APP_PORT"); value != "" {
		cfg.Port = value
	}
	if value := os.Getenv("CONSOLE_PORT"); value != "" {
		cfg.ConsolePort = value
	}
	if value := os.Getenv("SERVER_MAX_BODY_BYTES"); value != "" {
		cfg.Server.MaxBodyBytes = parseInt64(value, cfg.Server.MaxBodyBytes)
	}
	if value := os.Getenv("SERVER_IDLE_TIMEOUT"); value != "" {
		cfg.Server.IdleTimeout = parseInt(value, cfg.Server.IdleTimeout)
	}
	if value := os.Getenv("JWT_SECRET"); value != "" {
		cfg.JWT.Secret = value
	}
	if value := os.Getenv("DB_HOST"); value != "" {
		cfg.Databases.Default.Host = value
	}
	if value := os.Getenv("DB_PORT"); value != "" {
		cfg.Databases.Default.Port = value
	}
	if value := os.Getenv("DB_USER"); value != "" {
		cfg.Databases.Default.User = value
	}
	if value := os.Getenv("DB_PASSWORD"); value != "" {
		cfg.Databases.Default.Password = value
	}
	if value := os.Getenv("DB_NAME"); value != "" {
		cfg.Databases.Default.DBName = value
	}
	if value := os.Getenv("DB_SSLMODE"); value != "" {
		cfg.Databases.Default.SSLMode = value
	}
	if value := os.Getenv("DB_DRIVER"); value != "" {
		cfg.Databases.Default.Driver = value
	}
	if value := os.Getenv("DB_CHARSET"); value != "" {
		cfg.Databases.Default.Charset = value
	}
	if value := os.Getenv("DB_PARSE_TIME"); value != "" {
		cfg.Databases.Default.ParseTime = parseBool(value)
	}
	if value := os.Getenv("DB_LOC"); value != "" {
		cfg.Databases.Default.Loc = value
	}
	if value := os.Getenv("DB_TLS"); value != "" {
		cfg.Databases.Default.TLS = parseBool(value)
	}
	if value := os.Getenv("DB_CONNECT_TIMEOUT"); value != "" {
		cfg.Databases.Default.ConnectTimeout = parseInt(value, cfg.Databases.Default.ConnectTimeout)
	}
	if value := os.Getenv("DB_ENABLED"); value != "" {
		cfg.Databases.Default.Enabled = parseBool(value)
	}
	if value := os.Getenv("REDIS_ADDR"); value != "" {
		cfg.Redis.Addr = value
	}
	if value := os.Getenv("REDIS_PASSWORD"); value != "" {
		cfg.Redis.Password = value
	}
	if value := os.Getenv("REDIS_DB"); value != "" {
		cfg.Redis.DB = parseInt(value, cfg.Redis.DB)
	}
	if value := os.Getenv("REDIS_ENABLED"); value != "" {
		cfg.Redis.Enabled = parseBool(value)
	}
	if value := os.Getenv("WORKER_ENABLED"); value != "" {
		cfg.Job.Enabled = parseBool(value)
	}
	if value := os.Getenv("SCHEDULER_ENABLED"); value != "" {
		cfg.Scheduler.Enabled = parseBool(value)
	}
	if value := os.Getenv("SCHEDULER_TIMEZONE"); value != "" {
		cfg.Scheduler.Timezone = value
	}
	if value := os.Getenv("DEMO_ENABLED"); value != "" {
		cfg.Demo.Enabled = parseBool(value)
	}
	if value := os.Getenv("HSTS_ENABLED"); value != "" {
		cfg.Security.HSTSEnabled = parseBool(value)
	}
	if value := os.Getenv("CONFIG_ENCRYPTION_KEY"); value != "" {
		cfg.Security.ConfigEncryptionKey = value
	}
	if value := os.Getenv("LOGIN_PROTECTION_ENABLED"); value != "" {
		cfg.Security.Login.Enabled = parseBool(value)
	}
	if value := os.Getenv("LOGIN_ATTEMPTS_PER_MINUTE"); value != "" {
		cfg.Security.Login.AttemptsPerMinute = parseInt(value, cfg.Security.Login.AttemptsPerMinute)
	}
	if value := os.Getenv("LOGIN_BURST"); value != "" {
		cfg.Security.Login.Burst = parseInt(value, cfg.Security.Login.Burst)
	}
	if value := os.Getenv("LOGIN_FAILURE_LIMIT"); value != "" {
		cfg.Security.Login.FailureLimit = parseInt(value, cfg.Security.Login.FailureLimit)
	}
	if value := os.Getenv("LOGIN_LOCK_SECONDS"); value != "" {
		cfg.Security.Login.LockSeconds = parseInt(value, cfg.Security.Login.LockSeconds)
	}
}

func parseBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "t", "true", "y", "yes", "on":
		return true
	case "0", "f", "false", "n", "no", "off":
		return false
	default:
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return false
		}
		return parsed
	}
}

func parseInt(value string, fallback int) int {
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func parseInt64(value string, fallback int64) int64 {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func (c *Config) normalize(service string, debugConfigured bool) {
	if strings.TrimSpace(c.App.Name) == "" {
		c.App.Name = "grove"
	}
	if !debugConfigured {
		c.App.Debug = !strings.EqualFold(strings.TrimSpace(c.App.Env), "production")
	}
	if strings.TrimSpace(c.Port) == "" {
		c.Port = "8080"
	}
	if strings.TrimSpace(c.ConsolePort) == "" {
		c.ConsolePort = "8081"
	}
	if strings.TrimSpace(c.Log.Path) == "" {
		c.Log.Path = "./logs"
	}
	if strings.TrimSpace(c.Log.Service) == "" {
		if strings.TrimSpace(service) != "" {
			c.Log.Service = service
		} else {
			c.Log.Service = c.App.Name
		}
	}
	if strings.TrimSpace(c.JWT.Issuer) == "" {
		c.JWT.Issuer = c.App.Name
	}
	if strings.TrimSpace(c.Scheduler.Timezone) == "" {
		c.Scheduler.Timezone = "Local"
	}
	if strings.TrimSpace(c.API.Prefix) == "" {
		c.API.Prefix = "/api/v1"
	}
	if c.Databases.Resources == nil {
		c.Databases.Resources = map[string]DatabaseConfig{}
	}
	normalizeDatabaseConfig(&c.Databases.Default)
	for name, databaseCfg := range c.Databases.Resources {
		normalizeDatabaseConfig(&databaseCfg)
		c.Databases.Resources[name] = databaseCfg
	}
	if c.Casbin.Enforcers == nil {
		c.Casbin.Enforcers = map[string]CasbinEnforcerConfig{}
	}
	if strings.TrimSpace(c.Storage.Default) == "" {
		c.Storage.Default = "local"
	}
	if c.Storage.Disks == nil {
		c.Storage.Disks = map[string]StorageDiskConfig{}
	}
	if len(c.Storage.Disks) == 0 {
		c.Storage.Disks["local"] = StorageDiskConfig{
			Driver:  "local",
			Root:    "./storage",
			BaseURL: "/storage",
		}
	}
	for name, disk := range c.Storage.Disks {
		if strings.TrimSpace(disk.Driver) == "" {
			disk.Driver = name
		}
		if disk.Driver == "local" {
			if strings.TrimSpace(disk.Root) == "" {
				disk.Root = "./storage"
			}
			if strings.TrimSpace(disk.BaseURL) == "" {
				disk.BaseURL = "/storage"
			}
		}
		if strings.TrimSpace(disk.STS.Region) == "" {
			disk.STS.Region = disk.Region
		}
		if strings.TrimSpace(disk.STS.RoleSession) == "" {
			disk.STS.RoleSession = "grove-console"
		}
		if disk.STS.Duration <= 0 {
			disk.STS.Duration = 3600
		}
		if len(disk.STS.AllowPrefix) == 0 {
			disk.STS.AllowPrefix = []string{"console/${user_id}"}
		}
		if len(disk.STS.AllowActions) == 0 {
			disk.STS.AllowActions = []string{
				"s3:PutObject",
				"s3:GetObject",
				"s3:AbortMultipartUpload",
				"s3:ListBucketMultipartUploads",
				"s3:ListMultipartUploadParts",
			}
		}
		c.Storage.Disks[name] = disk
	}
	if _, ok := c.Storage.Disks[c.Storage.Default]; !ok {
		for name := range c.Storage.Disks {
			c.Storage.Default = name
			break
		}
	}
	if strings.TrimSpace(c.Storage.DefaultUploadPolicy) == "" {
		c.Storage.DefaultUploadPolicy = "document"
	}
	if len(c.Storage.UploadPolicies) == 0 {
		c.Storage.UploadPolicies = defaultUploadPolicies()
	}
	if len(c.Docs.Schemes) == 0 {
		c.Docs.Schemes = []string{"http"}
	}
	if strings.TrimSpace(c.Docs.ScalarScriptURL) == "" {
		c.Docs.ScalarScriptURL = "https://cdn.jsdelivr.net/npm/@scalar/api-reference"
	}
	if c.Security.TrustedProxies == nil {
		c.Security.TrustedProxies = []string{}
	}
	if c.Security.Login.AttemptsPerMinute <= 0 {
		c.Security.Login.AttemptsPerMinute = 10
	}
	if c.Security.Login.Burst <= 0 {
		c.Security.Login.Burst = 5
	}
	if c.Security.Login.FailureLimit <= 0 {
		c.Security.Login.FailureLimit = 5
	}
	if c.Security.Login.LockSeconds <= 0 {
		c.Security.Login.LockSeconds = 900
	}
	if strings.TrimSpace(c.Observability.MetricsPath) == "" {
		c.Observability.MetricsPath = "/metrics"
	}
	if c.Observability.ReadinessTimeout <= 0 {
		c.Observability.ReadinessTimeout = 3
	}
}

func normalizeDatabaseConfig(cfg *DatabaseConfig) {
	if cfg == nil {
		return
	}
	driver := strings.ToLower(strings.TrimSpace(cfg.Driver))
	if driver == "postgresql" {
		driver = "postgres"
	}
	if driver == "" {
		driver = "postgres"
	}
	cfg.Driver = driver
	if strings.TrimSpace(cfg.Port) == "" {
		if driver == "mysql" {
			cfg.Port = "3306"
		} else {
			cfg.Port = "5432"
		}
	}
	if strings.TrimSpace(cfg.Charset) == "" {
		cfg.Charset = "utf8mb4"
	}
	if strings.TrimSpace(cfg.Loc) == "" {
		cfg.Loc = "Local"
	}
}

func (c Config) Validate(service string) error {
	service = strings.ToLower(strings.TrimSpace(service))
	switch service {
	case "", "api", "console", "worker":
	default:
		return fmt.Errorf("unknown service %q", service)
	}

	if err := validatePort("port", c.Port); err != nil {
		return err
	}
	if err := validatePort("console_port", c.ConsolePort); err != nil {
		return err
	}
	if err := validatePort("worker_port", c.WorkerPort); err != nil {
		return err
	}
	if err := validateServerConfig(c.Server); err != nil {
		return err
	}
	if c.API.DefaultPerPage <= 0 || c.API.MaxPerPage <= 0 || c.API.DefaultPerPage > c.API.MaxPerPage {
		return fmt.Errorf("api pagination defaults must be positive and default_per_page cannot exceed max_per_page")
	}
	if strings.EqualFold(strings.TrimSpace(c.App.Env), "production") {
		if c.App.Debug {
			return fmt.Errorf("production app.debug must be false")
		}
		secret := strings.TrimSpace(c.JWT.Secret)
		if secret == "" || secret == "change-me" || len(secret) < 32 {
			return fmt.Errorf("jwt secret must be set to a strong value in production")
		}
		if password := strings.TrimSpace(c.Security.InitialRootPassword); password != "" {
			if err := ValidateInitialRootPassword(password); err != nil {
				return fmt.Errorf("production %w", err)
			}
		}
		if containsString(c.CORS.AllowedOrigins, "*") {
			return fmt.Errorf("production cors allowed_origins cannot contain wildcard")
		}
		if service == "console" {
			if !c.Databases.Default.Enabled {
				return fmt.Errorf("production console requires default database to be enabled")
			}
			consoleEnforcer, ok := c.casbinEnforcer("console")
			if !ok || !consoleEnforcer.Enabled {
				return fmt.Errorf("production console requires console casbin enforcer to be enabled")
			}
		}
	}
	if c.Job.Enabled && !c.Redis.Enabled {
		return fmt.Errorf("job requires redis to be enabled")
	}
	if service == "worker" && !c.Job.Enabled && !c.Scheduler.Enabled {
		return fmt.Errorf("worker requires job or scheduler to be enabled")
	}
	if err := validateDatabaseConfig("default", c.Databases.Default); err != nil {
		return err
	}
	for name, databaseCfg := range c.Databases.Resources {
		if err := validateDatabaseConfig(name, databaseCfg); err != nil {
			return err
		}
	}
	for name, enforcer := range c.Casbin.Enforcers {
		if !enforcer.Enabled {
			continue
		}
		databaseName := strings.ToLower(strings.TrimSpace(enforcer.Database))
		if databaseName == "" {
			databaseName = "default"
		}
		databaseCfg, ok := c.databaseConfig(databaseName)
		if !ok || !databaseCfg.Enabled {
			return fmt.Errorf("casbin enforcer %q requires enabled database %q", name, databaseName)
		}
	}
	timezone := strings.TrimSpace(c.Scheduler.Timezone)
	if timezone == "" {
		timezone = "Local"
	}
	if timezone != "Local" {
		if _, err := time.LoadLocation(timezone); err != nil {
			return fmt.Errorf("scheduler timezone %q is invalid: %w", timezone, err)
		}
	}
	if c.CORS.AllowCredentials && containsString(c.CORS.AllowedOrigins, "*") {
		return fmt.Errorf("cors allow_credentials cannot be used with wildcard origin")
	}
	if c.Observability.TraceSampleRatio < 0 || c.Observability.TraceSampleRatio > 1 {
		return fmt.Errorf("observability trace_sample_ratio must be between 0 and 1")
	}
	metricsPath := strings.TrimSpace(c.Observability.MetricsPath)
	if c.Observability.MetricsEnabled && (!strings.HasPrefix(metricsPath, "/") || metricsPath == "/") {
		return fmt.Errorf("observability metrics_path must be an absolute non-root path")
	}
	if metricsPath == "/health" || metricsPath == "/health/live" || metricsPath == "/health/ready" {
		return fmt.Errorf("observability metrics_path conflicts with health endpoints")
	}
	if endpoint := strings.TrimSpace(c.Observability.OTLPTraceEndpoint); endpoint != "" {
		parsed, err := url.ParseRequestURI(endpoint)
		if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return fmt.Errorf("observability otlp_trace_endpoint must be an absolute http or https URL")
		}
	}
	for _, proxy := range c.Security.TrustedProxies {
		proxy = strings.TrimSpace(proxy)
		if proxy == "" {
			return fmt.Errorf("trusted proxy cannot be empty")
		}
		if proxy == "*" || proxy == "0.0.0.0/0" || proxy == "::/0" {
			return fmt.Errorf("trusted proxy %q is too broad", proxy)
		}
		if net.ParseIP(proxy) == nil {
			if _, _, err := net.ParseCIDR(proxy); err != nil {
				return fmt.Errorf("trusted proxy %q is invalid", proxy)
			}
		}
	}
	if strings.TrimSpace(c.Storage.Default) == "" {
		return fmt.Errorf("storage default disk is required")
	}
	if _, ok := c.Storage.Disks[c.Storage.Default]; !ok {
		return fmt.Errorf("storage default disk %q is not configured", c.Storage.Default)
	}
	for name, disk := range c.Storage.Disks {
		switch strings.TrimSpace(strings.ToLower(disk.Driver)) {
		case "local", "s3", "aws":
		default:
			return fmt.Errorf("storage disk %q uses unsupported driver %q", name, disk.Driver)
		}
	}
	defaultPolicy := strings.ToLower(strings.TrimSpace(c.Storage.DefaultUploadPolicy))
	if _, ok := c.Storage.UploadPolicies[defaultPolicy]; !ok {
		return fmt.Errorf("storage default upload policy %q is not configured", defaultPolicy)
	}
	for name, policy := range c.Storage.UploadPolicies {
		policyName := strings.ToLower(strings.TrimSpace(name))
		if policyName == "" {
			return fmt.Errorf("storage upload policy name cannot be empty")
		}
		if policy.MaxBytes <= 0 {
			return fmt.Errorf("storage upload policy %q max_bytes must be positive", policyName)
		}
		if len(policy.Extensions) == 0 || len(policy.MIMETypes) == 0 {
			return fmt.Errorf("storage upload policy %q requires extensions and mime_types", policyName)
		}
		cleanDir := strings.TrimPrefix(path.Clean("/"+strings.TrimSpace(policy.Directory)), "/")
		if cleanDir == "." || cleanDir == "" || cleanDir != strings.Trim(strings.TrimSpace(policy.Directory), "/") {
			return fmt.Errorf("storage upload policy %q directory is invalid", policyName)
		}
	}
	return nil
}

func (c Config) databaseConfig(name string) (DatabaseConfig, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" || name == "default" {
		return c.Databases.Default, true
	}
	for resourceName, cfg := range c.Databases.Resources {
		if strings.EqualFold(strings.TrimSpace(resourceName), name) {
			return cfg, true
		}
	}
	return DatabaseConfig{}, false
}

func (c Config) casbinEnforcer(name string) (CasbinEnforcerConfig, bool) {
	for enforcerName, cfg := range c.Casbin.Enforcers {
		if strings.EqualFold(strings.TrimSpace(enforcerName), name) {
			return cfg, true
		}
	}
	return CasbinEnforcerConfig{}, false
}

func validateDatabaseConfig(name string, cfg DatabaseConfig) error {
	if !cfg.Enabled {
		return nil
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("database resource name cannot be empty")
	}
	driver := strings.ToLower(strings.TrimSpace(cfg.Driver))
	switch driver {
	case "postgres", "postgresql":
	case "mysql":
	default:
		return fmt.Errorf("database %q uses unsupported driver %q", name, cfg.Driver)
	}
	if strings.TrimSpace(cfg.Host) == "" {
		return fmt.Errorf("database %q host is required", name)
	}
	if err := validatePort(fmt.Sprintf("database %q port", name), cfg.Port); err != nil {
		return err
	}
	if strings.TrimSpace(cfg.User) == "" {
		return fmt.Errorf("database %q user is required", name)
	}
	if strings.TrimSpace(cfg.DBName) == "" {
		return fmt.Errorf("database %q dbname is required", name)
	}
	if driver == "mysql" {
		if strings.TrimSpace(cfg.Charset) == "" {
			return fmt.Errorf("database %q charset is required for mysql", name)
		}
		if strings.TrimSpace(cfg.Loc) == "" {
			return fmt.Errorf("database %q loc is required for mysql", name)
		}
	}
	if cfg.ConnectTimeout <= 0 {
		return fmt.Errorf("database %q connect_timeout must be positive", name)
	}
	return nil
}

func defaultUploadPolicies() map[string]UploadPolicyConfig {
	return map[string]UploadPolicyConfig{
		"avatar": {
			Directory:  "avatars",
			MaxBytes:   5 * 1024 * 1024,
			Extensions: []string{".jpg", ".jpeg", ".png", ".gif", ".webp"},
			MIMETypes:  []string{"image/jpeg", "image/png", "image/gif", "image/webp"},
		},
		"document": {
			Directory:  "documents",
			MaxBytes:   20 * 1024 * 1024,
			Extensions: []string{".pdf", ".txt", ".csv", ".doc", ".docx", ".xls", ".xlsx"},
			MIMETypes: []string{
				"application/pdf",
				"text/plain",
				"text/csv",
				"application/x-ole-storage",
				"application/zip",
				"application/octet-stream",
			},
		},
	}
}

func validatePort(name, value string) error {
	port, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("%s must be a valid port", name)
	}
	return nil
}

func validateServerConfig(cfg ServerConfig) error {
	if cfg.ShutdownTimeout <= 0 {
		return fmt.Errorf("server.shutdown_timeout must be positive")
	}
	if cfg.ReadTimeout <= 0 {
		return fmt.Errorf("server.read_timeout must be positive")
	}
	if cfg.WriteTimeout <= 0 {
		return fmt.Errorf("server.write_timeout must be positive")
	}
	if cfg.IdleTimeout <= 0 {
		return fmt.Errorf("server.idle_timeout must be positive")
	}
	if cfg.MaxHeaderBytes <= 0 {
		return fmt.Errorf("server.max_header_bytes must be positive")
	}
	if cfg.MaxBodyBytes <= 0 {
		return fmt.Errorf("server.max_body_bytes must be positive")
	}
	return nil
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == target {
			return true
		}
	}
	return false
}
