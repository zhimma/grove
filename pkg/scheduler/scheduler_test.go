package scheduler

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync/atomic"
	"testing"
	"time"
)

type pointerJob struct{}

func (*pointerJob) Run(context.Context) error { return nil }

func TestNewValidatesLocationAndAppliesDefaults(t *testing.T) {
	s, err := New(Config{})
	if err != nil {
		t.Fatal(err)
	}
	if s.location != time.Local || s.stopTimeout != 30*time.Second {
		t.Fatalf("location=%v stopTimeout=%v", s.location, s.stopTimeout)
	}
	if _, err := New(Config{Location: "not/a-timezone"}); err == nil {
		t.Fatal("expected invalid timezone error")
	}
}

func TestRegisterValidatesTask(t *testing.T) {
	s, _ := NewDefault()
	var nilPointerJob *pointerJob
	tests := []struct {
		name string
		task *Task
	}{
		{name: "nil task"},
		{name: "empty name", task: &Task{Schedule: "* * * * * *", Job: JobFunc(func(context.Context) error { return nil })}},
		{name: "empty schedule", task: &Task{Name: "task", Job: JobFunc(func(context.Context) error { return nil })}},
		{name: "nil job", task: &Task{Name: "task", Schedule: "* * * * * *"}},
		{name: "typed nil job", task: &Task{Name: "task", Schedule: "* * * * * *", Job: nilPointerJob}},
		{name: "nil job func", task: &Task{Name: "task", Schedule: "* * * * * *", Job: JobFunc(nil)}},
		{name: "negative timeout", task: &Task{Name: "task", Schedule: "* * * * * *", Job: JobFunc(func(context.Context) error { return nil }), Timeout: -time.Second}},
		{name: "invalid schedule", task: &Task{Name: "task", Schedule: "invalid", Job: JobFunc(func(context.Context) error { return nil })}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := s.Register(test.task); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestRegisterCopiesTaskAndRejectsDuplicates(t *testing.T) {
	s, _ := NewDefault()
	var originalCalls atomic.Int64
	var replacementCalls atomic.Int64
	task := &Task{
		Name:     "copy_task",
		Schedule: "0 0 0 * * *",
		Job: JobFunc(func(context.Context) error {
			originalCalls.Add(1)
			return nil
		}),
	}
	if err := s.Register(task); err != nil {
		t.Fatal(err)
	}
	task.Name = "mutated"
	task.Schedule = "invalid"
	task.Job = JobFunc(func(context.Context) error {
		replacementCalls.Add(1)
		return nil
	})

	if err := s.Run("copy_task"); err != nil {
		t.Fatal(err)
	}
	if originalCalls.Load() != 1 || replacementCalls.Load() != 0 {
		t.Fatalf("original=%d replacement=%d", originalCalls.Load(), replacementCalls.Load())
	}
	if err := s.RegisterFunc("copy_task", "0 0 0 * * *", func(context.Context) error { return nil }); err == nil {
		t.Fatal("expected duplicate task error")
	}
}

func TestTasksAreSortedAndRemoveDeletesCronEntry(t *testing.T) {
	s, _ := NewDefault()
	job := JobFunc(func(context.Context) error { return nil })
	if err := s.RegisterFunc("zeta", "0 0 0 * * *", job); err != nil {
		t.Fatal(err)
	}
	if err := s.RegisterFunc("alpha", "0 0 0 * * *", job); err != nil {
		t.Fatal(err)
	}
	if got := s.Tasks(); !reflect.DeepEqual(got, []string{"alpha", "zeta"}) {
		t.Fatalf("tasks = %#v", got)
	}

	s.mu.RLock()
	entryID := s.entries["alpha"]
	s.mu.RUnlock()
	if err := s.Remove("alpha"); err != nil {
		t.Fatal(err)
	}
	if entry := s.cron.Entry(entryID); entry.ID != 0 {
		t.Fatalf("cron entry still exists: %#v", entry)
	}
	if err := s.Run("alpha"); err == nil {
		t.Fatal("removed task must not be runnable")
	}
	if err := s.Remove("alpha"); err == nil {
		t.Fatal("removing missing task must fail")
	}
}

func TestMutexTaskPreventsOverlapAndReportsRunning(t *testing.T) {
	s, _ := NewDefault()
	started := make(chan struct{})
	release := make(chan struct{})
	firstDone := make(chan error, 1)
	if err := s.Register(&Task{
		Name:     "mutex_task",
		Schedule: "0 0 0 * * *",
		Mutex:    true,
		Job: JobFunc(func(context.Context) error {
			close(started)
			<-release
			return nil
		}),
	}); err != nil {
		t.Fatal(err)
	}
	go func() { firstDone <- s.Run("mutex_task") }()
	<-started
	if !s.IsRunning("mutex_task") {
		t.Fatal("task should report running")
	}
	if err := s.Run("mutex_task"); !errors.Is(err, ErrTaskRunning) {
		t.Fatalf("second run error = %v", err)
	}
	close(release)
	if err := <-firstDone; err != nil {
		t.Fatal(err)
	}
	if s.IsRunning("mutex_task") {
		t.Fatal("task should no longer report running")
	}
}

func TestRunReturnsJobError(t *testing.T) {
	s, _ := NewDefault()
	jobErr := errors.New("job failed")
	if err := s.RegisterFunc("failing", "0 0 0 * * *", func(context.Context) error { return jobErr }); err != nil {
		t.Fatal(err)
	}
	if err := s.Run("failing"); !errors.Is(err, jobErr) {
		t.Fatalf("run error = %v", err)
	}
}

func TestTaskTimeoutCancelsExecution(t *testing.T) {
	s, _ := NewDefault()
	if err := s.Register(&Task{
		Name:     "timeout",
		Schedule: "0 0 0 * * *",
		Timeout:  20 * time.Millisecond,
		Job: JobFunc(func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		}),
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.Run("timeout"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("run error = %v", err)
	}
}

func TestStopCancelsRunningTasksAndIsIdempotent(t *testing.T) {
	s, _ := New(Config{StopTimeout: time.Second})
	started := make(chan struct{})
	runDone := make(chan error, 1)
	if err := s.RegisterFunc("cancel", "0 0 0 * * *", func(ctx context.Context) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	}); err != nil {
		t.Fatal(err)
	}
	go func() { runDone <- s.Run("cancel") }()
	<-started
	if err := s.Stop(); err != nil {
		t.Fatal(err)
	}
	if err := <-runDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("run error = %v", err)
	}
	if err := s.Stop(); err != nil {
		t.Fatalf("second stop: %v", err)
	}
	if err := s.Start(); !errors.Is(err, ErrSchedulerStopped) {
		t.Fatalf("start after stop error = %v", err)
	}
	if err := s.RegisterFunc("late", "0 0 0 * * *", func(context.Context) error { return nil }); !errors.Is(err, ErrSchedulerStopped) {
		t.Fatalf("register after stop error = %v", err)
	}
	if err := s.Run("cancel"); !errors.Is(err, ErrSchedulerStopped) {
		t.Fatalf("run after stop error = %v", err)
	}
}

func TestStopReturnsTimeoutForUncooperativeTask(t *testing.T) {
	s, _ := New(Config{StopTimeout: 30 * time.Millisecond})
	started := make(chan struct{})
	release := make(chan struct{})
	runDone := make(chan error, 1)
	if err := s.RegisterFunc("blocked", "0 0 0 * * *", func(context.Context) error {
		close(started)
		<-release
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	go func() { runDone <- s.Run("blocked") }()
	<-started
	if err := s.Stop(); !errors.Is(err, ErrStopTimeout) {
		t.Fatalf("stop error = %v", err)
	}
	close(release)
	if err := <-runDone; err != nil {
		t.Fatal(err)
	}
	if err := s.Stop(); err != nil {
		t.Fatalf("stop after release: %v", err)
	}
}

func TestStartAndStopAreIdempotent(t *testing.T) {
	s, _ := NewDefault()
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	if err := s.Stop(); err != nil {
		t.Fatal(err)
	}
}

func TestConvenienceMethodsAndCronExpressions(t *testing.T) {
	s, _ := NewDefault()
	job := JobFunc(func(context.Context) error { return nil })
	registrations := []func() error{
		func() error { return s.EverySecond("second", job) },
		func() error { return s.EveryMinute("minute", job) },
		func() error { return s.EveryFiveMinutes("five", job) },
		func() error { return s.EveryTenMinutes("ten", job) },
		func() error { return s.EveryThirtyMinutes("thirty", job) },
		func() error { return s.Hourly("hourly", job) },
		func() error { return s.Daily("daily", job) },
		func() error { return s.DailyAt("daily_at", 8, 30, job) },
		func() error { return s.Weekly("weekly", job) },
		func() error { return s.Monthly("monthly", job) },
	}
	for _, register := range registrations {
		if err := register(); err != nil {
			t.Fatal(err)
		}
	}
	if CronExpression.EveryMinute != "0 * * * * *" || CronExpression.Daily != "0 0 0 * * *" {
		t.Fatalf("unexpected cron expressions: %#v", CronExpression)
	}
}

func TestGlobalScheduler(t *testing.T) {
	Init(nil)
	t.Cleanup(func() { Init(nil) })
	if err := RegisterFunc("test", "0 0 0 * * *", func(context.Context) error { return nil }); err == nil {
		t.Fatal("expected uninitialized scheduler error")
	}
	s, _ := NewDefault()
	Init(s)
	if err := RegisterFunc("test", "0 0 0 * * *", func(context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if err := Start(); err != nil {
		t.Fatal(err)
	}
	if err := Stop(); err != nil {
		t.Fatal(err)
	}
}

func BenchmarkSchedulerRegister(b *testing.B) {
	s, _ := NewDefault()
	job := JobFunc(func(context.Context) error { return nil })
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := s.RegisterFunc(fmt.Sprintf("task_%d", i), "0 0 0 * * *", job); err != nil {
			b.Fatal(err)
		}
	}
}
