package config

// defaultCasbinAutoLoadSeconds keeps replicas within half a minute of a policy
// change without making every request hit the database.
const defaultCasbinAutoLoadSeconds = 30

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
			Level:      "info",
			Path:       "./logs",
			Console:    true,
			MaxSizeMB:  100,
			MaxAgeDays: 14,
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
