package config

type LoadOptions struct {
	Service    string
	ConfigFile string
}

type Config struct {
	App           AppConfig           `yaml:"app"`
	Port          string              `yaml:"port"`
	ConsolePort   string              `yaml:"console_port"`
	WorkerPort    string              `yaml:"worker_port"`
	Server        ServerConfig        `yaml:"server"`
	Log           LogConfig           `yaml:"log"`
	Databases     DatabasesConfig     `yaml:"databases"`
	Redis         RedisConfig         `yaml:"redis"`
	JWT           JWTConfig           `yaml:"jwt"`
	Job           JobConfig           `yaml:"job"`
	Scheduler     SchedulerConfig     `yaml:"scheduler"`
	Casbin        CasbinConfig        `yaml:"casbin"`
	Storage       StorageConfig       `yaml:"storage"`
	Docs          DocsConfig          `yaml:"docs"`
	CORS          CORSConfig          `yaml:"cors"`
	API           APIConfig           `yaml:"api"`
	Demo          DemoConfig          `yaml:"demo"`
	Security      SecurityConfig      `yaml:"security"`
	Observability ObservabilityConfig `yaml:"observability"`
}

type AppConfig struct {
	Name  string `yaml:"name"`
	Env   string `yaml:"env"`
	Debug bool   `yaml:"debug"`
}

type ServerConfig struct {
	ShutdownTimeout int   `yaml:"shutdown_timeout"`
	ReadTimeout     int   `yaml:"read_timeout"`
	WriteTimeout    int   `yaml:"write_timeout"`
	MaxHeaderBytes  int   `yaml:"max_header_bytes"`
	MaxBodyBytes    int64 `yaml:"max_body_bytes"`
}

type LogConfig struct {
	Level   string `yaml:"level"`
	Path    string `yaml:"path"`
	Console bool   `yaml:"console"`
	Service string `yaml:"service"`
}

type DatabasesConfig struct {
	Default   DatabaseConfig            `yaml:"default"`
	Resources map[string]DatabaseConfig `yaml:"resources"`
}

type DatabaseConfig struct {
	Enabled         bool   `yaml:"enabled"`
	Driver          string `yaml:"driver"`
	Host            string `yaml:"host"`
	Port            string `yaml:"port"`
	User            string `yaml:"user"`
	Password        string `yaml:"password"`
	DBName          string `yaml:"dbname"`
	SSLMode         string `yaml:"ssl_mode"`
	MaxConnections  int    `yaml:"max_connections"`
	MaxIdleConns    int    `yaml:"max_idle_conns"`
	ConnMaxLifetime int    `yaml:"conn_max_lifetime"`
}

type RedisConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

type JWTConfig struct {
	Secret             string `yaml:"secret"`
	Issuer             string `yaml:"issuer"`
	AccessExpiryHours  int    `yaml:"access_expiry_hours"`
	RefreshExpiryHours int    `yaml:"refresh_expiry_hours"`
}

type JobConfig struct {
	Enabled     bool           `yaml:"enabled"`
	Concurrency int            `yaml:"concurrency"`
	Queues      map[string]int `yaml:"queues"`
}

type SchedulerConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Timezone string `yaml:"timezone"`
}

type ObservabilityConfig struct {
	Enabled           bool    `yaml:"enabled"`
	MetricsEnabled    bool    `yaml:"metrics_enabled"`
	MetricsPath       string  `yaml:"metrics_path"`
	ReadinessTimeout  int     `yaml:"readiness_timeout"`
	TraceSampleRatio  float64 `yaml:"trace_sample_ratio"`
	OTLPTraceEndpoint string  `yaml:"otlp_trace_endpoint"`
	OTLPInsecure      bool    `yaml:"otlp_insecure"`
}

type CasbinConfig struct {
	Enforcers map[string]CasbinEnforcerConfig `yaml:"enforcers"`
}

type CasbinEnforcerConfig struct {
	Enabled   bool   `yaml:"enabled"`
	Database  string `yaml:"database"`
	Mode      string `yaml:"mode"`
	TableName string `yaml:"table_name"`
	ModelPath string `yaml:"model_path"`
}

type StorageConfig struct {
	Default             string                        `yaml:"default"`
	Disks               map[string]StorageDiskConfig  `yaml:"disks"`
	DefaultUploadPolicy string                        `yaml:"default_upload_policy"`
	UploadPolicies      map[string]UploadPolicyConfig `yaml:"upload_policies"`
}

type UploadPolicyConfig struct {
	Directory  string   `yaml:"directory"`
	MaxBytes   int64    `yaml:"max_bytes"`
	Extensions []string `yaml:"extensions"`
	MIMETypes  []string `yaml:"mime_types"`
}

type StorageDiskConfig struct {
	Driver    string           `yaml:"driver"`
	Root      string           `yaml:"root"`
	BaseURL   string           `yaml:"base_url"`
	Endpoint  string           `yaml:"endpoint"`
	Region    string           `yaml:"region"`
	Bucket    string           `yaml:"bucket"`
	AccessKey string           `yaml:"access_key"`
	SecretKey string           `yaml:"secret_key"`
	Secure    bool             `yaml:"secure"`
	Prefix    string           `yaml:"prefix"`
	STS       StorageSTSConfig `yaml:"sts"`
}

type StorageSTSConfig struct {
	Enabled      bool     `yaml:"enabled"`
	Endpoint     string   `yaml:"endpoint"`
	Region       string   `yaml:"region"`
	RoleARN      string   `yaml:"role_arn"`
	RoleSession  string   `yaml:"role_session_name"`
	Duration     int32    `yaml:"duration"`
	AllowPrefix  []string `yaml:"allow_prefix"`
	AllowActions []string `yaml:"allow_actions"`
}

type DocsConfig struct {
	Enabled         bool     `yaml:"enabled"`
	Title           string   `yaml:"title"`
	Description     string   `yaml:"description"`
	Version         string   `yaml:"version"`
	BasePath        string   `yaml:"base_path"`
	Schemes         []string `yaml:"schemes"`
	ScalarScriptURL string   `yaml:"scalar_script_url"`
}

type CORSConfig struct {
	Enabled          bool     `yaml:"enabled"`
	AllowedOrigins   []string `yaml:"allowed_origins"`
	AllowedMethods   []string `yaml:"allowed_methods"`
	AllowedHeaders   []string `yaml:"allowed_headers"`
	AllowCredentials bool     `yaml:"allow_credentials"`
	MaxAge           int      `yaml:"max_age"`
}

type APIConfig struct {
	Prefix         string `yaml:"prefix"`
	DefaultPerPage int    `yaml:"default_per_page"`
	MaxPerPage     int    `yaml:"max_per_page"`
}

type DemoConfig struct {
	Enabled bool `yaml:"enabled"`
}

type SecurityConfig struct {
	TrustedProxies      []string              `yaml:"trusted_proxies"`
	HSTSEnabled         bool                  `yaml:"hsts_enabled"`
	ConfigEncryptionKey string                `yaml:"config_encryption_key"`
	Login               LoginProtectionConfig `yaml:"login"`
}

type LoginProtectionConfig struct {
	Enabled           bool `yaml:"enabled"`
	AttemptsPerMinute int  `yaml:"attempts_per_minute"`
	Burst             int  `yaml:"burst"`
	FailureLimit      int  `yaml:"failure_limit"`
	LockSeconds       int  `yaml:"lock_seconds"`
}
