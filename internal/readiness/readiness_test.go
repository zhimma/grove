package readiness

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestCheckerReportsSortedDependenciesWithoutErrorDetails(t *testing.T) {
	wantErr := errors.New("password=must-not-leak")
	checker := New(map[string]Check{
		"redis": func(context.Context) error { return wantErr },
		"database.default": func(context.Context) error {
			return nil
		},
	}, time.Second)

	report := checker.Run(context.Background())
	if report.Ready {
		t.Fatal("expected readiness failure")
	}
	if len(report.Dependencies) != 2 || report.Dependencies[0].Name != "database.default" || report.Dependencies[1].Status != "unavailable" {
		t.Fatalf("unexpected report: %#v", report)
	}
	if report.Dependencies[1].err != wantErr {
		t.Fatalf("expected internal error to remain available for logs")
	}
	body, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), wantErr.Error()) {
		t.Fatalf("readiness JSON leaked dependency error: %s", body)
	}
}

func TestCheckerTimesOutIndividualDependency(t *testing.T) {
	checker := New(map[string]Check{
		"database.default": func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		},
	}, 10*time.Millisecond)

	report := checker.Run(context.Background())
	if report.Ready || len(report.Dependencies) != 1 || report.Dependencies[0].Status != "timeout" {
		t.Fatalf("unexpected timeout report: %#v", report)
	}
}

func TestCheckerReturnsWhenDependencyIgnoresContext(t *testing.T) {
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	var calls atomic.Int64
	checker := New(map[string]Check{
		"stuck": func(context.Context) error {
			calls.Add(1)
			<-release
			return nil
		},
	}, 10*time.Millisecond)

	startedAt := time.Now()
	report := checker.Run(context.Background())
	if elapsed := time.Since(startedAt); elapsed > 250*time.Millisecond {
		t.Fatalf("readiness exceeded its timeout: %s", elapsed)
	}
	if report.Ready || len(report.Dependencies) != 1 || report.Dependencies[0].Status != "timeout" {
		t.Fatalf("unexpected timeout report: %#v", report)
	}
	secondStarted := time.Now()
	second := checker.Run(context.Background())
	if elapsed := time.Since(secondStarted); elapsed > 100*time.Millisecond {
		t.Fatalf("second readiness probe should not wait for stuck check: %s", elapsed)
	}
	if second.Ready || len(second.Dependencies) != 1 || second.Dependencies[0].Status != "timeout" {
		t.Fatalf("unexpected concurrent timeout report: %#v", second)
	}
	if calls.Load() != 1 {
		t.Fatalf("stuck check should not be started repeatedly, calls=%d", calls.Load())
	}
}
