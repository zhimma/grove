package config

import (
	"os"
	"regexp"
	"strconv"
	"strings"
)

var envPattern = regexp.MustCompile(`\$\{([A-Z][A-Z0-9_]*)(?::([^}]*))?\}`)

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
