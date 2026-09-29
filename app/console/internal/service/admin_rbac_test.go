package service

import (
	"context"
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/zhimma/grove/internal/model"
	"github.com/zhimma/grove/pkg/database"
	"github.com/zhimma/grove/pkg/pagination"
	"github.com/zhimma/grove/pkg/rbac"
)

type failingAdminRoleBindings struct {
	calls    [][2]string
	failRole string
}

func (f *failingAdminRoleBindings) ReplaceConsoleRoleForUser(adminID, roleID string) error {
	f.calls = append(f.calls, [2]string{adminID, roleID})
	if roleID == f.failRole {
		return errors.New("injected grouping failure")
	}
	return nil
}

func TestCreateAdminCompensatesDatabaseWhenGroupingFails(t *testing.T) {
	dbs, db, oldRoleID, _ := openAdminRBACTestContext(t)
	bindings := &failingAdminRoleBindings{failRole: oldRoleID}
	service := &AdminService{dbs: dbs, roleBindings: bindings}

	_, err := service.CreateAdmin(context.Background(), CreateAdminInput{
		Account:  "operator",
		Password: "password123",
		RoleID:   oldRoleID,
	})
	if err == nil {
		t.Fatal("expected grouping failure")
	}
	var count int64
	if err := db.Unscoped().Model(&model.ConsoleAdmin{}).Where("account = ?", "operator").Count(&count).Error; err != nil {
		t.Fatalf("count compensated admin: %v", err)
	}
	if count != 0 {
		t.Fatalf("failed create left %d admin rows", count)
	}
}

func TestUpdateAdminRoleCompensatesDatabaseWhenGroupingFails(t *testing.T) {
	dbs, db, oldRoleID, newRoleID := openAdminRBACTestContext(t)
	admin := model.ConsoleAdmin{
		Account:  "operator",
		Username: "operator",
		Password: "hash",
		RoleID:   oldRoleID,
		Status:   model.ConsoleAdminStatusActive,
	}
	if err := db.Create(&admin).Error; err != nil {
		t.Fatalf("create admin: %v", err)
	}
	bindings := &failingAdminRoleBindings{failRole: newRoleID}
	service := &AdminService{dbs: dbs, roleBindings: bindings}

	_, err := service.UpdateAdmin(context.Background(), UpdateAdminInput{AdminID: admin.ID, RoleID: &newRoleID})
	if err == nil {
		t.Fatal("expected grouping failure")
	}
	var persisted model.ConsoleAdmin
	if err := db.First(&persisted, "id = ?", admin.ID).Error; err != nil {
		t.Fatalf("reload admin: %v", err)
	}
	if persisted.RoleID != oldRoleID {
		t.Fatalf("failed role sync left database role %q, expected %q", persisted.RoleID, oldRoleID)
	}
	if len(bindings.calls) != 4 || bindings.calls[0][1] != "" || bindings.calls[1][1] != newRoleID || bindings.calls[2][1] != "" || bindings.calls[3][1] != oldRoleID {
		t.Fatalf("role change must clear, replace, then compensate: %#v", bindings.calls)
	}
}

func TestDeleteAdminDoesNotDeleteDatabaseRowWhenGroupingRemovalFails(t *testing.T) {
	dbs, db, oldRoleID, _ := openAdminRBACTestContext(t)
	admin := model.ConsoleAdmin{
		Account:  "operator",
		Username: "operator",
		Password: "hash",
		RoleID:   oldRoleID,
		Status:   model.ConsoleAdminStatusActive,
	}
	if err := db.Create(&admin).Error; err != nil {
		t.Fatalf("create admin: %v", err)
	}
	bindings := &failingAdminRoleBindings{failRole: ""}
	service := &AdminService{dbs: dbs, roleBindings: bindings}

	if err := service.DeleteAdmin(context.Background(), DeleteAdminInput{AdminID: admin.ID}); err == nil {
		t.Fatal("expected grouping removal failure")
	}
	var count int64
	if err := db.Model(&model.ConsoleAdmin{}).Where("id = ?", admin.ID).Count(&count).Error; err != nil {
		t.Fatalf("count admin: %v", err)
	}
	if count != 1 {
		t.Fatal("admin was deleted before grouping removal succeeded")
	}
}

func TestUpdateAdminRoleSynchronizesDatabaseAndGrouping(t *testing.T) {
	dbs, db, oldRoleID, newRoleID := openAdminRBACTestContext(t)
	enforcer, err := rbac.New(db, &rbac.Config{TableName: "console_casbin_rules"})
	if err != nil {
		t.Fatalf("new enforcer: %v", err)
	}
	admin := model.ConsoleAdmin{
		Account:  "operator",
		Username: "operator",
		Password: "hash",
		RoleID:   oldRoleID,
		Status:   model.ConsoleAdminStatusActive,
	}
	if err := db.Create(&admin).Error; err != nil {
		t.Fatalf("create admin: %v", err)
	}
	if err := enforcer.ReplaceConsoleRoleForUser(admin.ID, oldRoleID); err != nil {
		t.Fatalf("seed old grouping: %v", err)
	}
	service := NewAdminService(dbs, enforcer, pagination.Policy{})

	if _, err := service.UpdateAdmin(context.Background(), UpdateAdminInput{AdminID: admin.ID, RoleID: &newRoleID}); err != nil {
		t.Fatalf("update admin role: %v", err)
	}
	var persisted model.ConsoleAdmin
	if err := db.First(&persisted, "id = ?", admin.ID).Error; err != nil {
		t.Fatalf("reload admin: %v", err)
	}
	if persisted.RoleID != newRoleID {
		t.Fatalf("database role = %q, expected %q", persisted.RoleID, newRoleID)
	}
	groupings, err := enforcer.GetFilteredGroupingPolicy(0, admin.ID)
	if err != nil {
		t.Fatalf("get grouping: %v", err)
	}
	if len(groupings) != 1 || len(groupings[0]) != 2 || groupings[0][1] != newRoleID {
		t.Fatalf("unexpected final grouping: %#v", groupings)
	}
}

func openAdminRBACTestContext(t *testing.T) (database.Connections, *gorm.DB, string, string) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/admin-rbac.db"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.ConsoleRole{}, &model.ConsoleAdmin{}); err != nil {
		t.Fatalf("auto migrate admin RBAC models: %v", err)
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
	oldRole := model.ConsoleRole{Name: "Old", Code: "old", Status: model.ConsoleRoleStatusActive}
	newRole := model.ConsoleRole{Name: "New", Code: "new", Status: model.ConsoleRoleStatusActive}
	if err := db.Create(&oldRole).Error; err != nil {
		t.Fatalf("create old role: %v", err)
	}
	if err := db.Create(&newRole).Error; err != nil {
		t.Fatalf("create new role: %v", err)
	}
	return database.NewConnectionsFromDBs(db, nil), db, oldRole.ID, newRole.ID
}
