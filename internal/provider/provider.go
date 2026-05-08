package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/zhimma/grove/internal/config"
	"github.com/zhimma/grove/pkg/auth"
	"github.com/zhimma/grove/pkg/cache"
	pkgcasbin "github.com/zhimma/grove/pkg/casbin"
	"github.com/zhimma/grove/pkg/database"
	"github.com/zhimma/grove/pkg/event"
	"github.com/zhimma/grove/pkg/httpclient"
	"github.com/zhimma/grove/pkg/job"
	"github.com/zhimma/grove/pkg/logger"
	"github.com/zhimma/grove/pkg/scheduler"
	"github.com/zhimma/grove/pkg/storage"
	"github.com/zhimma/grove/pkg/transaction"
)

type Provider struct {
	Config       *config.Config
	DB           database.Repo
	RedisClient  *redis.Client
	TokenManager *auth.Manager
	JobClient    *job.Client
	JobServer    *job.Server
	Enforcers    map[string]*pkgcasbin.Enforcer
	Storage      *storage.Manager
	TxManager    transaction.Manager
	Cache        *cache.Manager
	HTTPClient   *httpclient.Client
	Event        *event.Dispatcher
	Scheduler    *scheduler.Scheduler
}

type Option func(*Provider) error

func APIOptions() []Option {
	return []Option{
		WithDatabase(),
		WithRedis(),
		WithAuth(),
		WithJob(),
		WithCasbin(),
		WithStorage(),
		WithTransaction(),
		WithCache(),
		WithHTTPClient(),
		WithEvent(),
	}
}

func ConsoleOptions() []Option {
	return []Option{
		WithDatabase(),
		WithRedis(),
		WithAuth(),
		WithCasbin(),
		WithStorage(),
		WithTransaction(),
		WithCache(),
		WithHTTPClient(),
		WithEvent(),
	}
}

func WorkerOptions() []Option {
	return []Option{
		WithRedis(),
		WithJobServer(),
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
		Level:   cfg.Log.Level,
		Path:    cfg.Log.Path,
		Service: serviceName,
		Console: cfg.Log.Console,
	}); err != nil {
		return nil, err
	}

	p := &Provider{Config: cfg}
	for _, opt := range opts {
		if err := opt(p); err != nil {
			_ = p.Close()
			return nil, err
		}
	}
	return p, nil
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
			MaxConnections:  p.Config.Databases.Default.MaxConnections,
			MaxIdleConns:    p.Config.Databases.Default.MaxIdleConns,
			ConnMaxLifetime: p.Config.Databases.Default.ConnMaxLifetime,
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
				MaxConnections:  cfg.MaxConnections,
				MaxIdleConns:    cfg.MaxIdleConns,
				ConnMaxLifetime: cfg.ConnMaxLifetime,
			}
		}
		repo, err := database.NewRepo(defaultCfg, resourceConfigs)
		if err != nil {
			return err
		}
		p.DB = repo
		return nil
	}
}

func WithCasbin() Option {
	return func(p *Provider) error {
		if p.Enforcers == nil {
			p.Enforcers = map[string]*pkgcasbin.Enforcer{}
		}
		if p.DB == nil {
			return fmt.Errorf("权限控制依赖数据库仓储")
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
			enforcer, err := pkgcasbin.New(db, &pkgcasbin.Config{
				Mode:      pkgcasbin.Mode(cfg.Mode),
				TableName: cfg.TableName,
				ModelPath: cfg.ModelPath,
			})
			if err != nil {
				return fmt.Errorf("init casbin enforcer %q: %w", name, err)
			}
			p.Enforcers[strings.TrimSpace(strings.ToLower(name))] = enforcer
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
		if strings.EqualFold(p.Config.App.Env, "production") {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if err := client.Ping(ctx).Err(); err != nil {
				_ = client.Close()
				return fmt.Errorf("redis ping: %w", err)
			}
		}
		p.RedisClient = client
		return nil
	}
}

func WithAuth() Option {
	return func(p *Provider) error {
		manager, err := auth.NewManager(
			p.Config.JWT.Secret,
			p.Config.JWT.Issuer,
			time.Duration(p.Config.JWT.AccessExpiryHours)*time.Hour,
			time.Duration(p.Config.JWT.RefreshExpiryHours)*time.Hour,
		)
		if err != nil {
			return err
		}
		p.TokenManager = manager
		return nil
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
		p.JobClient = job.NewClient(job.RedisConfig{
			Addr:     p.Config.Redis.Addr,
			Password: p.Config.Redis.Password,
			DB:       p.Config.Redis.DB,
		})
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
		p.JobServer = job.NewServer(job.RedisConfig{
			Addr:     p.Config.Redis.Addr,
			Password: p.Config.Redis.Password,
			DB:       p.Config.Redis.DB,
		}, job.ServerConfig{
			Concurrency: p.Config.Job.Concurrency,
			Queues:      p.Config.Job.Queues,
		})
		return nil
	}
}

func WithStorage() Option {
	return func(p *Provider) error {
		manager := storage.NewManager(p.Config.Storage.Default)
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
					Secret:  p.Config.JWT.Secret,
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
				Name:      diskName,
				Driver:    strings.TrimSpace(strings.ToLower(diskCfg.Driver)),
				BaseURL:   diskCfg.BaseURL,
				Endpoint:  diskCfg.Endpoint,
				Region:    diskCfg.Region,
				Bucket:    diskCfg.Bucket,
				Prefix:    diskCfg.Prefix,
				IsDefault: diskName == strings.TrimSpace(strings.ToLower(p.Config.Storage.Default)),
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

func WithTransaction() Option {
	return func(p *Provider) error {
		if p.DB == nil || p.DB.Default() == nil {
			return nil
		}
		p.TxManager = transaction.NewManager(p.DB.Default())
		return nil
	}
}

func WithCache() Option {
	return func(p *Provider) error {
		manager := cache.NewManager()

		// 注册内存缓存
		memoryStore := cache.NewMemoryStore()
		manager.Register("memory", memoryStore)
		manager.SetDefault("memory")

		// 如果Redis可用，注册Redis缓存
		if p.RedisClient != nil {
			redisStore := cache.NewRedisStore(p.RedisClient, p.Config.App.Name)
			manager.Register("redis", redisStore)
			// 默认使用Redis（如果可用）
			manager.SetDefault("redis")
		}

		p.Cache = manager
		return nil
	}
}

func WithHTTPClient() Option {
	return func(p *Provider) error {
		p.HTTPClient = httpclient.New()
		return nil
	}
}

func WithEvent() Option {
	return func(p *Provider) error {
		p.Event = event.New()
		return nil
	}
}

func WithScheduler() Option {
	return func(p *Provider) error {
		sched, err := scheduler.New(scheduler.Config{
			Location: "Local",
		})
		if err != nil {
			return fmt.Errorf("init scheduler: %w", err)
		}
		p.Scheduler = sched
		return nil
	}
}

func (p *Provider) Close() error {
	if p == nil {
		return nil
	}
	var errs []error
	if p.Scheduler != nil {
		p.Scheduler.Stop()
	}
	if p.JobServer != nil {
		p.JobServer.Shutdown()
	}
	if p.JobClient != nil {
		if err := p.JobClient.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if p.RedisClient != nil {
		if err := p.RedisClient.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if p.DB != nil {
		if err := p.DB.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (p *Provider) GetEnforcer(name string) *pkgcasbin.Enforcer {
	if p == nil || p.Enforcers == nil {
		return nil
	}
	return p.Enforcers[strings.TrimSpace(strings.ToLower(name))]
}
