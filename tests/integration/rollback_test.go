//go:build integration

package integration_test

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
)

// The same conflicting data is legal in both new schemas and illegal in the
// old schema. A refusal must preserve both data and migration metadata.
func assertIntegrityRollbackPreflight(t *testing.T, ctx context.Context, db *sql.DB, root, config string, env []string) {
	t.Helper()
	var version int64
	var dirty bool
	if err := db.QueryRowContext(ctx, `SELECT version, dirty FROM grove_migrations`).Scan(&version, &dirty); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return // All migrations have already been rolled back.
		}
		t.Fatal(err)
	}
	if version != 202604150009 {
		return
	}
	output, err := runGroveWithError(ctx, root, config, env, "migrate", "down")
	if err == nil || !strings.Contains(output, "cannot restore legacy uniqueness on users") {
		t.Fatalf("expected safe rollback refusal, error=%v output=%s", err, output)
	}
	var after int64
	if err := db.QueryRowContext(ctx, `SELECT version, dirty FROM grove_migrations`).Scan(&after, &dirty); err != nil {
		t.Fatal(err)
	}
	if after != version || dirty {
		t.Fatalf("refused rollback changed metadata: version=%d dirty=%t", after, dirty)
	}
	var rows int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE email = 'reuse@example.test'`).Scan(&rows); err != nil || rows != 2 {
		t.Fatalf("refused rollback changed data: rows=%d error=%v", rows, err)
	}
	// The active uniqueness rule must still exist: MySQL's old down file drops
	// this index before discovering incompatible data at a later statement.
	if _, err := db.ExecContext(ctx, `INSERT INTO users (id, name, email) VALUES ('rollback-duplicate', 'Duplicate', 'reuse@example.test')`); err == nil {
		t.Fatal("refused rollback removed active uniqueness constraint")
	}
	// Explicitly reconcile this test fixture, then let the caller prove the
	// complete down lifecycle. Production code never mutates conflicting rows.
	if _, err := db.ExecContext(ctx, `UPDATE users SET email = 'original@example.test' WHERE id = 'soft-delete-user-1'`); err != nil {
		t.Fatal(err)
	}
}
