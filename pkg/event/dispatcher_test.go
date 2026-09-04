package event

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type testEvent struct {
	Data string
}

func (testEvent) EventName() string { return "test.event" }

type anotherEvent struct{}

func (anotherEvent) EventName() string { return "another.event" }

type pointerEvent struct{}

func (*pointerEvent) EventName() string { return "pointer.event" }

type pointerListener struct{}

func (*pointerListener) Handle(context.Context, Event) error { return nil }

func TestListenValidatesAndCopiesListeners(t *testing.T) {
	d := New(DefaultConfig())
	t.Cleanup(func() { _ = d.Close() })
	var nilListener *pointerListener
	if err := d.Listen("", ListenerFunc(func(context.Context, Event) error { return nil })); err == nil {
		t.Fatal("expected empty event name error")
	}
	if err := d.Listen("test.event", nil); err == nil {
		t.Fatal("expected nil listener error")
	}
	if err := d.Listen("test.event", nilListener); err == nil {
		t.Fatal("expected typed nil listener error")
	}
	if err := d.ListenFunc("test.event", nil); err == nil {
		t.Fatal("expected nil listener func error")
	}
	if err := d.ListenFunc("test.event", func(context.Context, Event) error { return nil }); err != nil {
		t.Fatal(err)
	}
	listeners := d.Listeners("test.event")
	listeners[0] = nil
	if d.Listeners("test.event")[0] == nil {
		t.Fatal("listeners must return a copy")
	}
}

func TestDispatchAggregatesErrorsAndPanic(t *testing.T) {
	d := New(DefaultConfig())
	t.Cleanup(func() { _ = d.Close() })
	firstErr := errors.New("first failed")
	var normalCalled atomic.Bool
	_ = d.ListenFunc("test.event", func(context.Context, Event) error { return firstErr })
	_ = d.ListenFunc("test.event", func(context.Context, Event) error { panic("boom") })
	_ = d.ListenFunc("test.event", func(context.Context, Event) error {
		normalCalled.Store(true)
		return nil
	})

	err := d.Dispatch(context.Background(), testEvent{})
	if !errors.Is(err, firstErr) {
		t.Fatalf("dispatch error does not contain listener error: %v", err)
	}
	var panicErr *ListenerPanicError
	if !errors.As(err, &panicErr) || panicErr.EventName != "test.event" || panicErr.Recovered != "boom" || len(panicErr.Stack) == 0 {
		t.Fatalf("panic error = %#v", panicErr)
	}
	if !normalCalled.Load() {
		t.Fatal("later listener was not executed")
	}
}

func TestDispatchValidatesEventAndAllowsNoListeners(t *testing.T) {
	d := New(DefaultConfig())
	t.Cleanup(func() { _ = d.Close() })
	if err := d.Dispatch(context.Background(), anotherEvent{}); err != nil {
		t.Fatalf("no-listener dispatch: %v", err)
	}
	var nilEvent *pointerEvent
	if err := d.Dispatch(context.Background(), nilEvent); err == nil {
		t.Fatal("expected typed nil event error")
	}
}

func TestDispatchAsyncWaitsForQueueOrContext(t *testing.T) {
	d := New(Config{QueueSize: 1, WorkerNum: 1})
	t.Cleanup(func() { _ = d.Close() })
	started := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int64
	_ = d.ListenFunc("test.event", func(context.Context, Event) error {
		if calls.Add(1) == 1 {
			close(started)
			<-release
		}
		return nil
	})

	if err := d.DispatchAsync(context.Background(), testEvent{Data: "running"}); err != nil {
		t.Fatal(err)
	}
	<-started
	if err := d.DispatchAsync(context.Background(), testEvent{Data: "queued"}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := d.DispatchAsync(ctx, testEvent{Data: "blocked"}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("dispatch error = %v", err)
	}
	close(release)
}

func TestTryDispatchAsyncReturnsQueueFull(t *testing.T) {
	d := New(Config{QueueSize: 1, WorkerNum: 1})
	t.Cleanup(func() { _ = d.Close() })
	started := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int64
	_ = d.ListenFunc("test.event", func(context.Context, Event) error {
		if calls.Add(1) == 1 {
			close(started)
			<-release
		}
		return nil
	})

	if err := d.TryDispatchAsync(context.Background(), testEvent{}); err != nil {
		t.Fatal(err)
	}
	<-started
	if err := d.TryDispatchAsync(context.Background(), testEvent{}); err != nil {
		t.Fatal(err)
	}
	if err := d.TryDispatchAsync(context.Background(), testEvent{}); !errors.Is(err, ErrQueueFull) {
		t.Fatalf("try dispatch error = %v", err)
	}
	close(release)
}

func TestAsyncContextKeepsValuesWithoutCancellation(t *testing.T) {
	type contextKey string
	const requestIDKey contextKey = "request_id"
	d := New(Config{QueueSize: 1, WorkerNum: 1})
	t.Cleanup(func() { _ = d.Close() })
	started := make(chan struct{})
	release := make(chan struct{})
	result := make(chan struct {
		value any
		err   error
	}, 1)
	_ = d.ListenFunc("test.event", func(ctx context.Context, _ Event) error {
		close(started)
		<-release
		result <- struct {
			value any
			err   error
		}{value: ctx.Value(requestIDKey), err: ctx.Err()}
		return nil
	})

	base := context.WithValue(context.Background(), requestIDKey, "req-123")
	ctx, cancel := context.WithCancel(base)
	if err := d.DispatchAsync(ctx, testEvent{}); err != nil {
		t.Fatal(err)
	}
	<-started
	cancel()
	close(release)
	got := <-result
	if got.value != "req-123" || got.err != nil {
		t.Fatalf("value=%v err=%v", got.value, got.err)
	}
}

func TestAsyncDispatchUsesListenerSnapshot(t *testing.T) {
	d := New(Config{QueueSize: 2, WorkerNum: 1})
	t.Cleanup(func() { _ = d.Close() })
	started := make(chan struct{})
	release := make(chan struct{})
	snapshotDone := make(chan struct{})
	var lateCalls atomic.Int64
	_ = d.ListenFunc("test.event", func(_ context.Context, raw Event) error {
		event := raw.(testEvent)
		switch event.Data {
		case "block":
			close(started)
			<-release
		case "snapshot":
			close(snapshotDone)
		}
		return nil
	})
	if err := d.DispatchAsync(context.Background(), testEvent{Data: "block"}); err != nil {
		t.Fatal(err)
	}
	<-started
	if err := d.DispatchAsync(context.Background(), testEvent{Data: "snapshot"}); err != nil {
		t.Fatal(err)
	}
	if err := d.ListenFunc("test.event", func(context.Context, Event) error {
		lateCalls.Add(1)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	close(release)
	<-snapshotDone
	if lateCalls.Load() != 0 {
		t.Fatalf("late listener received queued snapshot: %d", lateCalls.Load())
	}
	if err := d.Dispatch(context.Background(), testEvent{Data: "after"}); err != nil {
		t.Fatal(err)
	}
	if lateCalls.Load() != 1 {
		t.Fatalf("late listener was not active for later dispatch: %d", lateCalls.Load())
	}
}

func TestAsyncErrorsAreReported(t *testing.T) {
	listenerErr := errors.New("listener failed")
	reported := make(chan error, 1)
	d := New(Config{
		QueueSize: 1,
		WorkerNum: 1,
		ErrorHandler: func(_ context.Context, _ Event, err error) {
			reported <- err
		},
	})
	t.Cleanup(func() { _ = d.Close() })
	_ = d.ListenFunc("test.event", func(context.Context, Event) error { return listenerErr })
	_ = d.ListenFunc("test.event", func(context.Context, Event) error { panic("async panic") })

	if err := d.DispatchAsync(context.Background(), testEvent{}); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-reported:
		if !errors.Is(err, listenerErr) {
			t.Fatalf("reported error = %v", err)
		}
		var panicErr *ListenerPanicError
		if !errors.As(err, &panicErr) || panicErr.Recovered != "async panic" {
			t.Fatalf("reported panic = %#v", panicErr)
		}
	case <-time.After(time.Second):
		t.Fatal("async error was not reported")
	}
}

func TestErrorHandlerPanicDoesNotStopWorker(t *testing.T) {
	var reports atomic.Int64
	processed := make(chan struct{}, 2)
	d := New(Config{
		QueueSize: 2,
		WorkerNum: 1,
		ErrorHandler: func(context.Context, Event, error) {
			reports.Add(1)
			panic("handler panic")
		},
	})
	t.Cleanup(func() { _ = d.Close() })
	_ = d.ListenFunc("test.event", func(context.Context, Event) error {
		processed <- struct{}{}
		return errors.New("fail")
	})
	if err := d.DispatchAsync(context.Background(), testEvent{}); err != nil {
		t.Fatal(err)
	}
	if err := d.DispatchAsync(context.Background(), testEvent{}); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		select {
		case <-processed:
		case <-time.After(time.Second):
			t.Fatal("worker stopped after error handler panic")
		}
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	if reports.Load() != 2 {
		t.Fatalf("reports = %d", reports.Load())
	}
}

func TestCloseDrainsAcceptedWorkAndRejectsNewEvents(t *testing.T) {
	d := New(Config{QueueSize: 4, WorkerNum: 1})
	var calls atomic.Int64
	_ = d.ListenFunc("test.event", func(context.Context, Event) error {
		calls.Add(1)
		return nil
	})
	for range 4 {
		if err := d.DispatchAsync(context.Background(), testEvent{}); err != nil {
			t.Fatal(err)
		}
	}
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := d.Close(); err != nil {
				t.Errorf("close: %v", err)
			}
		}()
	}
	wg.Wait()
	if calls.Load() != 4 {
		t.Fatalf("drained calls = %d", calls.Load())
	}
	if err := d.Dispatch(context.Background(), testEvent{}); !errors.Is(err, ErrClosed) {
		t.Fatalf("sync after close = %v", err)
	}
	if err := d.DispatchAsync(context.Background(), testEvent{}); !errors.Is(err, ErrClosed) {
		t.Fatalf("async after close = %v", err)
	}
	if err := d.TryDispatchAsync(context.Background(), testEvent{}); !errors.Is(err, ErrClosed) {
		t.Fatalf("try after close = %v", err)
	}
	if err := d.ListenFunc("test.event", func(context.Context, Event) error { return nil }); !errors.Is(err, ErrClosed) {
		t.Fatalf("listen after close = %v", err)
	}
}

func TestCloseWaitsForActiveSynchronousDispatch(t *testing.T) {
	d := New(Config{QueueSize: 1, WorkerNum: 1})
	started := make(chan struct{})
	release := make(chan struct{})
	dispatchDone := make(chan error, 1)
	_ = d.ListenFunc("test.event", func(context.Context, Event) error {
		close(started)
		<-release
		return nil
	})
	go func() { dispatchDone <- d.Dispatch(context.Background(), testEvent{}) }()
	<-started
	closed := make(chan error, 1)
	go func() { closed <- d.Close() }()
	select {
	case err := <-closed:
		t.Fatalf("close returned before dispatch completed: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	if err := <-dispatchDone; err != nil {
		t.Fatal(err)
	}
	if err := <-closed; err != nil {
		t.Fatal(err)
	}
}

func TestForgetFlushAndHasListeners(t *testing.T) {
	d := New(DefaultConfig())
	t.Cleanup(func() { _ = d.Close() })
	_ = d.ListenFunc("test.event", func(context.Context, Event) error { return nil })
	_ = d.ListenFunc("another.event", func(context.Context, Event) error { return nil })
	if !d.HasListeners("test.event") {
		t.Fatal("missing registered listener")
	}
	d.Forget("test.event")
	if d.HasListeners("test.event") {
		t.Fatal("forgotten listener remains")
	}
	d.Flush()
	if d.HasListeners("another.event") {
		t.Fatal("flush did not remove listeners")
	}
}

func TestSubscribe(t *testing.T) {
	d := New(DefaultConfig())
	t.Cleanup(func() { _ = d.Close() })
	var received testEvent
	if err := Subscribe[testEvent](d, func(_ context.Context, event testEvent) error {
		received = event
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := d.Dispatch(context.Background(), testEvent{Data: "payload"}); err != nil {
		t.Fatal(err)
	}
	if received.Data != "payload" {
		t.Fatalf("received = %#v", received)
	}
	var pointerCalled atomic.Bool
	if err := Subscribe[*pointerEvent](d, func(_ context.Context, event *pointerEvent) error {
		pointerCalled.Store(event != nil)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := d.Dispatch(context.Background(), &pointerEvent{}); err != nil {
		t.Fatal(err)
	}
	if !pointerCalled.Load() {
		t.Fatal("pointer event subscription was not called")
	}
}

func BenchmarkDispatcherDispatch(b *testing.B) {
	d := New(DefaultConfig())
	b.Cleanup(func() { _ = d.Close() })
	_ = d.ListenFunc("test.event", func(context.Context, Event) error { return nil })
	for i := 0; i < b.N; i++ {
		if err := d.Dispatch(context.Background(), testEvent{Data: fmt.Sprint(i)}); err != nil {
			b.Fatal(err)
		}
	}
}
