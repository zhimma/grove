package provider

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/zhimma/grove/internal/config"
	"github.com/zhimma/grove/internal/observability"
	"github.com/zhimma/grove/internal/readiness"
	"github.com/zhimma/grove/pkg/auth"
	"github.com/zhimma/grove/pkg/cache"
	"github.com/zhimma/grove/pkg/database"
	"github.com/zhimma/grove/pkg/event"
	"github.com/zhimma/grove/pkg/httpclient"
	"github.com/zhimma/grove/pkg/job"
	"github.com/zhimma/grove/pkg/logger"
	"github.com/zhimma/grove/pkg/rbac"
	"github.com/zhimma/grove/pkg/route"
	"github.com/zhimma/grove/pkg/scheduler"
	"github.com/zhimma/grove/pkg/secretbox"
	"github.com/zhimma/grove/pkg/storage"
)

type Provider struct {
	Config        *config.Config
	DB            *database.Connections
	RedisClient   *redis.Client
	Tokens        *auth.Tokens
	JobClient     *job.Client
	JobServer     *job.Server
	Enforcers     map[string]*rbac.Enforcer
	Storage       *storage.Manager
	Cache         *cache.Stores
	HTTPClient    *httpclient.Client
	Event         *event.Dispatcher
	Scheduler     *scheduler.Scheduler
	ConfigSecrets *secretbox.Box
	Observability *observability.Runtime
	RouteCatalog  *route.Catalog
	serviceName   string

	closeMu   sync.Mutex
	closers   []providerCloser
	closeOnce sync.Once
	closeErr  error
}

type providerCloser struct {
	name string
	fn   func() error
}

type Option func(*Provider) error

func APIOptions() []Option {
	return []Option{
		WithObservability(),
		WithDatabase(),
		WithRedis(),
		WithAuth(),
		WithJob(),
		WithCasbin(),
		WithStorage(),
		WithCache(),
		WithHTTPClient(),
		WithEvent(),
	}
}

func ConsoleOptions() []Option {
	return []Option{
		WithObservability(),
		WithDatabase(),
		WithRedis(),
		WithAuth(),
		WithConfigSecrets(),
		WithCasbin(),
		WithStorage(),
		WithCache(),
		WithHTTPClient(),
		WithEvent(),
	}
}

func WithConfigSecrets() Option {
	return func(p *Provider) error {
		key := strings.TrimSpace(p.Config.Security.ConfigEncryptionKey)
		if key == "" {
			return nil
		}
		box, err := secretbox.New(key)
		if err != nil {
			return fmt.Errorf("init config encryption: %w", err)
		}
		p.ConfigSecrets = box
		return nil
	}
}

func WorkerOptions() []Option {
	return []Option{
		WithObservability(),
		WithRedis(),
		WithCache(),
		WithJobServer(),
		WithScheduler(),
	}
}

func New(cfg *config.Config, serviceName string, opts ...Option) (*Provider, error) {
	if cfg == nil {
		return nil, fmt.Errorf("provider 配置不能为空")
	}
	serviceName = strings.TrimSpace(serviceName)
	if serviceName == "" {
		serviceName = strings.TrimSpace(cfg.App.Name)
	}

	if err := logger.Init(logger.Config{
		Level:      cfg.Log.Level,
		Path:       cfg.Log.Path,
		Service:    serviceName,
		Console:    cfg.Log.Console,
		MaxSizeMB:  cfg.Log.MaxSizeMB,
		MaxAgeDays: cfg.Log.MaxAgeDays,
	}); err != nil {
		return nil, err
	}

	p := &Provider{Config: cfg, RouteCatalog: route.NewCatalog(), serviceName: serviceName}
	p.AddCloser("logger", logger.Close)
	for _, opt := range opts {
		if err := opt(p); err != nil {
			if closeErr := p.Close(); closeErr != nil {
				return nil, errors.Join(err, fmt.Errorf("rollback provider: %w", closeErr))
			}
			return nil, err
		}
	}
	return p, nil
}

func WithObservability() Option {
	return func(p *Provider) error {
		runtime, err := observability.New(context.Background(), observability.Config{
			ServiceName:      p.serviceName,
			Environment:      p.Config.App.Env,
			Enabled:          p.Config.Observability.Enabled,
			MetricsEnabled:   p.Config.Observability.MetricsEnabled,
			MetricsPath:      p.Config.Observability.MetricsPath,
			TraceSampleRatio: p.Config.Observability.TraceSampleRatio,
			OTLPTraceURL:     p.Config.Observability.OTLPTraceEndpoint,
			OTLPInsecure:     p.Config.Observability.OTLPInsecure,
		})
		if err != nil {
			return err
		}
		p.Observability = runtime
		if runtime != nil {
			p.AddCloser("observability", runtime.Close)
		}
		return nil
	}
}

func WithDatabase() Option {
	return func(p *Provider) error {
		defaultCfg := database.Config{
			Enabled:         p.Config.Databases.Default.Enabled,
			Driver:          p.Config.Databases.Default.Driver,
			Host:            p.Config.Databases.Default.Host,
			Port:            p.Config.Databases.Default.Port,
			User:            p.Config.Databases.Default.User,
			Password:        p.Config.Databases.Default.Password,
			DBName:          p.Config.Databases.Default.DBName,
			SSLMode:         p.Config.Databases.Default.SSLMode,
			Charset:         p.Config.Databases.Default.Charset,
			ParseTime:       p.Config.Databases.Default.ParseTime,
			Loc:             p.Config.Databases.Default.Loc,
			TLS:             p.Config.Databases.Default.TLS,
			MaxConnections:  p.Config.Databases.Default.MaxConnections,
			MaxIdleConns:    p.Config.Databases.Default.MaxIdleConns,
			ConnMaxLifetime: p.Config.Databases.Default.ConnMaxLifetime,
			ConnectTimeout:  p.Config.Databases.Default.ConnectTimeout,
		}
		resourceConfigs := make(map[string]database.Config, len(p.Config.Databases.Resources))
		for name, cfg := range p.Config.Databases.Resources {
			resourceConfigs[name] = database.Config{
				Enabled:         cfg.Enabled,
				Driver:          cfg.Driver,
				Host:            cfg.Host,
				Port:            cfg.Port,
				User:            cfg.User,
				Password:        cfg.Password,
				DBName:          cfg.DBName,
				SSLMode:         cfg.SSLMode,
				Charset:         cfg.Charset,
				ParseTime:       cfg.ParseTime,
				Loc:             cfg.Loc,
				TLS:             cfg.TLS,
				MaxConnections:  cfg.MaxConnections,
				MaxIdleConns:    cfg.MaxIdleConns,
				ConnMaxLifetime: cfg.ConnMaxLifetime,
				ConnectTimeout:  cfg.ConnectTimeout,
			}
		}
		dbs, err := database.NewConnections(defaultCfg, resourceConfigs)
		if err != nil {
			return err
		}
		p.DB = dbs
		if p.Observability != nil {
			for _, name := range dbs.Names() {
				db, getErr := dbs.Get(name)
				if getErr != nil {
					_ = dbs.Close()
					return getErr
				}
				if err := p.Observability.InstrumentGORM(name, db); err != nil {
					_ = dbs.Close()
					return err
				}
				sqlDB, err := db.DB()
				if err != nil {
					_ = dbs.Close()
					return err
				}
				if err := p.Observability.ObserveDBPool(name, sqlDB); err != nil {
					_ = dbs.Close()
					return err
				}
			}
		}
		p.AddCloser("database", dbs.Close)
		return nil
	}
}

func WithCasbin() Option {
	return func(p *Provider) error {
		if p.Enforcers == nil {
			p.Enforcers = map[string]*rbac.Enforcer{}
		}
		if p.DB == nil {
			return fmt.Errorf("权限控制依赖数据库连接")
		}

		for name, cfg := range p.Config.Casbin.Enforcers {
			if !cfg.Enabled {
				continue
			}
			resourceName := strings.TrimSpace(cfg.Database)
			if resourceName == "" {
				resourceName = "default"
			}
			db, err := p.DB.Get(resourceName)
			if err != nil {
				return fmt.Errorf("casbin enforcer %q database %q: %w", name, resourceName, err)
			}
			enforcer, err := rbac.New(db, &rbac.Config{
				Mode:             rbac.Mode(cfg.Mode),
				TableName:        cfg.TableName,
				ModelPath:        cfg.ModelPath,
				AutoLoadInterval: time.Duration(cfg.AutoLoadSeconds) * time.Second,
			})
			if err != nil {
				return fmt.Errorf("init casbin enforcer %q: %w", name, err)
			}
			enforcerName := strings.TrimSpace(strings.ToLower(name))
			p.Enforcers[enforcerName] = enforcer
			p.AddCloser("casbin:"+enforcerName, enforcer.Close)
		}
		return nil
	}
}

func WithRedis() Option {
	return func(p *Provider) error {
		if !p.Config.Redis.Enabled {
			return nil
		}

		client := redis.NewClient(&redis.Options{
			Addr:     p.Config.Redis.Addr,
			Password: p.Config.Redis.Password,
			DB:       p.Config.Redis.DB,
		})
		if p.Observability != nil {
			client.AddHook(p.Observability.NewRedisHook())
		}
		if strings.EqualFold(p.Config.App.Env, "production") {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if err := client.Ping(ctx).Err(); err != nil {
				_ = client.Close()
				return fmt.Errorf("redis ping: %w", err)
			}
		}
		p.RedisClient = client
		p.AddCloser("redis", client.Close)
		return nil
	}
}

func WithAuth() Option {
	return func(p *Provider) error {
		manager, err := auth.NewTokens(auth.Config{
			Secret:        p.Config.JWT.Secret,
			Issuer:        serviceTokenIssuer(p.Config.JWT.Issuer, p.serviceName),
			AccessExpiry:  time.Duration(p.Config.JWT.AccessExpiryHours) * time.Hour,
			RefreshExpiry: time.Duration(p.Config.JWT.RefreshExpiryHours) * time.Hour,
		})
		if err != nil {
			return err
		}
		p.Tokens = manager
		return nil
	}
}

func serviceTokenIssuer(baseIssuer, serviceName string) string {
	baseIssuer = strings.TrimSpace(baseIssuer)
	if baseIssuer == "" {
		baseIssuer = "grove"
	}
	switch strings.ToLower(strings.TrimSpace(serviceName)) {
	case "api", "console":
		return baseIssuer + ":" + strings.ToLower(strings.TrimSpace(serviceName))
	default:
		return baseIssuer
	}
}

func WithJob() Option {
	return func(p *Provider) error {
		if !p.Config.Job.Enabled {
			return nil
		}
		if !p.Config.Redis.Enabled {
			return fmt.Errorf("任务队列已启用，但 Redis 未启用")
		}
		client := job.NewClient(job.RedisConfig{
			Addr:     p.Config.Redis.Addr,
			Password: p.Config.Redis.Password,
			DB:       p.Config.Redis.DB,
		})
		p.JobClient = client
		p.AddCloser("job client", client.Close)
		return nil
	}
}

func WithJobServer() Option {
	return func(p *Provider) error {
		if !p.Config.Job.Enabled {
			return nil
		}
		if !p.Config.Redis.Enabled {
			return fmt.Errorf("任务队列已启用，但 Redis 未启用")
		}
		server := job.NewServer(job.RedisConfig{
			Addr:     p.Config.Redis.Addr,
			Password: p.Config.Redis.Password,
			DB:       p.Config.Redis.DB,
		}, job.ServerConfig{
			Concurrency: p.Config.Job.Concurrency,
			Queues:      p.Config.Job.Queues,
		})
		p.JobServer = server
		p.AddCloser("job server", func() error {
			server.Shutdown()
			return nil
		})
		return nil
	}
}

func WithStorage() Option {
	return func(p *Provider) error {
		manager := storage.NewManager(p.Config.Storage.Default)
		for name, policyCfg := range p.Config.Storage.UploadPolicies {
			policy, err := storage.NewUploadPolicy(storage.UploadPolicyConfig{
				Name:       name,
				Directory:  policyCfg.Directory,
				MaxBytes:   policyCfg.MaxBytes,
				Extensions: policyCfg.Extensions,
				MIMETypes:  policyCfg.MIMETypes,
			})
			if err != nil {
				return fmt.Errorf("init upload policy %q: %w", name, err)
			}
			manager.AddUploadPolicy(policy)
		}
		manager.SetDefaultUploadPolicy(p.Config.Storage.DefaultUploadPolicy)
		for name, diskCfg := range p.Config.Storage.Disks {
			diskName := strings.TrimSpace(strings.ToLower(name))
			if diskName == "" {
				continue
			}

			var (
				driver    storage.Driver
				stsIssuer storage.STSProvider
				err       error
			)

			switch strings.TrimSpace(strings.ToLower(diskCfg.Driver)) {
			case "local":
				driver, err = storage.NewLocalDriver(storage.LocalConfig{
					Root:    diskCfg.Root,
					BaseURL: diskCfg.BaseURL,
				})
			case "s3", "aws":
				driver, err = storage.NewS3Driver(storage.S3Config{
					Endpoint:  diskCfg.Endpoint,
					Region:    diskCfg.Region,
					Bucket:    diskCfg.Bucket,
					AccessKey: diskCfg.AccessKey,
					SecretKey: diskCfg.SecretKey,
					Secure:    diskCfg.Secure,
					BaseURL:   diskCfg.BaseURL,
				})
				if err == nil && diskCfg.STS.Enabled {
					stsIssuer, err = storage.NewAWSSTSProvider(storage.STSServiceConfig{
						Endpoint:        diskCfg.STS.Endpoint,
						Region:          diskCfg.STS.Region,
						Bucket:          diskCfg.Bucket,
						AccessKey:       diskCfg.AccessKey,
						SecretKey:       diskCfg.SecretKey,
						RoleARN:         diskCfg.STS.RoleARN,
						RoleSessionName: diskCfg.STS.RoleSession,
						Duration:        diskCfg.STS.Duration,
						AllowPrefix:     diskCfg.STS.AllowPrefix,
						AllowActions:    diskCfg.STS.AllowActions,
					})
				}
			default:
				return fmt.Errorf("storage disk %q unsupported driver %q", diskName, diskCfg.Driver)
			}
			if err != nil {
				return fmt.Errorf("init storage disk %q: %w", diskName, err)
			}

			manager.AddDisk(diskName, driver, storage.DiskConfig{
				Name:        diskName,
				Driver:      driver.Name(),
				BaseURL:     diskCfg.BaseURL,
				Public:      diskCfg.Public,
				ServeStatic: diskCfg.ServeStatic,
				Endpoint:    diskCfg.Endpoint,
				Region:      diskCfg.Region,
				Bucket:      diskCfg.Bucket,
				Prefix:      diskCfg.Prefix,
				IsDefault:   diskName == strings.TrimSpace(strings.ToLower(p.Config.Storage.Default)),
			}, stsIssuer)
		}
		if len(manager.Names()) == 0 {
			return fmt.Errorf("no storage disks configured")
		}
		if _, err := manager.Get(p.Config.Storage.Default); err != nil {
			return err
		}
		p.Storage = manager
		return nil
	}
}

func WithCache() Option {
	return func(p *Provider) error {
		manager := cache.NewStores()

		// 注册内存缓存
		memoryStore := cache.NewMemoryStore()
		manager.Register("memory", memoryStore)
		manager.SetDefault("memory")

		// 如果Redis可用，注册Redis缓存
		if p.RedisClient != nil {
			parts := []string{strings.TrimSpace(p.Config.App.Name), strings.TrimSpace(p.Config.App.Env), p.serviceName}
			parts = slices.DeleteFunc(parts, func(value string) bool { return value == "" })
			redisStore := cache.NewRedisStore(p.RedisClient, strings.Join(parts, ":"))
			manager.Register("redis", redisStore)
			// 默认使用Redis（如果可用）
			manager.SetDefault("redis")
		}

		p.Cache = manager
		p.AddCloser("cache", manager.Close)
		return nil
	}
}

func WithHTTPClient() Option {
	return func(p *Provider) error {
		p.HTTPClient = httpclient.New(httpclient.DefaultConfig())
		if p.Observability != nil {
			p.HTTPClient = p.HTTPClient.WithTracing()
		}
		return nil
	}
}

func WithEvent() Option {
	return func(p *Provider) error {
		dispatcher := event.New(event.DefaultConfig())
		p.Event = dispatcher
		p.AddCloser("event", dispatcher.Close)
		return nil
	}
}

func WithScheduler() Option {
	return func(p *Provider) error {
		if !p.Config.Scheduler.Enabled {
			return nil
		}
		// Mutex tasks need a lock every worker can see. The Redis cache store is
		// that shared thing when Redis is configured; without it the lock stays
		// nil and Mutex is process-local, which only holds for a single worker.
		var clusterLock cache.Store
		if p.Cache != nil && p.RedisClient != nil {
			clusterLock = p.Cache.Store("redis")
		}
		sched, err := scheduler.New(scheduler.Config{
			Location:    p.Config.Scheduler.Timezone,
			StopTimeout: time.Duration(p.Config.Server.ShutdownTimeout) * time.Second,
			Lock:        clusterLock,
		})
		if err != nil {
			return fmt.Errorf("init scheduler: %w", err)
		}
		p.Scheduler = sched
		p.AddCloser("scheduler", sched.Stop)
		return nil
	}
}

func (p *Provider) Close() error {
	if p == nil {
		return nil
	}
	p.closeOnce.Do(func() {
		p.closeMu.Lock()
		closers := append([]providerCloser(nil), p.closers...)
		p.closers = nil
		p.closeMu.Unlock()

		var errs []error
		for i := len(closers) - 1; i >= 0; i-- {
			closer := closers[i]
			if err := closer.fn(); err != nil {
				errs = append(errs, fmt.Errorf("close %s: %w", closer.name, err))
			}
		}
		p.closeErr = errors.Join(errs...)
	})
	return p.closeErr
}

// AddCloser registers cleanup for a successfully initialized Option.
// Provider calls registered functions in reverse order during Close.
func (p *Provider) AddCloser(name string, fn func() error) {
	if p == nil || fn == nil {
		return
	}
	p.closeMu.Lock()
	p.closers = append(p.closers, providerCloser{name: strings.TrimSpace(name), fn: fn})
	p.closeMu.Unlock()
}

func (p *Provider) GetEnforcer(name string) *rbac.Enforcer {
	if p == nil || p.Enforcers == nil {
		return nil
	}
	return p.Enforcers[strings.TrimSpace(strings.ToLower(name))]
}

func (p *Provider) ReadinessChecks() map[string]readiness.Check {
	checks := map[string]readiness.Check{}
	if p == nil || p.Config == nil {
		return checks
	}

	addDatabaseCheck := func(name string) {
		checkName := "database." + name
		checks[checkName] = func(ctx context.Context) error {
			if p.DB == nil {
				return fmt.Errorf("database connections are not initialized")
			}
			db, err := p.DB.Get(name)
			if err != nil {
				return err
			}
			sqlDB, err := db.DB()
			if err != nil {
				return err
			}
			return sqlDB.PingContext(ctx)
		}
	}
	if p.DB != nil {
		for _, name := range p.DB.Names() {
			addDatabaseCheck(name)
		}
	}
	if p.RedisClient != nil {
		checks["redis"] = func(ctx context.Context) error {
			return p.RedisClient.Ping(ctx).Err()
		}
	}
	if p.Config.Job.Enabled {
		checks["queue"] = func(ctx context.Context) error {
			if p.RedisClient == nil {
				return fmt.Errorf("queue backend is not initialized")
			}
			return p.RedisClient.Ping(ctx).Err()
		}
	}
	return checks
}
