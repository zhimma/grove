//go:build integration

package integration_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"golang.org/x/crypto/bcrypt"
)

func TestMySQLFreshDatabaseMigration(t *testing.T) {
	if os.Getenv("GROVE_INTEGRATION_DB") != "mysql" {
		t.Skip("set GROVE_INTEGRATION_DB=mysql to run MySQL integration")
	}
	testcontainers.SkipIfProviderIsNotHealthy(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "mysql:8.0",
			ExposedPorts: []string{"3306/tcp"},
			Env: map[string]string{
				"MYSQL_DATABASE":      "grove_test",
				"MYSQL_USER":          "grove",
				"MYSQL_PASSWORD":      "grove_test_password",
				"MYSQL_ROOT_PASSWORD": "grove_root_password",
			},
			WaitingFor: wait.ForLog("ready for connections").WithStartupTimeout(4 * time.Minute),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("start MySQL container: %v", err)
	}
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("MySQL container host: %v", err)
	}
	port, err := container.MappedPort(ctx, "3306/tcp")
	if err != nil {
		t.Fatalf("MySQL container port: %v", err)
	}
	dsn := fmt.Sprintf("grove:grove_test_password@tcp(%s:%s)/grove_test?charset=utf8mb4&parseTime=true&multiStatements=true", host, port.Port())
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open MySQL: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping MySQL: %v", err)
	}

	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	configPath := writeMySQLCLIConfig(t, host, port.Port())
	commandEnv := []string{
		"APP_ENV=development",
		"JWT_SECRET=0123456789abcdef0123456789abcdef",
		"GROVE_ROOT_PASSWORD=initial-root-password",
		"DB_ENABLED=",
		"DB_DRIVER=",
		"DB_HOST=",
		"DB_PORT=",
		"DB_USER=",
		"DB_PASSWORD=",
		"DB_NAME=",
		"DB_CHARSET=",
		"DB_PARSE_TIME=",
		"DB_LOC=",
		"DB_TLS=",
	}

	if output := runGrove(t, ctx, repoRoot, configPath, commandEnv, "migrate", "up"); !strings.Contains(output, "已执行") {
		t.Fatalf("unexpected MySQL migrate output: %s", output)
	}
	for _, table := range []string{
		"users",
		"console_roles",
		"console_admins",
		"system_configs",
		"console_operation_logs",
		"console_login_logs",
		"console_sessions",
		"console_scheduled_tasks",
		"grove_migrations",
	} {
		assertMySQLTableExists(t, db, table, true)
	}
	assertMySQLColumnExists(t, db, "console_operation_logs", "deleted_at", false)
	assertMySQLColumnExists(t, db, "console_login_logs", "deleted_at", false)
	for _, constraint := range []string{
		"fk_console_admins_role",
		"chk_console_admins_status",
		"chk_console_roles_status",
		"chk_system_configs_value_type",
		"fk_console_sessions_admin",
		"chk_console_sessions_expiry",
		"chk_console_scheduled_tasks_timeout",
	} {
		assertMySQLConstraintExists(t, db, constraint)
	}

	if output := runGrove(t, ctx, repoRoot, configPath, commandEnv, "migrate", "status"); !strings.Contains(output, "202604150011_add_system_config_secrets") {
		t.Fatalf("unexpected MySQL migration status: %s", output)
	}
	if _, err := db.ExecContext(ctx, `UPDATE grove_migrations SET dirty = TRUE`); err != nil {
		t.Fatalf("mark MySQL migration dirty: %v", err)
	}
	dirtyOutput, dirtyErr := runGroveWithError(ctx, repoRoot, configPath, commandEnv, "migrate", "status")
	if dirtyErr == nil || !strings.Contains(strings.ToLower(dirtyOutput), "dirty") {
		t.Fatalf("expected MySQL dirty migration error, err=%v output=%s", dirtyErr, dirtyOutput)
	}
	if _, err := db.ExecContext(ctx, `UPDATE grove_migrations SET dirty = FALSE`); err != nil {
		t.Fatalf("clear MySQL migration dirty state: %v", err)
	}

	if output := runGrove(t, ctx, repoRoot, configPath, commandEnv, "seed", "bootstrap"); !strings.Contains(output, "已执行") {
		t.Fatalf("unexpected MySQL bootstrap output: %s", output)
	}
	var originalHash string
	if err := db.QueryRowContext(ctx, `SELECT password FROM console_admins WHERE id = 'console-admin-root'`).Scan(&originalHash); err != nil {
		t.Fatalf("read MySQL root password: %v", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(originalHash), []byte("initial-root-password")); err != nil {
		t.Fatalf("MySQL bootstrap password does not match configured value: %v", err)
	}
	changedHash, err := bcrypt.GenerateFromPassword([]byte("changed-root-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash changed MySQL root password: %v", err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE console_admins SET password = ? WHERE id = 'console-admin-root'`, string(changedHash)); err != nil {
		t.Fatalf("change MySQL root password: %v", err)
	}
	runGrove(t, ctx, repoRoot, configPath, commandEnv, "seed", "bootstrap")
	var passwordAfterSecondBootstrap string
	if err := db.QueryRowContext(ctx, `SELECT password FROM console_admins WHERE id = 'console-admin-root'`).Scan(&passwordAfterSecondBootstrap); err != nil {
		t.Fatalf("read MySQL root password after second bootstrap: %v", err)
	}
	if passwordAfterSecondBootstrap != string(changedHash) {
		t.Fatal("repeated MySQL bootstrap overwrote the existing root password")
	}

	if output := runGrove(t, ctx, repoRoot, configPath, commandEnv, "seed", "demo"); !strings.Contains(output, "已执行") {
		t.Fatalf("unexpected MySQL demo seed output: %s", output)
	}

	assertMySQLExecFails(t, db, `INSERT INTO console_roles (id, name, code, menu_keys, status) VALUES ('invalid-role', 'Invalid', 'invalid', JSON_ARRAY(), 9)`)
	assertMySQLExecFails(t, db, `INSERT INTO console_admins (id, account, password, role_id, status) VALUES ('invalid-admin', 'invalid-admin', 'unused', 'missing-role', 1)`)
	assertMySQLExecFails(t, db, `INSERT INTO system_configs (id, config_group, config_key, name, value_type, value, default_value) VALUES ('invalid-config', 'test', 'invalid', 'Invalid', 'yaml', '', '')`)
	if _, err := db.ExecContext(ctx, `INSERT INTO casbin_rules (ptype, v0, v1) VALUES ('p', 'integration-role', 'GET /integration')`); err != nil {
		t.Fatalf("insert MySQL casbin rule: %v", err)
	}
	assertMySQLExecFails(t, db, `INSERT INTO casbin_rules (ptype, v0, v1) VALUES ('p', 'integration-role', 'GET /integration')`)
	for _, statement := range []string{
		`INSERT INTO users (id, name, email) VALUES ('soft-delete-user-1', 'First', 'reuse@example.test')`,
		`UPDATE users SET deleted_at = CURRENT_TIMESTAMP(6) WHERE id = 'soft-delete-user-1'`,
		`INSERT INTO users (id, name, email) VALUES ('soft-delete-user-2', 'Second', 'reuse@example.test')`,
	} {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			t.Fatalf("verify MySQL active-record uniqueness with %q: %v", statement, err)
		}
	}

	for attempts := 0; attempts < 32; attempts++ {
		output := runGrove(t, ctx, repoRoot, configPath, commandEnv, "migrate", "down")
		if strings.Contains(output, "没有可回滚的迁移") {
			break
		}
		if attempts == 31 {
			t.Fatal("MySQL migrations did not reach the empty state")
		}
	}
	assertMySQLTableExists(t, db, "users", false)
}

func writeMySQLCLIConfig(t *testing.T, host, port string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := fmt.Sprintf(`app:
  env: production
jwt:
  secret: 0123456789abcdef0123456789abcdef
databases:
  default:
    enabled: true
    driver: mysql
    host: %s
    port: %s
    user: grove
    password: grove_test_password
    dbname: grove_test
    charset: utf8mb4
    parse_time: true
    loc: Local
    tls: false
`, host, port)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write MySQL CLI config: %v", err)
	}
	return path
}

func assertMySQLTableExists(t *testing.T, db *sql.DB, table string, expected bool) {
	t.Helper()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?`, table).Scan(&count); err != nil {
		t.Fatalf("check MySQL table %s: %v", table, err)
	}
	if (count > 0) != expected {
		t.Fatalf("MySQL table %s existence = %t, expected %t", table, count > 0, expected)
	}
}

func assertMySQLColumnExists(t *testing.T, db *sql.DB, table, column string, expected bool) {
	t.Helper()
	var count int
	if err := db.QueryRow(`
SELECT COUNT(*)
FROM information_schema.columns
WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?`, table, column).Scan(&count); err != nil {
		t.Fatalf("check MySQL column %s.%s: %v", table, column, err)
	}
	if (count > 0) != expected {
		t.Fatalf("MySQL column %s.%s existence = %t, expected %t", table, column, count > 0, expected)
	}
}

func assertMySQLConstraintExists(t *testing.T, db *sql.DB, name string) {
	t.Helper()
	var count int
	if err := db.QueryRow(`
SELECT COUNT(*)
FROM information_schema.table_constraints
WHERE table_schema = DATABASE() AND constraint_name = ?`, name).Scan(&count); err != nil {
		t.Fatalf("check MySQL constraint %s: %v", name, err)
	}
	if count == 0 {
		t.Fatalf("MySQL constraint %s does not exist", name)
	}
}

func assertMySQLExecFails(t *testing.T, db *sql.DB, query string) {
	t.Helper()
	if _, err := db.Exec(query); err == nil {
		t.Fatalf("expected MySQL constraint to reject query: %s", query)
	}
}
