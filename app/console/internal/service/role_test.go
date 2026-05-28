package service

import (
	"context"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/zhimma/grove/internal/datatype"
	"github.com/zhimma/grove/internal/model"
	"github.com/zhimma/grove/pkg/rbac"
	"github.com/zhimma/grove/pkg/database"
)

type countingTxManager struct {
	count int
}

func (m *countingTxManager) Execute(ctx context.Context, fn func(ctx context.Context) error) error {
	m.count++
	return fn(ctx)
}

func (m *countingTxManager) ExecuteWithResult(ctx context.Context, fn func(ctx context.Context) (any, error)) (any, error) {
	m.count++
	return fn(ctx)
}

func TestRoleServiceFiltersAndValidatesMenuKeys(t *testing.T) {
	repo, _, roleID := openRoleServiceTestContext(t)
	service := NewRoleService(repo, nil)

	if err := repo.Default().
		Model(&model.ConsoleRole{}).
		Where("id = ?", roleID).
		Update("menu_keys", datatype.NewStringArray([]string{"ConsoleDashboard", "legacy.invalid", "ConsoleRoles"})).Error; err != nil {
		t.Fatalf("seed dirty menu keys: %v", err)
	}

	menuKeys, err := service.GetRoleMenus(context.Background(), GetRoleMenusInput{RoleID: roleID})
	if err != nil {
		t.Fatalf("get role menus: %v", err)
	}
	if len(menuKeys) != 2 || menuKeys[0] != "ConsoleDashboard" || menuKeys[1] != "ConsoleRoles" {
		t.Fatalf("unexpected filtered menu keys: %#v", menuKeys)
	}

	if err := service.SetRoleMenus(context.Background(), SetRoleMenusInput{
		RoleID:   roleID,
		MenuKeys: []string{"ConsoleDashboard", "bad.key"},
	}); err == nil {
		t.Fatal("expected invalid menu key error")
	}
}

func TestRoleServiceValidatesRuntimeAPIPermissions(t *testing.T) {
	repo, enforcer, roleID := openRoleServiceTestContext(t)

	engine := gin.New()
	engine.GET("/console/v1/dashboard/summary", func(*gin.Context) {})
	engine.GET("/console/v1/roles", func(*gin.Context) {})

	catalog := NewRuntimePermissionCatalog()
	catalog.LoadRoutes(engine.Routes())

	service := NewRoleService(repo, enforcer, catalog)
	if err := service.SetRolePermissions(context.Background(), SetRolePermissionsInput{
		RoleID:         roleID,
		APIPermissions: []string{"GET /console/v1/roles", "POST /console/v1/unknown"},
	}); err == nil {
		t.Fatal("expected invalid api permission error")
	}
}

func TestRoleServiceSetPermissionsUsesInjectedTransaction(t *testing.T) {
	repo, enforcer, roleID := openRoleServiceTestContext(t)

	engine := gin.New()
	engine.GET("/console/v1/roles", func(*gin.Context) {})

	catalog := NewRuntimePermissionCatalog()
	catalog.LoadRoutes(engine.Routes())

	txManager := &countingTxManager{}
	service := NewRoleService(repo, enforcer, catalog).WithTransaction(txManager)
	if err := service.SetRolePermissions(context.Background(), SetRolePermissionsInput{
		RoleID:         roleID,
		APIPermissions: []string{"GET /console/v1/roles"},
	}); err != nil {
		t.Fatalf("set role permissions: %v", err)
	}
	if txManager.count != 1 {
		t.Fatalf("expected transaction manager to execute once, got %d", txManager.count)
	}
}

func openRoleServiceTestContext(t *testing.T) (database.Repo, *rbac.Enforcer, string) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/role-service.db"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.ConsoleRole{}); err != nil {
		t.Fatalf("auto migrate role: %v", err)
	}
	if err := db.Exec(`
CREATE TABLE IF NOT EXISTS console_casbin_rules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    ptype TEXT,
    v0 TEXT,
    v1 TEXT,
    v2 TEXT,
    v3 TEXT,
    v4 TEXT,
    v5 TEXT
);`).Error; err != nil {
		t.Fatalf("create casbin table: %v", err)
	}

	enforcer, err := rbac.New(db, &rbac.Config{TableName: "console_casbin_rules"})
	if err != nil {
		t.Fatalf("new enforcer: %v", err)
	}

	role := model.ConsoleRole{
		Base:        model.Base{ID: "console-role-operator"},
		Name:        "Operator",
		Code:        "operator",
		DisplayName: "Operator",
		MenuKeys:    datatype.NewStringArray([]string{}),
		Status:      model.ConsoleRoleStatusActive,
	}
	if err := db.Create(&role).Error; err != nil {
		t.Fatalf("create role: %v", err)
	}

	return database.NewRepoWithConnections(db, nil), enforcer, role.ID
}
