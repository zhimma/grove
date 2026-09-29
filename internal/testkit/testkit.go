// Package testkit holds the fixtures tests across the repository share.
package testkit

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// OpenDB opens a SQLite database private to t, migrates models into it and
// closes it when t ends.
func OpenDB(t testing.TB, models ...any) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "test.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("sqlite handle: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if len(models) > 0 {
		if err := db.AutoMigrate(models...); err != nil {
			t.Fatalf("auto migrate: %v", err)
		}
	}
	return db
}

// CreateCasbinTable creates a Casbin rule table shaped like the migrated one.
// The adapter's auto-migration is off, so nothing else creates it in tests.
func CreateCasbinTable(t testing.TB, db *gorm.DB, table string) {
	t.Helper()
	ddl := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %[1]s (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    ptype TEXT NOT NULL DEFAULT '',
    v0 TEXT NOT NULL DEFAULT '',
    v1 TEXT NOT NULL DEFAULT '',
    v2 TEXT NOT NULL DEFAULT '',
    v3 TEXT NOT NULL DEFAULT '',
    v4 TEXT NOT NULL DEFAULT '',
    v5 TEXT NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_%[1]s_unique ON %[1]s (ptype, v0, v1, v2, v3, v4, v5);`, table)
	if err := db.Exec(ddl).Error; err != nil {
		t.Fatalf("create %s: %v", table, err)
	}
}
