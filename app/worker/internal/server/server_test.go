package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zhimma/grove/internal/config"
	"github.com/zhimma/grove/pkg/scheduler"
)

func TestWorkerExposesHealthAndMetrics(t *testing.T) {
	cfg := &config.Config{
		App:        config.AppConfig{Name: "grove", Env: "test"},
		WorkerPort: "0",
		Log:        config.LogConfig{Level: "error", Path: t.TempDir()},
		Server:     config.ServerConfig{ShutdownTimeout: 1},
		Databases: config.DatabasesConfig{
			Default: config.DatabaseConfig{Enabled: true},
		},
		Scheduler: config.SchedulerConfig{Enabled: true, Timezone: "UTC"},
		Observability: config.ObservabilityConfig{
			Enabled:          true,
			MetricsEnabled:   true,
			MetricsPath:      "/metrics",
			ReadinessTimeout: 1,
			TraceSampleRatio: 1,
		},
	}
	app, cleanup, err := NewServer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	for _, path := range []string{"/health/live", "/health/ready", "/metrics"} {
		resp := httptest.NewRecorder()
		app.health.Router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, path, nil))
		if resp.Code != http.StatusOK {
			t.Fatalf("%s expected 200, got %d body=%s", path, resp.Code, resp.Body.String())
		}
	}
}

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
		App:        config.AppConfig{Name: "grove", Env: "test"},
		WorkerPort: "0",
		Log:        config.LogConfig{Level: "error", Path: t.TempDir()},
		Server:     config.ServerConfig{ShutdownTimeout: 1},
		Scheduler:  config.SchedulerConfig{Enabled: true, Timezone: "UTC"},
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
