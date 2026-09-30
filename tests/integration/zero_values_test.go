//go:build integration

package integration_test

import (
	"database/sql"
	"errors"
	"testing"

	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/zhimma/grove/internal/model"
)

// 在真实迁移后的表上验证显式零值，不依赖 SQLite 自动建表的默认值。
func checkExplicitZeroValues(t *testing.T, db *sql.DB, driver string) {
	t.Helper()
	var dialect gorm.Dialector = postgres.New(postgres.Config{Conn: db})
	if driver == "mysql" {
		dialect = mysql.New(mysql.Config{Conn: db})
	}
	orm, err := gorm.Open(dialect, &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	rollback := errors.New("rollback zero-value fixtures")
	err = orm.Transaction(func(tx *gorm.DB) error {
		role := model.ConsoleRole{Name: "零值回归", Code: "zero-value-role", Status: model.ConsoleRoleStatusDisabled}
		if err := tx.Create(&role).Error; err != nil {
			return err
		}
		admin := model.ConsoleAdmin{Account: "zero-value-admin", Password: "test-only", RoleID: role.ID, Status: model.ConsoleAdminStatusDisabled}
		if err := tx.Create(&admin).Error; err != nil {
			return err
		}
		config := model.SystemConfig{ConfigGroup: "test", ConfigKey: "zero-value", Name: "只读配置", ValueType: "string", IsEditable: false}
		if err := tx.Create(&config).Error; err != nil {
			return err
		}
		audit := model.ConsoleOperationLog{AdminID: admin.ID, Method: "DELETE", Path: "/test", StatusCode: 403, Success: false}
		if err := tx.Create(&audit).Error; err != nil {
			return err
		}
		for _, check := range []struct {
			record     any
			id         string
			expression string
		}{
			{&role, role.ID, "status"},
			{&admin, admin.ID, "status"},
			{&config, config.ID, "CASE WHEN is_editable THEN 1 ELSE 0 END"},
			{&audit, audit.ID, "CASE WHEN success THEN 1 ELSE 0 END"},
		} {
			var actual int
			if err := tx.Model(check.record).Select(check.expression).Where("id = ?", check.id).Scan(&actual).Error; err != nil {
				return err
			}
			if actual != 0 {
				t.Errorf("%T explicit zero replaced with %d", check.record, actual)
			}
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatalf("zero-value fixtures: %v", err)
	}
}
