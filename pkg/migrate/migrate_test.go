package migrate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestEveryUpMigrationHasDownMigration(t *testing.T) {
	dir := filepath.Join("..", "..", "database", "migrations")
	upFiles, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	if err != nil {
		t.Fatalf("list up migrations: %v", err)
	}
	if len(upFiles) == 0 {
		t.Fatal("expected at least one up migration")
	}

	for _, upFile := range upFiles {
		downFile := strings.TrimSuffix(upFile, ".up.sql") + ".down.sql"
		if _, err := os.Stat(downFile); err != nil {
			if os.IsNotExist(err) {
				t.Errorf("missing down migration for %s", filepath.Base(upFile))
				continue
			}
			t.Errorf("stat down migration %s: %v", filepath.Base(downFile), err)
		}
	}
}

func TestSystemConfigsMigrationMatchesModelContract(t *testing.T) {
	path := filepath.Join("..", "..", "database", "migrations", "202604150005_create_system_configs.up.sql")
	content := mustReadMigration(t, path)

	for _, fragment := range []string{
		"CREATE TABLE IF NOT EXISTS system_configs",
		"id VARCHAR(26) PRIMARY KEY",
		"config_group VARCHAR(64) NOT NULL",
		"config_key VARCHAR(120) NOT NULL",
		"value_type VARCHAR(20) NOT NULL DEFAULT 'string'",
		"value TEXT NOT NULL DEFAULT ''",
		"default_value TEXT NOT NULL DEFAULT ''",
		"is_editable BOOLEAN NOT NULL DEFAULT TRUE",
		"is_system BOOLEAN NOT NULL DEFAULT FALSE",
		"sort_order INTEGER NOT NULL DEFAULT 0",
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_system_configs_group_key",
		"created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()",
		"updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()",
		"deleted_at TIMESTAMPTZ NULL",
	} {
		if !strings.Contains(content, fragment) {
			t.Errorf("system config migration missing %q", fragment)
		}
	}
}

func TestConsoleManagementDownPreservesBaseColumns(t *testing.T) {
	path := filepath.Join("..", "..", "database", "migrations", "202604150004_expand_console_management.down.sql")
	content := mustReadMigration(t, path)

	if strings.Contains(strings.ToUpper(content), "DROP COLUMN") {
		t.Fatal("004 down must not drop columns already defined by 003")
	}
	if !strings.Contains(content, "ALTER COLUMN email SET NOT NULL") {
		t.Fatal("004 down must restore the 003 email nullability contract")
	}
}

func TestConsoleAdminPasswordStateMigrationMatchesModelContract(t *testing.T) {
	path := filepath.Join("..", "..", "database", "migrations", "202604150008_add_console_admin_password_state.up.sql")
	content := mustReadMigration(t, path)
	if !strings.Contains(content, "must_change_password BOOLEAN NOT NULL DEFAULT FALSE") {
		t.Fatal("password state migration must add a non-null false-by-default flag")
	}
}

func TestRunSQLDirWithReplacements(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/seed.db"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec(`CREATE TABLE seed_values (value TEXT NOT NULL)`).Error; err != nil {
		t.Fatalf("create seed table: %v", err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "001_seed.sql")
	if err := os.WriteFile(path, []byte(`INSERT INTO seed_values (value) VALUES ('{{VALUE}}');`), 0o600); err != nil {
		t.Fatalf("write seed: %v", err)
	}

	count, err := RunSQLDirWithReplacements(db, dir, map[string]string{"{{VALUE}}": "replaced"})
	if err != nil {
		t.Fatalf("run seed with replacements: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one seed file, got %d", count)
	}

	var value string
	if err := db.Raw(`SELECT value FROM seed_values LIMIT 1`).Scan(&value).Error; err != nil {
		t.Fatalf("read seed value: %v", err)
	}
	if value != "replaced" {
		t.Fatalf("unexpected replaced value: %q", value)
	}
}

func TestCreateFilesSanitizesUnsafeMigrationName(t *testing.T) {
	dir := t.TempDir()

	upPath, downPath, err := CreateFiles(dir, "../Create Demo-Table!")
	if err != nil {
		t.Fatalf("CreateFiles returned error: %v", err)
	}

	for _, path := range []string{upPath, downPath} {
		if filepath.Dir(path) != dir {
			t.Fatalf("migration path escaped target dir: %s", path)
		}
		base := filepath.Base(path)
		if strings.Contains(base, "..") || strings.Contains(base, "-") || strings.Contains(base, "!") {
			t.Fatalf("migration filename was not sanitized: %s", base)
		}
	}

	if !strings.Contains(filepath.Base(upPath), "_create_demo_table.up.sql") {
		t.Fatalf("unexpected up migration name: %s", upPath)
	}
	if !strings.Contains(filepath.Base(downPath), "_create_demo_table.down.sql") {
		t.Fatalf("unexpected down migration name: %s", downPath)
	}
}

func mustReadMigration(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatalf("read migration %s: %v", filepath.Base(path), err)
	}
	return string(content)
}
