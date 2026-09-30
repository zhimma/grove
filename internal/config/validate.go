package config

import (
	"fmt"
	"net"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
)

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
	if c.Log.MaxSizeMB <= 0 || c.Log.MaxAgeDays < 0 {
		return fmt.Errorf("log.max_size_mb must be positive and log.max_age_days cannot be negative")
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
		if enforcer.AutoLoadSeconds < 0 {
			return fmt.Errorf("casbin enforcer %q auto_load_seconds must not be negative", name)
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
