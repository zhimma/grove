//go:build integration

package integration_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"golang.org/x/crypto/bcrypt"
)

func TestFreshDatabaseLifecycle(t *testing.T) {
	if os.Getenv("CI") == "" {
		testcontainers.SkipIfProviderIsNotHealthy(t)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	container, err := postgres.Run(
		ctx,
		"postgres:17-alpine",
		postgres.WithDatabase("grove_test"),
		postgres.WithUsername("grove"),
		postgres.WithPassword("grove_test_password"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("start PostgreSQL container: %v", err)
	}
	t.Cleanup(func() {
		_ = container.Terminate(context.Background())
	})

	connectionString, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("PostgreSQL connection string: %v", err)
	}
	db, err := sql.Open("pgx", connectionString)
	if err != nil {
		t.Fatalf("open PostgreSQL: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping PostgreSQL: %v", err)
	}

	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	configPath := writeCLIConfig(t, container)
	commandEnv := []string{
		"APP_ENV=production",
		"JWT_SECRET=0123456789abcdef0123456789abcdef",
		"GROVE_ROOT_PASSWORD=initial-root-password",
		"DB_ENABLED=",
		"DB_HOST=",
		"DB_PORT=",
		"DB_USER=",
		"DB_PASSWORD=",
		"DB_NAME=",
		"DB_SSLMODE=",
	}

	output := runGrove(t, ctx, repoRoot, configPath, commandEnv, "migrate", "up")
	if !strings.Contains(output, "已执行") {
		t.Fatalf("unexpected migrate up output: %s", output)
	}

	for _, relation := range []string{
		"users",
		"console_roles",
		"console_admins",
		"system_configs",
		"console_operation_logs",
		"console_login_logs",
		"console_sessions",
		"idx_system_configs_group_key",
		"idx_users_email_active",
		"idx_console_roles_code_active",
		"idx_console_admins_account_active",
		"idx_console_admins_email_active",
		"idx_console_admins_phone_active",
		"idx_casbin_rules_unique",
		"idx_console_casbin_rules_unique",
		"idx_console_sessions_refresh_token_hash",
		"idx_console_sessions_admin_active",
		"grove_migrations",
	} {
		assertRelationExists(t, db, relation, true)
	}
	assertRelationExists(t, db, "schema_migrations", false)
	assertColumnExists(t, db, "console_operation_logs", "deleted_at", false)
	assertColumnExists(t, db, "console_login_logs", "deleted_at", false)
	for _, constraint := range []string{
		"fk_console_admins_role",
		"chk_console_admins_status",
		"chk_console_roles_status",
		"chk_system_configs_value_type",
		"fk_console_sessions_admin",
		"chk_console_sessions_expiry",
	} {
		assertConstraintExists(t, db, constraint)
	}
	assertExecFails(t, db, `INSERT INTO console_roles (id, name, code, status) VALUES ('invalid-role', 'Invalid', 'invalid', 9)`)
	assertExecFails(t, db, `INSERT INTO console_admins (id, account, password, role_id, status) VALUES ('invalid-admin', 'invalid-admin', 'unused', 'missing-role', 1)`)
	assertExecFails(t, db, `INSERT INTO system_configs (id, config_group, config_key, name, value_type) VALUES ('invalid-config', 'test', 'invalid', 'Invalid', 'yaml')`)
	if _, err := db.Exec(`INSERT INTO casbin_rules (ptype, v0, v1) VALUES ('p', 'integration-role', 'GET /integration')`); err != nil {
		t.Fatalf("insert casbin rule: %v", err)
	}
	assertExecFails(t, db, `INSERT INTO casbin_rules (ptype, v0, v1) VALUES ('p', 'integration-role', 'GET /integration')`)
	if _, err := db.Exec(`INSERT INTO console_casbin_rules (ptype, v0, v1) VALUES ('p', 'integration-role', 'GET /integration')`); err != nil {
		t.Fatalf("insert console casbin rule: %v", err)
	}
	assertExecFails(t, db, `INSERT INTO console_casbin_rules (ptype, v0, v1) VALUES ('p', 'integration-role', 'GET /integration')`)

	for _, statement := range []string{
		`INSERT INTO users (id, name, email) VALUES ('soft-delete-user-1', 'First', 'reuse@example.test')`,
		`UPDATE users SET deleted_at = NOW() WHERE id = 'soft-delete-user-1'`,
		`INSERT INTO users (id, name, email) VALUES ('soft-delete-user-2', 'Second', 'reuse@example.test')`,
		`DELETE FROM users WHERE email = 'reuse@example.test'`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("verify active-record uniqueness with %q: %v", statement, err)
		}
	}

	status := runGrove(t, ctx, repoRoot, configPath, commandEnv, "migrate", "status")
	if !strings.Contains(status, "已执行") || !strings.Contains(status, "202604150010_create_console_sessions") {
		t.Fatalf("unexpected migration status: %s", status)
	}

	if _, err := db.ExecContext(ctx, `UPDATE grove_migrations SET dirty = TRUE`); err != nil {
		t.Fatalf("mark migration dirty: %v", err)
	}
	dirtyOutput, dirtyErr := runGroveWithError(ctx, repoRoot, configPath, commandEnv, "migrate", "status")
	if dirtyErr == nil || !strings.Contains(strings.ToLower(dirtyOutput), "dirty") {
		t.Fatalf("expected dirty migration error, err=%v output=%s", dirtyErr, dirtyOutput)
	}
	if _, err := db.ExecContext(ctx, `UPDATE grove_migrations SET dirty = FALSE`); err != nil {
		t.Fatalf("clear migration dirty state: %v", err)
	}

	runGrove(t, ctx, repoRoot, configPath, commandEnv, "seed", "bootstrap")
	assertExecFails(t, db, `DELETE FROM console_roles WHERE id = 'console-role-root'`)
	var originalHash string
	if err := db.QueryRowContext(ctx, `SELECT password FROM console_admins WHERE id = 'console-admin-root'`).Scan(&originalHash); err != nil {
		t.Fatalf("read root password: %v", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(originalHash), []byte("initial-root-password")); err != nil {
		t.Fatalf("bootstrap password does not match configured value: %v", err)
	}

	changedHash, err := bcrypt.GenerateFromPassword([]byte("changed-root-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash changed root password: %v", err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE console_admins SET password = $1 WHERE id = 'console-admin-root'`, string(changedHash)); err != nil {
		t.Fatalf("change root password: %v", err)
	}
	runGrove(t, ctx, repoRoot, configPath, commandEnv, "seed", "bootstrap")

	var passwordAfterSecondBootstrap string
	if err := db.QueryRowContext(ctx, `SELECT password FROM console_admins WHERE id = 'console-admin-root'`).Scan(&passwordAfterSecondBootstrap); err != nil {
		t.Fatalf("read root password after second bootstrap: %v", err)
	}
	if passwordAfterSecondBootstrap != string(changedHash) {
		t.Fatal("repeated bootstrap overwrote the existing root password")
	}

	for attempts := 0; attempts < 32; attempts++ {
		output := runGrove(t, ctx, repoRoot, configPath, commandEnv, "migrate", "down")
		if strings.Contains(output, "没有可回滚的迁移") {
			break
		}
		if attempts == 31 {
			t.Fatal("migrations did not reach the empty state")
		}
	}

	for _, relation := range []string{
		"users",
		"console_roles",
		"console_admins",
		"system_configs",
		"console_operation_logs",
		"console_login_logs",
		"console_sessions",
	} {
		assertRelationExists(t, db, relation, false)
	}
}

func writeCLIConfig(t *testing.T, container *postgres.PostgresContainer) string {
	t.Helper()
	ctx := context.Background()
	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("container host: %v", err)
	}
	port, err := container.MappedPort(ctx, "5432/tcp")
	if err != nil {
		t.Fatalf("container port: %v", err)
	}

	path := filepath.Join(t.TempDir(), "config.yaml")
	content := fmt.Sprintf(`app:
  env: production
jwt:
  secret: 0123456789abcdef0123456789abcdef
databases:
  default:
    enabled: true
    driver: postgres
    host: %s
    port: %s
    user: grove
    password: grove_test_password
    dbname: grove_test
    ssl_mode: disable
`, host, port.Port())
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write CLI config: %v", err)
	}
	return path
}

func runGrove(t *testing.T, ctx context.Context, repoRoot, configPath string, env []string, args ...string) string {
	t.Helper()
	output, err := runGroveWithError(ctx, repoRoot, configPath, env, args...)
	if err != nil {
		t.Fatalf("grove %s failed: %v\n%s", strings.Join(args, " "), err, output)
	}
	return output
}

func runGroveWithError(ctx context.Context, repoRoot, configPath string, env []string, args ...string) (string, error) {
	commandArgs := append([]string{"run", "./cmd/grove", "--config", configPath}, args...)
	cmd := exec.CommandContext(ctx, "go", commandArgs...)
	cmd.Dir = repoRoot
	cmd.Env = replaceEnvironment(os.Environ(), env...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func replaceEnvironment(current []string, replacements ...string) []string {
	keys := make(map[string]struct{}, len(replacements))
	for _, replacement := range replacements {
		key, _, _ := strings.Cut(replacement, "=")
		keys[key] = struct{}{}
	}

	result := make([]string, 0, len(current)+len(replacements))
	for _, item := range current {
		key, _, _ := strings.Cut(item, "=")
		if _, replaced := keys[key]; !replaced {
			result = append(result, item)
		}
	}
	return append(result, replacements...)
}

func assertRelationExists(t *testing.T, db *sql.DB, name string, expected bool) {
	t.Helper()
	var exists bool
	if err := db.QueryRow(`SELECT to_regclass($1) IS NOT NULL`, "public."+name).Scan(&exists); err != nil {
		t.Fatalf("check relation %s: %v", name, err)
	}
	if exists != expected {
		t.Fatalf("relation %s existence = %t, expected %t", name, exists, expected)
	}
}

func assertColumnExists(t *testing.T, db *sql.DB, table, column string, expected bool) {
	t.Helper()
	var exists bool
	if err := db.QueryRow(`
SELECT EXISTS (
    SELECT 1
    FROM information_schema.columns
    WHERE table_schema = 'public' AND table_name = $1 AND column_name = $2
)`, table, column).Scan(&exists); err != nil {
		t.Fatalf("check column %s.%s: %v", table, column, err)
	}
	if exists != expected {
		t.Fatalf("column %s.%s existence = %t, expected %t", table, column, exists, expected)
	}
}

func assertConstraintExists(t *testing.T, db *sql.DB, name string) {
	t.Helper()
	var exists bool
	if err := db.QueryRow(`SELECT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = $1)`, name).Scan(&exists); err != nil {
		t.Fatalf("check constraint %s: %v", name, err)
	}
	if !exists {
		t.Fatalf("constraint %s does not exist", name)
	}
}

func assertExecFails(t *testing.T, db *sql.DB, query string) {
	t.Helper()
	if _, err := db.Exec(query); err == nil {
		t.Fatalf("expected database constraint to reject query: %s", query)
	}
}
