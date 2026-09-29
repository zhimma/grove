package task

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
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
	dbs         *database.Connections
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
	return fixtureRow(t, f.dbs, name)
}

func fixtureRow(t *testing.T, dbs *database.Connections, name string) model.ConsoleScheduledTask {
	t.Helper()
	var row model.ConsoleScheduledTask
	if err := dbs.Default().Where("name = ?", name).First(&row).Error; err != nil {
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

func openReconcilerTestDB(t *testing.T) *database.Connections {
	t.Helper()
	dbs := openTaskTestDB(t)
	if err := dbs.Default().AutoMigrate(&model.ConsoleScheduledTask{}); err != nil {
		t.Fatalf("migrate scheduled tasks: %v", err)
	}
	return dbs
}

func TestReconcileRunsAndClearsAManualRequest(t *testing.T) {
	runs := make(chan struct{}, 4)
	fixture := newReconcilerFixture(t, Definition{
		Name: "report", DisplayName: "报表", Schedule: "0 0 * * * *", Mutex: true,
		Job: scheduler.JobFunc(func(context.Context) error {
			runs <- struct{}{}
			return nil
		}),
	})
	mustReconcile(t, fixture.reconciler)
	if err := fixture.scheduler.Start(); err != nil {
		t.Fatalf("start scheduler: %v", err)
	}

	requestedAt := time.Now()
	fixture.update(t, "report", map[string]any{"run_requested_at": requestedAt})
	mustReconcile(t, fixture.reconciler)

	select {
	case <-runs:
	case <-time.After(2 * time.Second):
		t.Fatal("manual request never ran the task")
	}
	waitUntil(t, func() bool { return !fixture.row(t, "report").HasPendingRunRequest() },
		"manual request was not cleared")

	// A cleared request must not run again on the next pass.
	mustReconcile(t, fixture.reconciler)
	select {
	case <-runs:
		t.Fatal("manual request ran twice")
	case <-time.After(200 * time.Millisecond):
	}
}

// Every replica reconciles the same table and can load the same pending row
// before any of them clears it, so the claim is what keeps a manual trigger
// from running once per replica. Exercised directly: driving it through
// Reconcile lets the database serialise the reads, which hides the race.
func TestOnlyOneWorkerClaimsAManualRequest(t *testing.T) {
	dbs := openReconcilerTestDB(t)
	definitions := map[string]Definition{"report": definition("report", "0 0 * * * *")}

	workers := make([]*Reconciler, 0, 4)
	for range 4 {
		sched, err := scheduler.New(scheduler.Config{Location: "UTC"})
		if err != nil {
			t.Fatalf("new scheduler: %v", err)
		}
		t.Cleanup(func() { _ = sched.Stop() })
		reconciler, err := NewReconciler(dbs, sched, definitions, time.Minute)
		if err != nil {
			t.Fatalf("new reconciler: %v", err)
		}
		workers = append(workers, reconciler)
	}
	mustReconcile(t, workers[0])

	if err := dbs.Default().Model(&model.ConsoleScheduledTask{}).
		Where("name = ?", "report").
		Update("run_requested_at", time.Now()).Error; err != nil {
		t.Fatalf("request run: %v", err)
	}

	var claims atomic.Int32
	var wg sync.WaitGroup
	start := make(chan struct{})
	for _, worker := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if worker.claimRunRequest(context.Background(), dbs.Default(), "report") {
				claims.Add(1)
			}
		}()
	}
	close(start)
	wg.Wait()

	if got := claims.Load(); got != 1 {
		t.Fatalf("%d workers claimed the same request, want exactly 1", got)
	}
	if fixtureRow(t, dbs, "report").HasPendingRunRequest() {
		t.Fatal("claimed request must leave the row with no pending flag")
	}
}

// A trigger that cannot run must still be answered. Leaving the flag set shows
// as "pending" in the console forever with no explanation.
func TestManualRequestThatCannotRunIsClearedAndExplained(t *testing.T) {
	for _, testCase := range []struct {
		name    string
		arrange func(t *testing.T, fixture *reconcilerFixture)
		reason  string
	}{
		{
			name: "disabled task",
			arrange: func(t *testing.T, fixture *reconcilerFixture) {
				fixture.update(t, "report", map[string]any{"enabled": false})
			},
			reason: "任务已停用，未执行",
		},
		{
			name: "task removed from the code",
			arrange: func(t *testing.T, fixture *reconcilerFixture) {
				fixture.reconciler.definitions = map[string]Definition{}
			},
			reason: "任务在代码中不存在，无法执行",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newReconcilerFixture(t, definition("report", "0 0 * * * *"))
			mustReconcile(t, fixture.reconciler)

			testCase.arrange(t, fixture)
			fixture.update(t, "report", map[string]any{"run_requested_at": time.Now()})
			mustReconcile(t, fixture.reconciler)

			row := fixture.row(t, "report")
			if row.HasPendingRunRequest() {
				t.Fatal("request must not stay pending when it cannot run")
			}
			if row.LastStatus != model.ScheduledTaskStatusSkipped {
				t.Fatalf("last status = %q, want %q", row.LastStatus, model.ScheduledTaskStatusSkipped)
			}
			if row.LastError != testCase.reason {
				t.Fatalf("last error = %q, want %q", row.LastError, testCase.reason)
			}
		})
	}
}

func waitUntil(t *testing.T, cond func() bool, message string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal(message)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// A GORM `default:true` tag makes the driver omit a false value from the
// INSERT, so the column default turns Mutex back on behind the seeder's back.
func TestSeedPreservesAMutexFalseDefinition(t *testing.T) {
	fixture := newReconcilerFixture(t, Definition{
		Name: "report", DisplayName: "报表", Schedule: "0 0 * * * *", Mutex: false,
		Job: scheduler.JobFunc(func(context.Context) error { return nil }),
	})

	mustReconcile(t, fixture.reconciler)

	if fixture.row(t, "report").Mutex {
		t.Fatal("seeded row must keep Mutex false as the definition declared it")
	}
}
