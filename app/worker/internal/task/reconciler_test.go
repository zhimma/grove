package task

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/zhimma/grove/internal/model"
	"github.com/zhimma/grove/pkg/database"
	"github.com/zhimma/grove/pkg/scheduler"
)

func TestReconcileSeedsARowForEveryDefinedTask(t *testing.T) {
	fixture := newReconcilerFixture(t, definition("report", "0 0 * * * *"))

	mustReconcile(t, fixture.reconciler)

	row := fixture.row(t, "report")
	if row.Schedule != "0 0 * * * *" || !row.Enabled {
		t.Fatalf("seeded row = %+v, want the definition's schedule and enabled", row)
	}
	assertScheduled(t, fixture.scheduler, "report", true)
}

func TestReconcileKeepsOperatorEditsAcrossRestarts(t *testing.T) {
	fixture := newReconcilerFixture(t, definition("report", "0 0 * * * *"))
	mustReconcile(t, fixture.reconciler)

	// An operator retunes the schedule from the console.
	fixture.update(t, "report", map[string]any{"schedule": "0 30 * * * *"})

	// A redeploy re-seeds from the same definitions and must not reset it.
	restarted := fixture.restart(t)
	mustReconcile(t, restarted)

	if got := fixture.row(t, "report").Schedule; got != "0 30 * * * *" {
		t.Fatalf("schedule after restart = %q, want the operator's value", got)
	}
}

func TestReconcileAppliesAScheduleChange(t *testing.T) {
	fixture := newReconcilerFixture(t, definition("report", "0 0 * * * *"))
	mustReconcile(t, fixture.reconciler)

	fixture.update(t, "report", map[string]any{"schedule": "0 30 * * * *"})
	mustReconcile(t, fixture.reconciler)

	if got := fixture.reconciler.applied["report"].schedule; got != "0 30 * * * *" {
		t.Fatalf("applied schedule = %q, want the edited value", got)
	}
	assertScheduled(t, fixture.scheduler, "report", true)
}

func TestReconcileRemovesADisabledTaskAndRestoresIt(t *testing.T) {
	fixture := newReconcilerFixture(t, definition("report", "0 0 * * * *"))
	mustReconcile(t, fixture.reconciler)

	fixture.update(t, "report", map[string]any{"enabled": false})
	mustReconcile(t, fixture.reconciler)
	assertScheduled(t, fixture.scheduler, "report", false)

	fixture.update(t, "report", map[string]any{"enabled": true})
	mustReconcile(t, fixture.reconciler)
	assertScheduled(t, fixture.scheduler, "report", true)
}

func TestReconcileIgnoresARowWithNoHandler(t *testing.T) {
	fixture := newReconcilerFixture(t, definition("report", "0 0 * * * *"))
	orphan := model.ConsoleScheduledTask{Name: "removed-in-a-later-release", Schedule: "0 0 * * * *", Enabled: true}
	if err := fixture.dbs.Default().Create(&orphan).Error; err != nil {
		t.Fatalf("seed orphan row: %v", err)
	}

	mustReconcile(t, fixture.reconciler)

	assertScheduled(t, fixture.scheduler, "removed-in-a-later-release", false)
	assertScheduled(t, fixture.scheduler, "report", true)
}

func TestReconcileSkipsARowWhoseScheduleNoLongerParses(t *testing.T) {
	fixture := newReconcilerFixture(t, definition("report", "0 0 * * * *"), definition("digest", "0 0 * * * *"))
	mustReconcile(t, fixture.reconciler)

	// Written straight to the column: Console validates, but an older row or a
	// manual edit can still hold something the parser rejects.
	fixture.update(t, "report", map[string]any{"schedule": "not-a-cron"})
	mustReconcile(t, fixture.reconciler)

	assertScheduled(t, fixture.scheduler, "report", false)
	assertScheduled(t, fixture.scheduler, "digest", true)
}

func TestInstrumentedRunRecordsSuccessAndFailure(t *testing.T) {
	failure := errors.New("upstream unavailable")
	calls := 0
	job := scheduler.JobFunc(func(context.Context) error {
		calls++
		if calls == 1 {
			return nil
		}
		return failure
	})

	fixture := newReconcilerFixture(t, Definition{
		Name: "report", DisplayName: "报表", Schedule: "0 0 * * * *", Mutex: true, Job: job,
	})
	mustReconcile(t, fixture.reconciler)
	instrumented := fixture.reconciler.instrument(fixture.reconciler.definitions["report"])

	if err := instrumented(context.Background()); err != nil {
		t.Fatalf("first run: %v", err)
	}
	row := fixture.row(t, "report")
	if row.LastStatus != model.ScheduledTaskStatusSuccess || row.LastRunAt == nil || row.LastError != "" {
		t.Fatalf("after success row = %+v", row)
	}

	if err := instrumented(context.Background()); !errors.Is(err, failure) {
		t.Fatalf("second run error = %v, want the job's error", err)
	}
	row = fixture.row(t, "report")
	if row.LastStatus != model.ScheduledTaskStatusFailed || row.LastError != failure.Error() {
		t.Fatalf("after failure row = %+v", row)
	}
}

func TestRecordRunTruncatesAnOversizedError(t *testing.T) {
	oversized := make([]byte, lastErrorLimit+50)
	for i := range oversized {
		oversized[i] = 'x'
	}
	failure := errors.New(string(oversized))

	fixture := newReconcilerFixture(t, Definition{
		Name: "report", DisplayName: "报表", Schedule: "0 0 * * * *",
		Job: scheduler.JobFunc(func(context.Context) error { return failure }),
	})
	mustReconcile(t, fixture.reconciler)

	if err := fixture.reconciler.instrument(fixture.reconciler.definitions["report"])(context.Background()); err == nil {
		t.Fatal("expected the job error to propagate")
	}
	if got := len(fixture.row(t, "report").LastError); got != lastErrorLimit {
		t.Fatalf("stored error length = %d, want %d", got, lastErrorLimit)
	}
}

func TestRunStopsWhenContextIsCancelled(t *testing.T) {
	fixture := newReconcilerFixture(t, definition("report", "0 0 * * * *"))
	fixture.reconciler.interval = 10 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		fixture.reconciler.Run(ctx)
	}()

	cancel()
	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after its context was cancelled")
	}
}

type reconcilerFixture struct {
	dbs         database.Connections
	scheduler   *scheduler.Scheduler
	reconciler  *Reconciler
	definitions map[string]Definition
}

func newReconcilerFixture(t *testing.T, definitions ...Definition) *reconcilerFixture {
	t.Helper()
	dbs := openReconcilerTestDB(t)
	registry := make(map[string]Definition, len(definitions))
	for _, item := range definitions {
		registry[item.Name] = item
	}

	fixture := &reconcilerFixture{dbs: dbs, definitions: registry}
	fixture.scheduler, fixture.reconciler = fixture.newReconciler(t)
	return fixture
}

func (f *reconcilerFixture) newReconciler(t *testing.T) (*scheduler.Scheduler, *Reconciler) {
	t.Helper()
	sched, err := scheduler.New(scheduler.Config{Location: "UTC"})
	if err != nil {
		t.Fatalf("new scheduler: %v", err)
	}
	t.Cleanup(func() { _ = sched.Stop() })

	reconciler, err := NewReconciler(f.dbs, sched, f.definitions, time.Minute)
	if err != nil {
		t.Fatalf("new reconciler: %v", err)
	}
	return sched, reconciler
}

// restart models a redeploy: same table, a fresh scheduler and reconciler.
func (f *reconcilerFixture) restart(t *testing.T) *Reconciler {
	t.Helper()
	_, reconciler := f.newReconciler(t)
	return reconciler
}

func (f *reconcilerFixture) row(t *testing.T, name string) model.ConsoleScheduledTask {
	t.Helper()
	var row model.ConsoleScheduledTask
	if err := f.dbs.Default().Where("name = ?", name).First(&row).Error; err != nil {
		t.Fatalf("load task %q: %v", name, err)
	}
	return row
}

func (f *reconcilerFixture) update(t *testing.T, name string, values map[string]any) {
	t.Helper()
	if err := f.dbs.Default().
		Model(&model.ConsoleScheduledTask{}).
		Where("name = ?", name).
		Updates(values).Error; err != nil {
		t.Fatalf("update task %q: %v", name, err)
	}
}

func definition(name, schedule string) Definition {
	return Definition{
		Name:        name,
		DisplayName: name,
		Schedule:    schedule,
		Mutex:       true,
		Job:         scheduler.JobFunc(func(context.Context) error { return nil }),
	}
}

func mustReconcile(t *testing.T, reconciler *Reconciler) {
	t.Helper()
	if err := reconciler.Reconcile(context.Background()); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
}

func assertScheduled(t *testing.T, sched *scheduler.Scheduler, name string, want bool) {
	t.Helper()
	found := false
	for _, registered := range sched.Tasks() {
		if registered == name {
			found = true
			break
		}
	}
	if found != want {
		t.Fatalf("task %q scheduled = %v, want %v (registered: %v)", name, found, want, sched.Tasks())
	}
}

func openReconcilerTestDB(t *testing.T) database.Connections {
	t.Helper()
	dbs := openTaskTestDB(t)
	if err := dbs.Default().AutoMigrate(&model.ConsoleScheduledTask{}); err != nil {
		t.Fatalf("migrate scheduled tasks: %v", err)
	}
	return dbs
}
