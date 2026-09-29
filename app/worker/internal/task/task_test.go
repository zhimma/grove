package task

import (
	"context"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/zhimma/grove/internal/model"
	"github.com/zhimma/grove/internal/testkit"
	"github.com/zhimma/grove/pkg/database"
	"github.com/zhimma/grove/pkg/scheduler"
)

func TestDefinitionsAreRunnableAsDeclared(t *testing.T) {
	registry, err := Definitions(openTaskTestDB(t))
	if err != nil {
		t.Fatalf("definitions: %v", err)
	}
	if len(registry) == 0 {
		t.Fatal("registry must declare at least one task")
	}

	for name, definition := range registry {
		if definition.Name != name {
			t.Errorf("registry key %q does not match definition name %q", name, definition.Name)
		}
		if definition.DisplayName == "" {
			t.Errorf("task %q needs a display name for the console list", name)
		}
		// A schedule the parser rejects would drop the task at reconcile time
		// with no console-visible reason, so it has to fail the build instead.
		if err := scheduler.ValidateSchedule(definition.Schedule); err != nil {
			t.Errorf("task %q: %v", name, err)
		}
	}
}

func TestDefinitionsRejectsMissingDatabase(t *testing.T) {
	if _, err := Definitions(nil); err == nil {
		t.Fatal("registry must not build without database connections")
	}
}

func TestPurgeExpiredSessionsDeletesOnlyExpiredRows(t *testing.T) {
	dbs := openTaskTestDB(t)
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	revokedAt := now.Add(-time.Hour)

	seed := []model.ConsoleSession{
		{AuditBase: model.AuditBase{ID: "expired"}, AdminID: "admin-1", RefreshTokenHash: "hash-expired", ExpiresAt: now.Add(-time.Minute), LastActiveAt: now.Add(-time.Hour)},
		{AuditBase: model.AuditBase{ID: "expiring-now"}, AdminID: "admin-1", RefreshTokenHash: "hash-now", ExpiresAt: now, LastActiveAt: now.Add(-time.Hour)},
		{AuditBase: model.AuditBase{ID: "active"}, AdminID: "admin-1", RefreshTokenHash: "hash-active", ExpiresAt: now.Add(time.Hour), LastActiveAt: now},
		// Revoked but unexpired: still part of the visible session history.
		{AuditBase: model.AuditBase{ID: "revoked"}, AdminID: "admin-1", RefreshTokenHash: "hash-revoked", ExpiresAt: now.Add(time.Hour), LastActiveAt: now, RevokedAt: &revokedAt},
	}
	if err := dbs.Default().Create(&seed).Error; err != nil {
		t.Fatalf("seed sessions: %v", err)
	}

	purge := newPurgeExpiredSessions(dbs)
	purge.now = func() time.Time { return now }
	if err := purge.Run(context.Background()); err != nil {
		t.Fatalf("purge: %v", err)
	}

	var remaining []string
	if err := dbs.Default().Model(&model.ConsoleSession{}).Order("id").Pluck("id", &remaining).Error; err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if want := []string{"active", "revoked"}; !slices.Equal(remaining, want) {
		t.Fatalf("remaining sessions = %v, want %v", remaining, want)
	}
}

func TestPurgeExpiredSessionsClearsBacklogLargerThanOneBatch(t *testing.T) {
	dbs := openTaskTestDB(t)
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)

	total := purgeBatchSize + 7
	sessions := make([]model.ConsoleSession, 0, total)
	for i := range total {
		sessions = append(sessions, model.ConsoleSession{
			AuditBase:        model.AuditBase{ID: "stale-" + strconv.Itoa(i)},
			AdminID:          "admin-1",
			RefreshTokenHash: "hash-" + strconv.Itoa(i),
			ExpiresAt:        now.Add(-time.Minute),
			LastActiveAt:     now.Add(-time.Hour),
		})
	}
	if err := dbs.Default().CreateInBatches(&sessions, 500).Error; err != nil {
		t.Fatalf("seed sessions: %v", err)
	}

	purge := newPurgeExpiredSessions(dbs)
	purge.now = func() time.Time { return now }
	if err := purge.Run(context.Background()); err != nil {
		t.Fatalf("purge: %v", err)
	}

	var left int64
	if err := dbs.Default().Model(&model.ConsoleSession{}).Count(&left).Error; err != nil {
		t.Fatalf("count sessions: %v", err)
	}
	if left != 0 {
		t.Fatalf("purge left %d expired sessions, want 0", left)
	}
}

func TestPurgeExpiredSessionsStopsOnCancelledContext(t *testing.T) {
	dbs := openTaskTestDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := newPurgeExpiredSessions(dbs).Run(ctx); err == nil {
		t.Fatal("purge must surface a cancelled context instead of running to completion")
	}
}

func openTaskTestDB(t *testing.T) database.Connections {
	t.Helper()
	db := testkit.OpenDB(t, &model.ConsoleSession{})
	return database.NewConnectionsFromDBs(db, nil)
}
