package provider

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zhimma/grove/internal/config"
	"github.com/zhimma/grove/pkg/cache"
	"github.com/zhimma/grove/pkg/database"
	"github.com/zhimma/grove/pkg/event"
)

type providerCloseStore struct {
	closed atomic.Bool
}

type providerCloseEvent struct{}

func (providerCloseEvent) EventName() string { return "provider.close" }

func (*providerCloseStore) Get(context.Context, string) ([]byte, bool, error) {
	return nil, false, nil
}
func (*providerCloseStore) Set(context.Context, string, []byte, time.Duration) error { return nil }
func (*providerCloseStore) Delete(context.Context, string) error                     { return nil }
func (*providerCloseStore) Add(context.Context, string, []byte, time.Duration) (bool, error) {
	return true, nil
}
func (*providerCloseStore) TTL(context.Context, string) (time.Duration, bool, error) {
	return 0, false, nil
}
func (s *providerCloseStore) Close() error {
	s.closed.Store(true)
	return nil
}

func TestServiceOptionSets(t *testing.T) {
	if len(APIOptions()) == 0 {
		t.Fatal("expected api options")
	}
	if len(ConsoleOptions()) == 0 {
		t.Fatal("expected console options")
	}
	if len(WorkerOptions()) == 0 {
		t.Fatal("expected worker options")
	}
}

func TestAPIAndConsoleOptionsIncludeCoreConveniences(t *testing.T) {
	if !optionSetContainsAtLeast(APIOptions(), 10) {
		t.Fatalf("expected api options to include core convenience components")
	}
	if !optionSetContainsAtLeast(ConsoleOptions(), 9) {
		t.Fatalf("expected console options to include redis/cache/http/event components")
	}
}

func TestNewRequiresConfig(t *testing.T) {
	provider, err := New(nil, "api")
	if err == nil {
		t.Fatal("expected error when config is nil")
	}
	if provider != nil {
		t.Fatal("expected nil provider when config is nil")
	}
}

func TestWithCasbinRequiresDatabaseConnections(t *testing.T) {
	p := &Provider{
		Config: &config.Config{
			Casbin: config.CasbinConfig{
				Enforcers: map[string]config.CasbinEnforcerConfig{
					"api": {
						Enabled:   true,
						Database:  "default",
						Mode:      "rbac",
						TableName: "casbin_rules",
					},
				},
			},
		},
	}

	err := WithCasbin()(p)
	if err == nil {
		t.Fatal("expected casbin init to fail without database connections")
	}
	if err.Error() != "权限控制依赖数据库连接" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetEnforcerReturnsNilWhenMissing(t *testing.T) {
	p := &Provider{
		DB: database.NewConnectionsFromDBs(nil, nil),
	}

	if got := p.GetEnforcer("api"); got != nil {
		t.Fatal("expected nil enforcer")
	}
}

func TestWithJobRequiresRedis(t *testing.T) {
	p := &Provider{
		Config: &config.Config{
			Redis: config.RedisConfig{Enabled: false},
			Job:   config.JobConfig{Enabled: true},
		},
	}

	err := WithJob()(p)
	if err == nil {
		t.Fatal("expected job init to fail without redis")
	}
	if err.Error() != "任务队列已启用，但 Redis 未启用" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewFallsBackToAppNameWhenServiceNameEmpty(t *testing.T) {
	cfg := &config.Config{
		App: config.AppConfig{Name: "grove", Env: "test"},
		Log: config.LogConfig{
			Level:   "error",
			Path:    t.TempDir(),
			Console: false,
		},
	}

	provider, err := New(cfg, "")
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	t.Cleanup(func() { _ = provider.Close() })

	if provider.Config.App.Name != "grove" {
		t.Fatalf("expected app name grove, got %s", provider.Config.App.Name)
	}
}

func TestWithConfigSecretsIsOptionalAndValidatesConfiguredKey(t *testing.T) {
	p := &Provider{Config: &config.Config{}}
	if err := WithConfigSecrets()(p); err != nil || p.ConfigSecrets != nil {
		t.Fatalf("empty key should leave secret box disabled: box=%v err=%v", p.ConfigSecrets, err)
	}

	p.Config.Security.ConfigEncryptionKey = "short"
	if err := WithConfigSecrets()(p); err == nil {
		t.Fatal("weak configured key must fail provider initialization")
	}

	p.Config.Security.ConfigEncryptionKey = "0123456789abcdef0123456789abcdef"
	if err := WithConfigSecrets()(p); err != nil || p.ConfigSecrets == nil {
		t.Fatalf("strong key should initialize secret box: box=%v err=%v", p.ConfigSecrets, err)
	}
}

func TestProviderCloseClosesCacheManager(t *testing.T) {
	manager := cache.NewManager()
	store := &providerCloseStore{}
	manager.Register("tracking", store)
	p := &Provider{Cache: manager}
	p.AddCloser("cache", manager.Close)
	if err := p.Close(); err != nil {
		t.Fatalf("close provider: %v", err)
	}
	if !store.closed.Load() {
		t.Fatal("provider close did not close cache manager")
	}
}

func TestProviderCloseClosesEventDispatcher(t *testing.T) {
	dispatcher := event.New()
	p := &Provider{Event: dispatcher}
	p.AddCloser("event", dispatcher.Close)
	if err := p.Close(); err != nil {
		t.Fatalf("close provider: %v", err)
	}
	if err := dispatcher.Dispatch(context.Background(), providerCloseEvent{}); !errors.Is(err, event.ErrClosed) {
		t.Fatalf("dispatch after provider close: %v", err)
	}
}

func TestProviderCloseRunsClosersInReverseOrder(t *testing.T) {
	p := &Provider{}
	var order []string
	for _, name := range []string{"logger", "database", "redis", "event"} {
		name := name
		p.AddCloser(name, func() error {
			order = append(order, name)
			return nil
		})
	}

	if err := p.Close(); err != nil {
		t.Fatalf("close provider: %v", err)
	}
	want := []string{"event", "redis", "database", "logger"}
	if !slices.Equal(order, want) {
		t.Fatalf("close order = %v, want %v", order, want)
	}
}

func TestProviderCloseAggregatesErrorsAndIsIdempotent(t *testing.T) {
	firstErr := errors.New("first close")
	secondErr := errors.New("second close")
	p := &Provider{}
	var calls atomic.Int64
	p.AddCloser("first", func() error {
		calls.Add(1)
		return firstErr
	})
	p.AddCloser("second", func() error {
		calls.Add(1)
		return secondErr
	})

	err := p.Close()
	if !errors.Is(err, firstErr) || !errors.Is(err, secondErr) {
		t.Fatalf("expected joined close errors, got %v", err)
	}
	if !strings.Contains(err.Error(), "first") || !strings.Contains(err.Error(), "second") {
		t.Fatalf("expected component names, got %v", err)
	}
	if err := p.Close(); !errors.Is(err, firstErr) || !errors.Is(err, secondErr) {
		t.Fatalf("second close should return same error, got %v", err)
	}
	if calls.Load() != 2 {
		t.Fatalf("closers called %d times", calls.Load())
	}
}

func TestNewRollsBackInitializedOptionsOnFailure(t *testing.T) {
	cfg := &config.Config{
		App: config.AppConfig{Name: "grove", Env: "test"},
		Log: config.LogConfig{Level: "error", Path: t.TempDir()},
	}
	initErr := errors.New("option failed")
	var closed atomic.Bool
	provider, err := New(cfg, "test",
		func(p *Provider) error {
			p.AddCloser("probe", func() error {
				closed.Store(true)
				return nil
			})
			return nil
		},
		func(*Provider) error { return initErr },
	)
	if provider != nil {
		t.Fatal("expected nil provider")
	}
	if !errors.Is(err, initErr) {
		t.Fatalf("expected option error, got %v", err)
	}
	if !closed.Load() {
		t.Fatal("initialized option was not rolled back")
	}
}

func TestWithSchedulerHonorsConfig(t *testing.T) {
	p := &Provider{Config: &config.Config{Server: config.ServerConfig{ShutdownTimeout: 1}}}
	if err := WithScheduler()(p); err != nil || p.Scheduler != nil {
		t.Fatalf("disabled scheduler: scheduler=%v err=%v", p.Scheduler, err)
	}

	p.Config.Scheduler.Enabled = true
	p.Config.Scheduler.Timezone = "UTC"
	if err := WithScheduler()(p); err != nil || p.Scheduler == nil {
		t.Fatalf("enabled scheduler: scheduler=%v err=%v", p.Scheduler, err)
	}
	if err := p.Scheduler.Stop(); err != nil {
		t.Fatalf("stop scheduler: %v", err)
	}
}

func TestWorkerOptionsIncludeEnabledScheduler(t *testing.T) {
	cfg := &config.Config{
		App:       config.AppConfig{Name: "grove", Env: "test"},
		Log:       config.LogConfig{Level: "error", Path: t.TempDir()},
		Server:    config.ServerConfig{ShutdownTimeout: 1},
		Scheduler: config.SchedulerConfig{Enabled: true, Timezone: "UTC"},
	}
	p, err := New(cfg, "worker", WorkerOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	if p.Scheduler == nil {
		t.Fatal("worker options did not initialize enabled scheduler")
	}
}

func optionSetContainsAtLeast(options []Option, count int) bool {
	return len(options) >= count && slices.ContainsFunc(options, func(opt Option) bool {
		return opt != nil
	})
}
