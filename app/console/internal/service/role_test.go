package service

import (
	"context"
	"errors"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/zhimma/grove/internal/datatype"
	"github.com/zhimma/grove/internal/model"
	"github.com/zhimma/grove/pkg/database"
	"github.com/zhimma/grove/pkg/rbac"
)

type recordingRolePolicies struct {
	replacements [][]string
	current      []string
	replaceErr   error
}

func (r *recordingRolePolicies) GetConsolePoliciesForRole(roleID string) ([][]string, error) {
	rules := make([][]string, 0, len(r.current))
	for _, permission := range r.current {
		rules = append(rules, []string{roleID, permission})
	}
	return rules, nil
}

func (r *recordingRolePolicies) ReplaceConsolePoliciesForRole(_ string, permissions []string) error {
	r.replacements = append(r.replacements, append([]string(nil), permissions...))
	if r.replaceErr != nil {
		return r.replaceErr
	}
	r.current = append([]string(nil), permissions...)
	return nil
}

func TestRoleServicePreservesHistoricalAndAcceptsNewMenuKeys(t *testing.T) {
	dbs, _, roleID := openRoleServiceTestContext(t)
	service := NewRoleService(dbs, nil)

	if err := dbs.Default().
		Model(&model.ConsoleRole{}).
		Where("id = ?", roleID).
		Update("menu_keys", datatype.NewStringArray([]string{"ConsoleDashboard", "legacy.invalid", "ConsoleFuture"})).Error; err != nil {
		t.Fatalf("seed dirty menu keys: %v", err)
	}

	menuKeys, err := service.GetRoleMenus(context.Background(), GetRoleMenusInput{RoleID: roleID})
	if err != nil {
		t.Fatalf("get role menus: %v", err)
	}
	if len(menuKeys) != 3 || menuKeys[0] != "ConsoleDashboard" || menuKeys[1] != "legacy.invalid" || menuKeys[2] != "ConsoleFuture" {
		t.Fatalf("unexpected preserved menu keys: %#v", menuKeys)
	}

	if err := service.SetRoleMenus(context.Background(), SetRoleMenusInput{
		RoleID:   roleID,
		MenuKeys: []string{"ConsoleFuture", "ConsoleFuture", "reports.monthly"},
	}); err != nil {
		t.Fatalf("set new frontend menu keys: %v", err)
	}
	menuKeys, err = service.GetRoleMenus(context.Background(), GetRoleMenusInput{RoleID: roleID})
	if err != nil {
		t.Fatalf("get updated role menus: %v", err)
	}
	if len(menuKeys) != 2 || menuKeys[0] != "ConsoleFuture" || menuKeys[1] != "reports.monthly" {
		t.Fatalf("unexpected updated menu keys: %#v", menuKeys)
	}

	if err := service.SetRoleMenus(context.Background(), SetRoleMenusInput{
		RoleID:   roleID,
		MenuKeys: []string{"bad key"},
	}); err == nil {
		t.Fatal("expected invalid menu key format error")
	}
}

func TestRoleServiceValidatesRuntimeAPIPermissions(t *testing.T) {
	dbs, enforcer, roleID := openRoleServiceTestContext(t)

	engine := gin.New()
	engine.GET("/console/v1/dashboard/summary", func(*gin.Context) {})
	engine.GET("/console/v1/roles", func(*gin.Context) {})

	catalog := NewRuntimePermissionCatalog()
	catalog.LoadRoutes(engine.Routes())

	service := NewRoleService(dbs, enforcer, catalog)
	if err := service.SetRolePermissions(context.Background(), SetRolePermissionsInput{
		RoleID:         roleID,
		APIPermissions: []string{"GET /console/v1/roles", "POST /console/v1/unknown"},
	}); err == nil {
		t.Fatal("expected invalid api permission error")
	}
}

func TestRoleServiceSetPermissionsUsesSingleAtomicReplacement(t *testing.T) {
	dbs, _, roleID := openRoleServiceTestContext(t)

	engine := gin.New()
	engine.GET("/console/v1/roles", func(*gin.Context) {})

	catalog := NewRuntimePermissionCatalog()
	catalog.LoadRoutes(engine.Routes())

	policies := &recordingRolePolicies{}
	service := &RoleService{dbs: dbs, rolePolicies: policies, runtimePermission: catalog}
	if err := service.SetRolePermissions(context.Background(), SetRolePermissionsInput{
		RoleID:         roleID,
		APIPermissions: []string{"GET /console/v1/roles"},
	}); err != nil {
		t.Fatalf("set role permissions: %v", err)
	}
	if len(policies.replacements) != 1 || len(policies.replacements[0]) != 1 || policies.replacements[0][0] != "GET /console/v1/roles" {
		t.Fatalf("unexpected policy replacements: %#v", policies.replacements)
	}
}

func TestRoleServiceSetPermissionsReturnsReplacementFailure(t *testing.T) {
	dbs, _, roleID := openRoleServiceTestContext(t)
	engine := gin.New()
	engine.GET("/console/v1/roles", func(*gin.Context) {})
	catalog := NewRuntimePermissionCatalog()
	catalog.LoadRoutes(engine.Routes())
	policies := &recordingRolePolicies{replaceErr: errors.New("injected policy failure")}
	service := &RoleService{dbs: dbs, rolePolicies: policies, runtimePermission: catalog}
	if err := service.SetRolePermissions(context.Background(), SetRolePermissionsInput{
		RoleID:         roleID,
		APIPermissions: []string{"GET /console/v1/roles"},
	}); err == nil {
		t.Fatal("expected policy replacement failure")
	}
}

func TestDeleteRoleRestoresPoliciesWhenDatabaseDeleteFails(t *testing.T) {
	dbs, _, roleID := openRoleServiceTestContext(t)
	if err := dbs.Default().Exec(`
CREATE TRIGGER fail_role_soft_delete
BEFORE UPDATE OF deleted_at ON console_roles
WHEN OLD.id = '` + roleID + `'
BEGIN
    SELECT RAISE(ABORT, 'injected role delete failure');
END;`).Error; err != nil {
		t.Fatalf("create role delete trigger: %v", err)
	}
	policies := &recordingRolePolicies{current: []string{"GET /console/v1/roles"}}
	service := &RoleService{dbs: dbs, rolePolicies: policies}

	if err := service.DeleteRole(context.Background(), DeleteRoleInput{RoleID: roleID}); err == nil {
		t.Fatal("expected role delete failure")
	}
	if len(policies.replacements) != 2 || len(policies.replacements[0]) != 0 || len(policies.replacements[1]) != 1 {
		t.Fatalf("expected clear then restore, got %#v", policies.replacements)
	}
	if len(policies.current) != 1 || policies.current[0] != "GET /console/v1/roles" {
		t.Fatalf("old policies were not restored: %#v", policies.current)
	}
}

func openRoleServiceTestContext(t *testing.T) (database.Connections, *rbac.Enforcer, string) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/role-service.db"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.ConsoleRole{}, &model.ConsoleAdmin{}); err != nil {
		t.Fatalf("auto migrate role models: %v", err)
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

	return database.NewConnectionsFromDBs(db, nil), enforcer, role.ID
}
