package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

var envPattern = regexp.MustCompile(`\$\{([^:}]+)(?::([^}]*))?\}`)

func Load() (*Config, error) {
	return LoadWithOptions(LoadOptions{})
}

func LoadWithOptions(opts LoadOptions) (*Config, error) {
	cfg := defaultConfig()

	configFile := strings.TrimSpace(opts.ConfigFile)
	if configFile == "" {
		configFile = "config.yaml"
	}

	configDir := filepath.Dir(configFile)
	if configDir == "." {
		configDir = ""
	}

	debugConfigured := false
	if raw, err := os.ReadFile(filepath.Clean(configFile)); err == nil {
		rawText := string(raw)
		debugConfigured = configHasAppDebug(rawText)
		envForDotEnv := detectAppEnv(rawText, cfg.App.Env)
		loadDotEnv(configDir, envForDotEnv)
		expanded := expandEnv(rawText)
		if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
			return nil, fmt.Errorf("unmarshal config: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read config: %w", err)
	} else {
		loadDotEnv(configDir, cfg.App.Env)
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
		Server: ServerConfig{
			ShutdownTimeout: 30,
			ReadTimeout:     30,
			WriteTimeout:    30,
			MaxHeaderBytes:  1 << 20,
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
				Port:            "5432",
				SSLMode:         "disable",
				MaxConnections:  20,
				MaxIdleConns:    10,
				ConnMaxLifetime: 3600,
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
		Casbin: CasbinConfig{
			Enforcers: map[string]CasbinEnforcerConfig{},
		},
		Storage: StorageConfig{
			Default: "local",
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
			Enabled:     true,
			Title:       "Grove API",
			Description: "API framework scaffold for interface-driven services",
			Version:     "1.0.0",
			BasePath:    "/api/v1",
			Schemes:     []string{"http"},
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
	}
}

func loadDotEnv(configDir, env string) {
	if strings.EqualFold(env, "production") {
		return
	}

	candidates := []string{".env"}
	if configDir != "" {
		candidates = append([]string{filepath.Join(configDir, ".env")}, candidates...)
	}

	for _, candidate := range candidates {
		raw, err := os.ReadFile(filepath.Clean(candidate))
		if err != nil {
			continue
		}
		lines := bytes.Split(raw, []byte("\n"))
		for _, line := range lines {
			trimmed := strings.TrimSpace(string(line))
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				continue
			}
			parts := strings.SplitN(trimmed, "=", 2)
			if len(parts) != 2 {
				continue
			}
			key := strings.TrimSpace(parts[0])
			value := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
			if os.Getenv(key) == "" {
				_ = os.Setenv(key, value)
			}
		}
		return
	}
}

func detectAppEnv(rawConfig, fallback string) string {
	if value := strings.TrimSpace(os.Getenv("APP_ENV")); value != "" {
		return value
	}
	var partial struct {
		App AppConfig `yaml:"app"`
	}
	expanded := expandEnv(rawConfig)
	if err := yaml.Unmarshal([]byte(expanded), &partial); err == nil {
		if value := strings.TrimSpace(partial.App.Env); value != "" {
			return value
		}
	}
	return fallback
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
	if strings.TrimSpace(c.API.Prefix) == "" {
		c.API.Prefix = "/api/v1"
	}
	if c.Databases.Resources == nil {
		c.Databases.Resources = map[string]DatabaseConfig{}
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
	if len(c.Docs.Schemes) == 0 {
		c.Docs.Schemes = []string{"http"}
	}
}

func (c Config) Validate(service string) error {
	if err := validatePort("port", c.Port); err != nil {
		return err
	}
	if err := validatePort("console_port", c.ConsolePort); err != nil {
		return err
	}
	if strings.EqualFold(strings.TrimSpace(c.App.Env), "production") {
		secret := strings.TrimSpace(c.JWT.Secret)
		if secret == "" || secret == "change-me" || len(secret) < 32 {
			return fmt.Errorf("jwt secret must be set to a strong value in production")
		}
	}
	if c.Job.Enabled && !c.Redis.Enabled {
		return fmt.Errorf("job requires redis to be enabled")
	}
	if c.CORS.AllowCredentials && containsString(c.CORS.AllowedOrigins, "*") {
		return fmt.Errorf("cors allow_credentials cannot be used with wildcard origin")
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
	_ = service
	return nil
}

func validatePort(name, value string) error {
	port, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("%s must be a valid port", name)
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
