package rbac

import (
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	rawcasbin "github.com/casbin/casbin/v3"
	casbinmodel "github.com/casbin/casbin/v3/model"
	"github.com/casbin/casbin/v3/persist"
	gormadapter "github.com/casbin/gorm-adapter/v3"
	"gorm.io/gorm"
)

type Mode string

const (
	ModeRBAC        Mode = "rbac"
	ModeRBACDomains Mode = "rbac_domains"
)

type Config struct {
	Mode      Mode
	TableName string
	ModelPath string
	// AutoLoadInterval makes the enforcer re-read its policy from the adapter
	// on a timer. Without it a replica keeps serving the policy it loaded at
	// boot, so a role edit made on one instance never reaches the others.
	// ponytail: timer reload, not a Redis watcher — changes take up to one
	// interval to propagate. Swap in a casbin watcher if that ever matters.
	AutoLoadInterval time.Duration
}

type Enforcer struct {
	*rawcasbin.SyncedEnforcer
	mode      Mode
	tableName string
	version   atomic.Uint64
}

func New(db *gorm.DB, cfg *Config) (*Enforcer, error) {
	if db == nil {
		return nil, fmt.Errorf("casbin database is required")
	}

	mode := ModeRBAC
	tableName := "casbin_rules"
	if cfg != nil {
		if cfg.Mode != "" {
			mode = cfg.Mode
		}
		if strings.TrimSpace(cfg.TableName) != "" {
			tableName = strings.TrimSpace(cfg.TableName)
		}
	}

	gormadapter.TurnOffAutoMigrate(db)
	adapter, err := gormadapter.NewAdapterByDBUseTableName(db, "", tableName)
	if err != nil {
		return nil, fmt.Errorf("create casbin adapter: %w", err)
	}

	model, err := casbinmodel.NewModelFromString(defaultModel(mode))
	if err != nil {
		return nil, fmt.Errorf("create casbin model: %w", err)
	}
	if cfg != nil && strings.TrimSpace(cfg.ModelPath) != "" {
		model, err = casbinmodel.NewModelFromFile(cfg.ModelPath)
		if err != nil {
			return nil, fmt.Errorf("load casbin model from file: %w", err)
		}
	}

	enforcer, err := rawcasbin.NewSyncedEnforcer(model, adapter)
	if err != nil {
		return nil, fmt.Errorf("create casbin enforcer: %w", err)
	}
	if err := enforcer.LoadPolicy(); err != nil {
		return nil, fmt.Errorf("load casbin policy: %w", err)
	}
	if cfg != nil && cfg.AutoLoadInterval > 0 {
		enforcer.StartAutoLoadPolicy(cfg.AutoLoadInterval)
	}

	return &Enforcer{
		SyncedEnforcer: enforcer,
		mode:           mode,
		tableName:      tableName,
	}, nil
}

func (e *Enforcer) Mode() Mode {
	if e == nil {
		return ModeRBAC
	}
	return e.mode
}

func (e *Enforcer) TableName() string {
	if e == nil {
		return ""
	}
	return e.tableName
}

func (e *Enforcer) Can(subject, permission string) (bool, error) {
	if e == nil || e.SyncedEnforcer == nil {
		return false, fmt.Errorf("casbin enforcer is not configured")
	}
	return e.Enforce(subject, permission)
}

func (e *Enforcer) CanInDomain(domain, subject, permission string) (bool, error) {
	if e == nil || e.SyncedEnforcer == nil {
		return false, fmt.Errorf("casbin enforcer is not configured")
	}
	return e.Enforce(domain, subject, permission)
}

// RefreshPolicy reloads the adapter state for this process. Callers can use
// the monotonically increasing version to observe that a refresh occurred;
// cross-process propagation remains an explicit deployment concern.
func (e *Enforcer) RefreshPolicy() error {
	if e == nil || e.SyncedEnforcer == nil {
		return fmt.Errorf("casbin enforcer is not configured")
	}
	lock := e.GetLock()
	lock.Lock()
	defer lock.Unlock()
	if err := e.SyncedEnforcer.Enforcer.LoadPolicy(); err != nil {
		return fmt.Errorf("reload casbin policy: %w", err)
	}
	e.version.Add(1)
	return nil
}

func (e *Enforcer) PolicyVersion() uint64 {
	if e == nil {
		return 0
	}
	return e.version.Load()
}

// Deprecated: Use Can instead.
func (e *Enforcer) CheckConsolePermission(userID, permission string) (bool, error) {
	return e.Can(userID, permission)
}

func (e *Enforcer) AddConsolePolicies(rules [][]string) error {
	_, err := e.AddPolicies(rules)
	return err
}

func (e *Enforcer) ReplaceConsolePoliciesForRole(roleID string, permissions []string) error {
	roleID = strings.TrimSpace(roleID)
	if roleID == "" {
		return fmt.Errorf("console role ID is required")
	}
	rules := make([][]string, 0, len(permissions))
	seen := make(map[string]struct{}, len(permissions))
	for _, permission := range permissions {
		permission = strings.TrimSpace(permission)
		if permission == "" {
			continue
		}
		if _, ok := seen[permission]; ok {
			continue
		}
		seen[permission] = struct{}{}
		rules = append(rules, []string{roleID, permission})
	}
	if e == nil || e.SyncedEnforcer == nil {
		return fmt.Errorf("casbin enforcer is not configured")
	}
	if _, ok := e.GetAdapter().(persist.UpdatableAdapter); !ok {
		return fmt.Errorf("casbin adapter does not support atomic filtered policy replacement")
	}
	_, err := e.UpdateFilteredPolicies(rules, 0, roleID)
	return err
}

func (e *Enforcer) RemoveConsolePoliciesForRole(roleID string) error {
	_, err := e.RemoveFilteredPolicy(0, roleID)
	return err
}

func (e *Enforcer) GetConsolePoliciesForRole(roleID string) ([][]string, error) {
	return e.GetFilteredPolicy(0, roleID)
}

func (e *Enforcer) AddConsoleRoleForUser(userID, roleID string) error {
	_, err := e.AddGroupingPolicy(userID, roleID)
	return err
}

func (e *Enforcer) ReplaceConsoleRoleForUser(userID, roleID string) error {
	userID = strings.TrimSpace(userID)
	roleID = strings.TrimSpace(roleID)
	if userID == "" {
		return fmt.Errorf("console user ID is required")
	}
	rules := make([][]string, 0, 1)
	if roleID != "" {
		rules = append(rules, []string{userID, roleID})
	}
	return e.replaceFilteredPolicies("g", "g", rules, 0, userID)
}

func (e *Enforcer) DeleteConsoleRolesForUser(userID string) error {
	_, err := e.RemoveFilteredGroupingPolicy(0, userID)
	return err
}

func (e *Enforcer) replaceFilteredPolicies(sec, ptype string, rules [][]string, fieldIndex int, fieldValues ...string) error {
	if e == nil || e.SyncedEnforcer == nil {
		return fmt.Errorf("casbin enforcer is not configured")
	}
	lock := e.GetLock()
	lock.Lock()
	defer lock.Unlock()

	adapter, ok := e.GetAdapter().(persist.UpdatableAdapter)
	if !ok {
		return fmt.Errorf("casbin adapter does not support atomic filtered policy replacement")
	}
	if _, err := adapter.UpdateFilteredPolicies(sec, ptype, rules, fieldIndex, fieldValues...); err != nil {
		return err
	}
	if err := e.SyncedEnforcer.Enforcer.LoadPolicy(); err != nil {
		return fmt.Errorf("reload casbin policy after replacement: %w", err)
	}
	return nil
}

func defaultModel(mode Mode) string {
	switch mode {
	case ModeRBACDomains:
		return `
[request_definition]
r = dom, sub, obj

[policy_definition]
p = dom, sub, obj

[role_definition]
g = _, _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub, r.dom) && r.dom == p.dom && r.obj == p.obj
`
	default:
		return `
[request_definition]
r = sub, obj

[policy_definition]
p = sub, obj

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub) && r.obj == p.obj
`
	}
}

// Close stops the policy auto-reload goroutine. Safe on a nil Enforcer and on
// one that never started auto-loading.
func (e *Enforcer) Close() error {
	if e == nil || e.SyncedEnforcer == nil {
		return nil
	}
	e.StopAutoLoadPolicy()
	return nil
}
