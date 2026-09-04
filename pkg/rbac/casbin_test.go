package rbac

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestReplaceConsolePoliciesForRolePreservesOldPoliciesOnFailure(t *testing.T) {
	db, enforcer := openRBACFailureTest(t)
	roleID := "console-role-operator"
	if err := enforcer.AddConsolePolicies([][]string{{roleID, "GET /console/v1/old"}}); err != nil {
		t.Fatalf("seed old policy: %v", err)
	}
	if err := db.Exec(`
CREATE TRIGGER fail_console_policy_insert
BEFORE INSERT ON console_casbin_rules
WHEN NEW.ptype = 'p' AND NEW.v1 = 'GET /console/v1/fail'
BEGIN
    SELECT RAISE(ABORT, 'injected policy failure');
END;`).Error; err != nil {
		t.Fatalf("create policy failure trigger: %v", err)
	}

	if err := enforcer.ReplaceConsolePoliciesForRole(roleID, []string{"GET /console/v1/fail"}); err == nil {
		t.Fatal("expected policy replacement to fail")
	}
	assertPoliciesEqual(t, enforcer, roleID, [][]string{{roleID, "GET /console/v1/old"}})
}

func TestReplaceConsoleRoleForUserPreservesOldGroupingOnFailure(t *testing.T) {
	db, enforcer := openRBACFailureTest(t)
	adminID := "console-admin-operator"
	if err := enforcer.AddConsoleRoleForUser(adminID, "console-role-old"); err != nil {
		t.Fatalf("seed old grouping: %v", err)
	}
	if err := db.Exec(`
CREATE TRIGGER fail_console_grouping_insert
BEFORE INSERT ON console_casbin_rules
WHEN NEW.ptype = 'g' AND NEW.v1 = 'console-role-fail'
BEGIN
    SELECT RAISE(ABORT, 'injected grouping failure');
END;`).Error; err != nil {
		t.Fatalf("create grouping failure trigger: %v", err)
	}

	if err := enforcer.ReplaceConsoleRoleForUser(adminID, "console-role-fail"); err == nil {
		t.Fatal("expected grouping replacement to fail")
	}
	assertGroupingsEqual(t, enforcer, adminID, [][]string{{adminID, "console-role-old"}})
}

func TestReplaceConsoleRoleForUserReplacesMultipleStaleGroupings(t *testing.T) {
	_, enforcer := openRBACFailureTest(t)
	adminID := "console-admin-operator"
	if _, err := enforcer.AddGroupingPolicies([][]string{
		{adminID, "console-role-old-a"},
		{adminID, "console-role-old-b"},
	}); err != nil {
		t.Fatalf("seed stale groupings: %v", err)
	}

	if err := enforcer.ReplaceConsoleRoleForUser(adminID, "console-role-current"); err != nil {
		t.Fatalf("replace stale groupings: %v", err)
	}
	assertGroupingsEqual(t, enforcer, adminID, [][]string{{adminID, "console-role-current"}})
	if err := enforcer.ReplaceConsoleRoleForUser(adminID, ""); err != nil {
		t.Fatalf("clear grouping: %v", err)
	}
	assertGroupingsEqual(t, enforcer, adminID, [][]string{})
}

func TestRefreshPolicyReloadsDatabaseChangesAndAdvancesVersion(t *testing.T) {
	db, enforcer := openRBACFailureTest(t)
	allowed, err := enforcer.Can("user-1", "GET /console/v1/roles")
	if err != nil {
		t.Fatalf("initial enforce: %v", err)
	}
	if allowed {
		t.Fatal("unexpected initial permission")
	}
	initialVersion := enforcer.PolicyVersion()
	if err := db.Exec("INSERT INTO console_casbin_rules (ptype, v0, v1) VALUES ('p', 'member', 'GET /console/v1/roles'), ('g', 'user-1', 'member')").Error; err != nil {
		t.Fatalf("insert policy: %v", err)
	}
	if allowed, err := enforcer.Can("user-1", "GET /console/v1/roles"); err != nil {
		t.Fatalf("enforce before refresh: %v", err)
	} else if allowed {
		t.Fatal("database policy should not be visible before refresh")
	}
	if err := enforcer.RefreshPolicy(); err != nil {
		t.Fatalf("refresh policy: %v", err)
	}
	allowed, err = enforcer.Can("user-1", "GET /console/v1/roles")
	if err != nil {
		t.Fatalf("enforce after refresh: %v", err)
	}
	if !allowed {
		t.Fatal("database policy should be visible after refresh")
	}
	if enforcer.PolicyVersion() <= initialVersion {
		t.Fatalf("policy version did not advance: initial=%d current=%d", initialVersion, enforcer.PolicyVersion())
	}
}

func TestNilEnforcerCanFailsClosed(t *testing.T) {
	var enforcer *Enforcer
	if _, err := enforcer.Can("user-1", "permission"); err == nil {
		t.Fatal("nil enforcer must return an error")
	}
}

const consoleCasbinRuleDDL = `
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
ON console_casbin_rules (ptype, v0, v1, v2, v3, v4, v5);`

func openRBACFailureTest(t *testing.T) (*gorm.DB, *Enforcer) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/rbac.db"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec(consoleCasbinRuleDDL).Error; err != nil {
		t.Fatalf("create casbin table: %v", err)
	}
	enforcer, err := New(db, &Config{TableName: "console_casbin_rules"})
	if err != nil {
		t.Fatalf("new enforcer: %v", err)
	}
	return db, enforcer
}

func assertPoliciesEqual(t *testing.T, enforcer *Enforcer, roleID string, expected [][]string) {
	t.Helper()
	actual, err := enforcer.GetConsolePoliciesForRole(roleID)
	if err != nil {
		t.Fatalf("get role policies: %v", err)
	}
	assertRulesEqual(t, actual, expected)
}

func assertGroupingsEqual(t *testing.T, enforcer *Enforcer, adminID string, expected [][]string) {
	t.Helper()
	actual, err := enforcer.GetFilteredGroupingPolicy(0, adminID)
	if err != nil {
		t.Fatalf("get user groupings: %v", err)
	}
	assertRulesEqual(t, actual, expected)
}

func assertRulesEqual(t *testing.T, actual, expected [][]string) {
	t.Helper()
	if len(actual) != len(expected) {
		t.Fatalf("rules length = %d, expected %d: %#v", len(actual), len(expected), actual)
	}
	for i := range expected {
		if len(actual[i]) != len(expected[i]) {
			t.Fatalf("rule %d length mismatch: %#v", i, actual[i])
		}
		for j := range expected[i] {
			if actual[i][j] != expected[i][j] {
				t.Fatalf("rule %d field %d = %q, expected %q", i, j, actual[i][j], expected[i][j])
			}
		}
	}
}

// A second instance must pick up a policy written by another instance. The
// enforcer only reloads on a timer, so this drives the shortest interval the
// API accepts and waits for the reload rather than asserting immediately.
func TestAutoLoadPolicyPicksUpPolicyWrittenByAnotherEnforcer(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/autoload.db"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec(consoleCasbinRuleDDL).Error; err != nil {
		t.Fatalf("create casbin table: %v", err)
	}

	writer, err := New(db, &Config{TableName: "console_casbin_rules"})
	if err != nil {
		t.Fatalf("new writer enforcer: %v", err)
	}
	reader, err := New(db, &Config{TableName: "console_casbin_rules", AutoLoadInterval: 20 * time.Millisecond})
	if err != nil {
		t.Fatalf("new reader enforcer: %v", err)
	}
	t.Cleanup(func() { _ = reader.Close() })

	if allowed, err := reader.Can("role-1", "GET /console/v1/roles"); err != nil || allowed {
		t.Fatalf("reader must not allow before the policy exists: allowed=%v err=%v", allowed, err)
	}
	if err := writer.AddConsolePolicies([][]string{{"role-1", "GET /console/v1/roles"}}); err != nil {
		t.Fatalf("write policy: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		allowed, err := reader.Can("role-1", "GET /console/v1/roles")
		if err != nil {
			t.Fatalf("enforce: %v", err)
		}
		if allowed {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("reader never reloaded the policy written by the other enforcer")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
