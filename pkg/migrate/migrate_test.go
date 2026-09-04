package migrate

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestEveryUpMigrationHasDownMigration(t *testing.T) {
	for _, dialect := range []string{"postgres", "mysql"} {
		dir := filepath.Join("..", "..", "database", "migrations", dialect)
		upFiles, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
		if err != nil {
			t.Fatalf("list %s migrations: %v", dialect, err)
		}
		if len(upFiles) == 0 {
			t.Fatalf("expected %s migrations", dialect)
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
}

func TestResolveDialectDirSupportsNestedAndConcretePaths(t *testing.T) {
	base := filepath.Join("..", "..", "database", "migrations")
	for _, dialect := range []string{"postgres", "mysql"} {
		got, err := ResolveDialectDir(base, dialect)
		if err != nil {
			t.Fatalf("resolve %s migrations: %v", dialect, err)
		}
		if !strings.HasSuffix(filepath.ToSlash(got), "/"+dialect) {
			t.Fatalf("expected %s dialect directory, got %s", dialect, got)
		}
		concrete, err := ResolveDialectDir(got, dialect)
		if err != nil {
			t.Fatalf("resolve concrete %s migrations: %v", dialect, err)
		}
		if concrete != got {
			t.Fatalf("concrete directory changed: got %s want %s", concrete, got)
		}
	}
}

func TestDialectMigrationVersionsMatch(t *testing.T) {
	versions := map[string]map[string]struct{}{}
	for _, dialect := range []string{"postgres", "mysql"} {
		dir := filepath.Join("..", "..", "database", "migrations", dialect)
		files, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
		if err != nil {
			t.Fatalf("list %s migrations: %v", dialect, err)
		}
		versions[dialect] = map[string]struct{}{}
		for _, file := range files {
			name := filepath.Base(file)
			versions[dialect][strings.TrimSuffix(name, ".up.sql")] = struct{}{}
		}
	}
	if len(versions["postgres"]) != len(versions["mysql"]) {
		t.Fatalf("migration version counts differ: postgres=%d mysql=%d", len(versions["postgres"]), len(versions["mysql"]))
	}
	for version := range versions["postgres"] {
		if _, ok := versions["mysql"][version]; !ok {
			t.Fatalf("mysql migration missing %s", version)
		}
	}
}

func TestMySQLMigrationsDoNotContainPostgreSQLOnlySyntax(t *testing.T) {
	dir := filepath.Join("..", "..", "database", "migrations", "mysql")
	files, err := filepath.Glob(filepath.Join(dir, "*.sql"))
	if err != nil {
		t.Fatalf("list mysql migrations: %v", err)
	}
	for _, file := range files {
		content := strings.ToLower(mustReadMigration(t, file))
		for _, forbidden := range []string{"timestamptz", "jsonb", "::jsonb", "on conflict", "using casbin_rules"} {
			if strings.Contains(content, forbidden) {
				t.Errorf("mysql migration %s contains PostgreSQL syntax %q", filepath.Base(file), forbidden)
			}
		}
	}
}

func TestDialectSeedVersionsAndSafetyBoundariesExist(t *testing.T) {
	for _, dialect := range []string{"postgres", "mysql"} {
		for _, kind := range []string{"bootstrap", "demo"} {
			dir := filepath.Join("..", "..", "database", "seeds", dialect, kind)
			files, err := filepath.Glob(filepath.Join(dir, "*.sql"))
			if err != nil {
				t.Fatalf("list %s %s seeds: %v", dialect, kind, err)
			}
			if len(files) == 0 {
				t.Fatalf("expected %s %s seed files", dialect, kind)
			}
		}
	}
}

func TestSystemConfigsMigrationMatchesModelContract(t *testing.T) {
	path := filepath.Join("..", "..", "database", "migrations", "postgres", "202604150005_create_system_configs.up.sql")
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
	path := filepath.Join("..", "..", "database", "migrations", "postgres", "202604150004_expand_console_management.down.sql")
	content := mustReadMigration(t, path)

	if strings.Contains(strings.ToUpper(content), "DROP COLUMN") {
		t.Fatal("004 down must not drop columns already defined by 003")
	}
	if !strings.Contains(content, "ALTER COLUMN email SET NOT NULL") {
		t.Fatal("004 down must restore the 003 email nullability contract")
	}
}

func TestConsoleAdminPasswordStateMigrationMatchesModelContract(t *testing.T) {
	path := filepath.Join("..", "..", "database", "migrations", "postgres", "202604150008_add_console_admin_password_state.up.sql")
	content := mustReadMigration(t, path)
	if !strings.Contains(content, "must_change_password BOOLEAN NOT NULL DEFAULT FALSE") {
		t.Fatal("password state migration must add a non-null false-by-default flag")
	}
}

func TestIntegrityMigrationDefinesDeletionAndConstraintContracts(t *testing.T) {
	path := filepath.Join("..", "..", "database", "migrations", "postgres", "202604150009_define_integrity_semantics.up.sql")
	content := mustReadMigration(t, path)
	for _, fragment := range []string{
		"DROP COLUMN IF EXISTS deleted_at",
		"ADD CONSTRAINT fk_console_admins_role",
		"ON DELETE RESTRICT",
		"ADD CONSTRAINT chk_console_admins_status",
		"ADD CONSTRAINT chk_console_roles_status",
		"ADD CONSTRAINT chk_system_configs_value_type",
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_casbin_rules_unique",
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_console_casbin_rules_unique",
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_console_admins_account_active",
		"WHERE deleted_at IS NULL",
	} {
		if !strings.Contains(content, fragment) {
			t.Errorf("integrity migration missing %q", fragment)
		}
	}
}

func TestConsoleSessionsMigrationDefinesPersistentRefreshContract(t *testing.T) {
	path := filepath.Join("..", "..", "database", "migrations", "postgres", "202604150010_create_console_sessions.up.sql")
	content := mustReadMigration(t, path)
	for _, fragment := range []string{
		"CREATE TABLE IF NOT EXISTS console_sessions",
		"refresh_token_hash CHAR(64) NOT NULL",
		"CONSTRAINT fk_console_sessions_admin",
		"ON DELETE CASCADE",
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_console_sessions_refresh_token_hash",
		"CREATE INDEX IF NOT EXISTS idx_console_sessions_admin_active",
		"WHERE revoked_at IS NULL",
	} {
		if !strings.Contains(content, fragment) {
			t.Errorf("console sessions migration missing %q", fragment)
		}
	}
}

func TestSystemConfigSecretMigrationMatchesModelContract(t *testing.T) {
	upPath := filepath.Join("..", "..", "database", "migrations", "postgres", "202604150011_add_system_config_secrets.up.sql")
	up := mustReadMigration(t, upPath)
	if !strings.Contains(up, "ADD COLUMN IF NOT EXISTS is_secret BOOLEAN NOT NULL DEFAULT FALSE") {
		t.Fatal("system config secret migration must add is_secret")
	}

	downPath := filepath.Join("..", "..", "database", "migrations", "postgres", "202604150011_add_system_config_secrets.down.sql")
	down := mustReadMigration(t, downPath)
	if strings.Contains(down, "RAISE EXCEPTION") {
		t.Fatal("system config secret down migration must not fail after the migration engine marks the version dirty")
	}
	for _, fragment := range []string{"DROP COLUMN IF EXISTS is_secret"} {
		if !strings.Contains(down, fragment) {
			t.Fatalf("system config secret down migration missing %q", fragment)
		}
	}
}

func TestValidateDownMigrationRejectsEncryptedSystemConfigsBeforeExecution(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/migration-guard.db"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec(`CREATE TABLE system_configs (is_secret BOOLEAN NOT NULL DEFAULT FALSE)`).Error; err != nil {
		t.Fatalf("create system_configs: %v", err)
	}

	const migration = "202604150011_add_system_config_secrets"
	if err := validateDownMigration(db, migration); err != nil {
		t.Fatalf("empty system configs must allow down migration: %v", err)
	}
	if err := db.Exec(`INSERT INTO system_configs (is_secret) VALUES (TRUE)`).Error; err != nil {
		t.Fatalf("insert secret config: %v", err)
	}
	if err := validateDownMigration(db, migration); err == nil || !strings.Contains(err.Error(), "cannot remove is_secret") {
		t.Fatalf("encrypted system configs must block down migration, got %v", err)
	}
	if err := db.Exec(`DELETE FROM system_configs`).Error; err != nil {
		t.Fatalf("delete secret config: %v", err)
	}
	if err := validateDownMigration(db, migration); err != nil {
		t.Fatalf("cleared system configs must allow down migration: %v", err)
	}
	if err := validateDownMigration(db, "202604150010_create_console_sessions"); err != nil {
		t.Fatalf("unrelated migration must not be blocked: %v", err)
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

func TestListMigrationsParsesVersionedFiles(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{
		"202604150002_second.up.sql",
		"202604150001_first.up.sql",
		"202604150001_first.down.sql",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("-- migration\n"), 0o600); err != nil {
			t.Fatalf("write migration %s: %v", name, err)
		}
	}

	files, err := listMigrations(dir)
	if err != nil {
		t.Fatalf("list migrations: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected two up migrations, got %d", len(files))
	}
	if files[0].Version != 202604150001 || files[0].Name != "202604150001_first" {
		t.Fatalf("unexpected first migration: %#v", files[0])
	}
	if files[1].Version != 202604150002 || files[1].Name != "202604150002_second" {
		t.Fatalf("unexpected second migration: %#v", files[1])
	}
	if countApplied(files, 202604150001, 202604150002) != 1 {
		t.Fatal("applied migration count must use version boundaries")
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

func TestCreateFilesAllocatesUniqueVersionsOnRepeatedCalls(t *testing.T) {
	dir := t.TempDir()
	up1, down1, err := CreateFiles(dir, "first")
	if err != nil {
		t.Fatalf("create first migration: %v", err)
	}
	up2, down2, err := CreateFiles(dir, "second")
	if err != nil {
		t.Fatalf("create second migration: %v", err)
	}
	if up1 == up2 || down1 == down2 {
		t.Fatalf("repeated migrations must have unique paths: %s %s", up1, up2)
	}
	for _, path := range []string{up1, down1, up2, down2} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected migration file %s: %v", path, err)
		}
	}
}

func TestCreateFilesAllocatesCompletePairsConcurrently(t *testing.T) {
	dir := t.TempDir()
	const count = 8
	type pair struct {
		up   string
		down string
		err  error
	}
	results := make(chan pair, count)
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			up, down, err := CreateFiles(dir, "concurrent_"+strconv.Itoa(i))
			results <- pair{up: up, down: down, err: err}
		}(i)
	}
	wg.Wait()
	close(results)

	seen := make(map[string]struct{}, count*2)
	for result := range results {
		if result.err != nil {
			t.Fatalf("concurrent migration creation failed: %v", result.err)
		}
		for _, path := range []string{result.up, result.down} {
			if _, ok := seen[path]; ok {
				t.Fatalf("migration path was reused: %s", path)
			}
			seen[path] = struct{}{}
			if _, err := os.Stat(path); err != nil {
				t.Fatalf("expected migration file %s: %v", path, err)
			}
		}
	}
	if len(seen) != count*2 {
		t.Fatalf("expected %d migration files, got %d", count*2, len(seen))
	}
	if leftovers, err := filepath.Glob(filepath.Join(dir, ".grove-migration-*")); err != nil || len(leftovers) != 0 {
		t.Fatalf("temporary migration files must be removed: files=%v err=%v", leftovers, err)
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

// The scheduled task table must stay schedule-only. A column that could carry
// a command, script or payload would turn a Console edit into remote code
// execution, which is exactly the design this table avoids.
func TestConsoleScheduledTasksMigrationStoresScheduleNotTaskBody(t *testing.T) {
	for _, dialect := range []string{"postgres", "mysql"} {
		path := filepath.Join("..", "..", "database", "migrations", dialect, "202604150014_create_console_scheduled_tasks.up.sql")
		content := mustReadMigration(t, path)

		for _, fragment := range []string{
			"CREATE TABLE IF NOT EXISTS console_scheduled_tasks",
			"name VARCHAR(120) NOT NULL UNIQUE",
			"schedule VARCHAR(120) NOT NULL",
			"run_requested_at",
			"last_status VARCHAR(20)",
			"CONSTRAINT chk_console_scheduled_tasks_timeout",
		} {
			if !strings.Contains(content, fragment) {
				t.Errorf("%s scheduled tasks migration missing %q", dialect, fragment)
			}
		}

		for _, forbidden := range []string{"command", "script", "shell", "payload", "handler_path", "exec"} {
			if strings.Contains(strings.ToLower(content), forbidden) {
				t.Errorf("%s scheduled tasks migration must not store a task body, found %q", dialect, forbidden)
			}
		}
	}
}
