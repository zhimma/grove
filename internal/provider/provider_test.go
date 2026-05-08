package provider

import (
	"slices"
	"testing"

	"github.com/zhimma/grove/internal/config"
	"github.com/zhimma/grove/pkg/database"
)

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

func TestWithCasbinRequiresDatabaseRepo(t *testing.T) {
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
		t.Fatal("expected casbin init to fail without database repo")
	}
	if err.Error() != "权限控制依赖数据库仓储" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetEnforcerReturnsNilWhenMissing(t *testing.T) {
	p := &Provider{
		DB: database.NewRepoWithConnections(nil, nil),
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

func optionSetContainsAtLeast(options []Option, count int) bool {
	return len(options) >= count && slices.ContainsFunc(options, func(opt Option) bool {
		return opt != nil
	})
}
