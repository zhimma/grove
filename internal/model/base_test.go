package model

import (
	"errors"
	"testing"

	"gorm.io/gorm"

	"github.com/zhimma/grove/internal/testkit"
)

func TestBusinessModelsUseSoftDeleteAndAuditLogsUsePhysicalDelete(t *testing.T) {
	db := testkit.OpenDB(t, &ConsoleRole{}, &ConsoleAdmin{}, &ConsoleOperationLog{})

	role := ConsoleRole{
		Base:   Base{ID: "console-role-delete-test"},
		Name:   "Delete Test",
		Code:   "delete-test",
		Status: ConsoleRoleStatusActive,
	}
	if err := db.Create(&role).Error; err != nil {
		t.Fatalf("create role: %v", err)
	}
	admin := ConsoleAdmin{
		Base:     Base{ID: "console-admin-delete-test"},
		Account:  "delete-test",
		Password: "unused",
		RoleID:   role.ID,
		Status:   ConsoleAdminStatusActive,
	}
	if err := db.Create(&admin).Error; err != nil {
		t.Fatalf("create admin: %v", err)
	}
	if err := db.Delete(&admin).Error; err != nil {
		t.Fatalf("delete admin: %v", err)
	}

	var visible ConsoleAdmin
	if err := db.First(&visible, "id = ?", admin.ID).Error; !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("soft-deleted admin should be hidden, got %v", err)
	}
	var deleted ConsoleAdmin
	if err := db.Unscoped().First(&deleted, "id = ?", admin.ID).Error; err != nil {
		t.Fatalf("unscoped query should find soft-deleted admin: %v", err)
	}
	if !deleted.DeletedAt.Valid {
		t.Fatal("soft-deleted admin must record deleted_at")
	}

	log := ConsoleOperationLog{
		AuditBase: AuditBase{ID: "console-operation-log-delete-test"},
		Method:    "DELETE",
		Path:      "/console/v1/test",
		Route:     "/console/v1/test",
		Module:    "test",
		Action:    "delete",
	}
	if err := db.Create(&log).Error; err != nil {
		t.Fatalf("create operation log: %v", err)
	}
	if err := db.Delete(&log).Error; err != nil {
		t.Fatalf("delete operation log: %v", err)
	}
	var count int64
	if err := db.Unscoped().Model(&ConsoleOperationLog{}).Where("id = ?", log.ID).Count(&count).Error; err != nil {
		t.Fatalf("count deleted operation log: %v", err)
	}
	if count != 0 {
		t.Fatalf("audit log delete must be physical, count=%d", count)
	}
}
