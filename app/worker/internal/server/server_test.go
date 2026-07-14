package server

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zhimma/grove/internal/config"
	"github.com/zhimma/grove/pkg/scheduler"
)

func TestNewServerRejectsDisabledWorker(t *testing.T) {
	cfg := &config.Config{
		App: config.AppConfig{Name: "grove", Env: "test"},
		Log: config.LogConfig{Level: "error", Path: t.TempDir()},
	}
	app, cleanup, err := NewServer(cfg)
	if !errors.Is(err, ErrWorkerDisabled) {
		t.Fatalf("expected ErrWorkerDisabled, got %v", err)
	}
	if app != nil || cleanup != nil {
		t.Fatal("disabled worker must not create an app")
	}
}

func TestWorkerStartsSchedulerWhenQueueIsDisabled(t *testing.T) {
	cfg := &config.Config{
		App:       config.AppConfig{Name: "grove", Env: "test"},
		Log:       config.LogConfig{Level: "error", Path: t.TempDir()},
		Server:    config.ServerConfig{ShutdownTimeout: 1},
		Scheduler: config.SchedulerConfig{Enabled: true, Timezone: "UTC"},
	}
	app, cleanup, err := NewServer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	if app.provider.Scheduler == nil || app.provider.JobServer != nil {
		t.Fatalf("scheduler=%v jobServer=%v", app.provider.Scheduler, app.provider.JobServer)
	}

	var calls atomic.Int64
	if err := app.provider.Scheduler.Register(&scheduler.Task{
		Name:     "worker_start_probe",
		Schedule: "* * * * * *",
		Job: scheduler.JobFunc(func(context.Context) error {
			calls.Add(1)
			return nil
		}),
	}); err != nil {
		t.Fatal(err)
	}
	if err := app.Start(); err != nil {
		t.Fatal(err)
	}
	deadline := time.NewTimer(1500 * time.Millisecond)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for calls.Load() == 0 {
		select {
		case <-deadline.C:
			t.Fatal("worker start did not start scheduler")
		case <-ticker.C:
		}
	}
	if err := app.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
}
