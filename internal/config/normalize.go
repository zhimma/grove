package config

import (
	"strings"
)

func (c *Config) normalize(debugConfigured bool) {
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
	for name, enforcerCfg := range c.Casbin.Enforcers {

		if enforcerCfg.AutoLoadSeconds == 0 {
			enforcerCfg.AutoLoadSeconds = defaultCasbinAutoLoadSeconds
			c.Casbin.Enforcers[name] = enforcerCfg
		}
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
