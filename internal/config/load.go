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

	loadDotEnv(configDir, cfg.App.Env)

	if raw, err := os.ReadFile(filepath.Clean(configFile)); err == nil {
		expanded := expandEnv(string(raw))
		if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
			return nil, fmt.Errorf("unmarshal config: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read config: %w", err)
	}

	applyEnvironmentOverrides(&cfg)
	cfg.normalize(opts.Service)
	return &cfg, nil
}

func defaultConfig() Config {
	return Config{
		App: AppConfig{
			Name: "grove",
			Env:  "development",
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
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false
	}
	return parsed
}

func parseInt(value string, fallback int) int {
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func (c *Config) normalize(service string) {
	if strings.TrimSpace(c.App.Name) == "" {
		c.App.Name = "grove"
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
