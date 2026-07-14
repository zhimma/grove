package main

import (
	"bytes"
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/zhimma/grove/internal/model"
	"github.com/zhimma/grove/pkg/rbac"
)

func TestRBACCommandExposesCheckAndDryRunRepair(t *testing.T) {
	cmd := newRBACCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("rbac help: %v", err)
	}
	assertContains(t, out.String(), "check")
	assertContains(t, out.String(), "repair")

	out.Reset()
	repair := newRBACRepairCmd()
	repair.SetOut(&out)
	repair.SetErr(&out)
	repair.SetArgs([]string{"--help"})
	if err := repair.Execute(); err != nil {
		t.Fatalf("rbac repair help: %v", err)
	}
	assertContains(t, out.String(), "--dry-run")
}

func TestInspectAndRepairConsoleRBAC(t *testing.T) {
	db, enforcer, adminID, roleID := openRBACCommandTestContext(t)
	if err := enforcer.ReplaceConsoleRoleForUser(adminID, "console-role-wrong"); err != nil {
		t.Fatalf("seed wrong grouping: %v", err)
	}
	if err := enforcer.ReplaceConsoleRoleForUser("console-admin-orphan", roleID); err != nil {
		t.Fatalf("seed orphan grouping: %v", err)
	}
	if err := enforcer.ReplaceConsolePoliciesForRole("console-role-orphan", []string{"GET /console/v1/orphan"}); err != nil {
		t.Fatalf("seed orphan policy: %v", err)
	}

	differences, err := inspectConsoleRBAC(context.Background(), db, enforcer)
	if err != nil {
		t.Fatalf("inspect RBAC: %v", err)
	}
	for _, kind := range []string{"admin_grouping_mismatch", "orphan_grouping", "orphan_role_policy"} {
		if !containsRBACDifference(differences, kind) {
			t.Fatalf("missing %s difference: %#v", kind, differences)
		}
	}

	if err := repairConsoleRBAC(context.Background(), db, enforcer); err != nil {
		t.Fatalf("repair RBAC: %v", err)
	}
	differences, err = inspectConsoleRBAC(context.Background(), db, enforcer)
	if err != nil {
		t.Fatalf("inspect repaired RBAC: %v", err)
	}
	if len(differences) != 0 {
		t.Fatalf("RBAC differences remain after repair: %#v", differences)
	}
}

func openRBACCommandTestContext(t *testing.T) (*gorm.DB, *rbac.Enforcer, string, string) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/rbac-command.db"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.ConsoleRole{}, &model.ConsoleAdmin{}); err != nil {
		t.Fatalf("auto migrate RBAC models: %v", err)
	}
	if err := db.Exec(`
CREATE TABLE console_casbin_rules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    ptype TEXT NOT NULL DEFAULT '',
    v0 TEXT NOT NULL DEFAULT '',
    v1 TEXT NOT NULL DEFAULT '',
    v2 TEXT NOT NULL DEFAULT '',
    v3 TEXT NOT NULL DEFAULT '',
    v4 TEXT NOT NULL DEFAULT '',
    v5 TEXT NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX idx_console_casbin_rules_unique
ON console_casbin_rules (ptype, v0, v1, v2, v3, v4, v5);`).Error; err != nil {
		t.Fatalf("create casbin table: %v", err)
	}
	enforcer, err := rbac.New(db, &rbac.Config{TableName: "console_casbin_rules"})
	if err != nil {
		t.Fatalf("new enforcer: %v", err)
	}
	role := model.ConsoleRole{Name: "Operator", Code: "operator", Status: model.ConsoleRoleStatusActive}
	if err := db.Create(&role).Error; err != nil {
		t.Fatalf("create role: %v", err)
	}
	admin := model.ConsoleAdmin{
		Account:  "operator",
		Username: "operator",
		Password: "hash",
		RoleID:   role.ID,
		Status:   model.ConsoleAdminStatusActive,
	}
	if err := db.Create(&admin).Error; err != nil {
		t.Fatalf("create admin: %v", err)
	}
	return db, enforcer, admin.ID, role.ID
}

func containsRBACDifference(differences []rbacDifference, kind string) bool {
	for _, difference := range differences {
		if difference.Kind == kind {
			return true
		}
	}
	return false
}
