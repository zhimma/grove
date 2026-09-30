package migrate

import (
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestLegacyUniquenessRejectsConflictsWithoutDeletingData(t *testing.T) {
	for _, tc := range []struct{ table, columns, values string }{
		{"users", "email", "'reuse@example.test'"},
		{"console_roles", "code", "'operator'"},
		{"console_admins", "account", "'operator'"},
		{"system_configs", "config_group, config_key", "'site', 'title'"},
	} {
		t.Run(tc.table, func(t *testing.T) {
			db, err := gorm.Open(sqlite.Open(t.TempDir()+"/preflight.db"), &gorm.Config{})
			if err != nil {
				t.Fatal(err)
			}
			sqlDB, err := db.DB()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = sqlDB.Close() })
			for _, ddl := range []string{
				"CREATE TABLE users (email TEXT)",
				"CREATE TABLE console_roles (code TEXT)",
				"CREATE TABLE console_admins (account TEXT)",
				"CREATE TABLE system_configs (config_group TEXT, config_key TEXT)",
			} {
				if err := db.Exec(ddl).Error; err != nil {
					t.Fatal(err)
				}
			}
			insert := "INSERT INTO " + tc.table + " (" + tc.columns + ") VALUES (" + tc.values + ")"
			if err := db.Exec(insert).Error; err != nil {
				t.Fatal(err)
			}
			if err := validateLegacyUniqueness(db); err != nil {
				t.Fatal(err)
			}
			if err := db.Exec(insert).Error; err != nil {
				t.Fatal(err)
			}
			if err := validateLegacyUniqueness(db); err == nil || !strings.Contains(err.Error(), tc.table) {
				t.Fatalf("expected conflict on %s: %v", tc.table, err)
			}
			var count int64
			if err := db.Table(tc.table).Count(&count).Error; err != nil || count != 2 {
				t.Fatalf("changed conflicting data: count=%d err=%v", count, err)
			}
		})
	}
}
