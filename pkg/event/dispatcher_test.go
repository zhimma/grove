package event

import (
	"context"
	"testing"
	"time"
)

// TestEvent 测试事件
type TestEvent struct {
	Name string
	Data string
}

func (e TestEvent) EventName() string {
	return "test.event"
}

// AnotherEvent 另一个测试事件
type AnotherEvent struct {
	ID int
}

func (e AnotherEvent) EventName() string {
	return "another.event"
}

func TestDispatcher_ListenAndDispatch(t *testing.T) {
	dispatcher := New()

	var received bool
	var receivedData string

	// 注册监听器
	dispatcher.ListenFunc("test.event", func(ctx context.Context, event Event) error {
		if e, ok := event.(TestEvent); ok {
			received = true
			receivedData = e.Data
		}
		return nil
	})

	// 分发事件
	event := TestEvent{Name: "test", Data: "hello"}
	err := dispatcher.Dispatch(context.Background(), event)
	if err != nil {
		t.Fatalf("dispatch failed: %v", err)
	}

	if !received {
		t.Fatal("listener should be called")
	}
	if receivedData != "hello" {
		t.Fatalf("expected hello, got %s", receivedData)
	}
}

func TestDispatcher_MultipleListeners(t *testing.T) {
	dispatcher := New()

	callCount := 0

	// 注册多个监听器
	dispatcher.ListenFunc("test.event", func(ctx context.Context, event Event) error {
		callCount++
		return nil
	})

	dispatcher.ListenFunc("test.event", func(ctx context.Context, event Event) error {
		callCount++
		return nil
	})

	dispatcher.Dispatch(context.Background(), TestEvent{})

	if callCount != 2 {
		t.Fatalf("expected 2 calls, got %d", callCount)
	}
}

func TestDispatcher_AsyncDispatch(t *testing.T) {
	dispatcher := NewAsync(100, 2)
	defer dispatcher.Close()

	done := make(chan bool, 1)

	dispatcher.ListenFunc("test.event", func(ctx context.Context, event Event) error {
		time.Sleep(50 * time.Millisecond)
		done <- true
		return nil
	})

	// 异步分发
	err := dispatcher.DispatchAsync(context.Background(), TestEvent{})
	if err != nil {
		t.Fatalf("async dispatch failed: %v", err)
	}

	// 等待处理完成
	select {
	case <-done:
		// 成功
	case <-time.After(1 * time.Second):
		t.Fatal("async listener timeout")
	}
}

func TestDispatcher_CloseIsIdempotent(t *testing.T) {
	dispatcher := NewAsync(1, 1)
	dispatcher.Close()
	dispatcher.Close()
}

func TestDispatcherCloseDrainsQueuedAsyncJobs(t *testing.T) {
	dispatcher := NewAsync(10, 1)
	done := make(chan struct{}, 2)

	dispatcher.ListenFunc("test.event", func(ctx context.Context, event Event) error {
		done <- struct{}{}
		return nil
	})

	if err := dispatcher.DispatchAsync(context.Background(), TestEvent{}); err != nil {
		t.Fatalf("dispatch first event: %v", err)
	}
	if err := dispatcher.DispatchAsync(context.Background(), TestEvent{}); err != nil {
		t.Fatalf("dispatch second event: %v", err)
	}

	closed := make(chan struct{})
	go func() {
		dispatcher.Close()
		close(closed)
	}()

	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("close should not hang with queued jobs")
	}

	for i := 0; i < 2; i++ {
		select {
		case <-done:
		default:
			t.Fatalf("expected queued job %d to be drained", i+1)
		}
	}
}

func TestDispatcher_NoListeners(t *testing.T) {
	dispatcher := New()

	// 分发没有监听器的事件
	err := dispatcher.Dispatch(context.Background(), AnotherEvent{ID: 1})
	if err != nil {
		t.Fatalf("dispatch should not fail: %v", err)
	}
}

func TestDispatcher_Forget(t *testing.T) {
	dispatcher := New()

	called := false
	dispatcher.ListenFunc("test.event", func(ctx context.Context, event Event) error {
		called = true
		return nil
	})

	// 忘记事件
	dispatcher.Forget("test.event")

	// 分发不应该触发监听器
	dispatcher.Dispatch(context.Background(), TestEvent{})

	if called {
		t.Fatal("listener should not be called after forget")
	}
}

func TestDispatcher_Flush(t *testing.T) {
	dispatcher := New()

	dispatcher.ListenFunc("event1", func(ctx context.Context, event Event) error { return nil })
	dispatcher.ListenFunc("event2", func(ctx context.Context, event Event) error { return nil })

	// 清空
	dispatcher.Flush()

	if dispatcher.HasListeners("event1") || dispatcher.HasListeners("event2") {
		t.Fatal("all listeners should be flushed")
	}
}

func TestDispatcher_HasListeners(t *testing.T) {
	dispatcher := New()

	if dispatcher.HasListeners("test.event") {
		t.Fatal("should not have listeners initially")
	}

	dispatcher.ListenFunc("test.event", func(ctx context.Context, event Event) error { return nil })

	if !dispatcher.HasListeners("test.event") {
		t.Fatal("should have listeners after register")
	}
}

func TestDispatcher_ListenerPanic(t *testing.T) {
	dispatcher := New()

	// 注册会panic的监听器
	dispatcher.ListenFunc("test.event", func(ctx context.Context, event Event) error {
		panic("test panic")
	})

	// 注册正常的监听器
	normalCalled := false
	dispatcher.ListenFunc("test.event", func(ctx context.Context, event Event) error {
		normalCalled = true
		return nil
	})

	// 分发不应该崩溃
	err := dispatcher.Dispatch(context.Background(), TestEvent{})
	if err != nil {
		t.Fatalf("dispatch should not fail: %v", err)
	}

	if !normalCalled {
		t.Fatal("normal listener should be called")
	}
}

func TestGlobalDispatcher(t *testing.T) {
	// 重置全局调度器
	defaultDispatcher = nil

	called := false
	ListenFunc("test.event", func(ctx context.Context, event Event) error {
		called = true
		return nil
	})

	err := Dispatch(context.Background(), TestEvent{})
	if err != nil {
		t.Fatalf("global dispatch failed: %v", err)
	}

	if !called {
		t.Fatal("global listener should be called")
	}
}

func TestSubscribe(t *testing.T) {
	dispatcher := New()

	var receivedEvent TestEvent
	Subscribe[TestEvent](dispatcher, func(ctx context.Context, event TestEvent) error {
		receivedEvent = event
		return nil
	})

	event := TestEvent{Name: "subscribed", Data: "data"}
	dispatcher.Dispatch(context.Background(), event)

	if receivedEvent.Name != "subscribed" {
		t.Fatalf("expected subscribed, got %s", receivedEvent.Name)
	}
}

// BenchmarkDispatcher_Dispatch 基准测试
func BenchmarkDispatcher_Dispatch(b *testing.B) {
	dispatcher := New()

	dispatcher.ListenFunc("test.event", func(ctx context.Context, event Event) error {
		return nil
	})

	ctx := context.Background()
	event := TestEvent{Name: "benchmark", Data: "data"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dispatcher.Dispatch(ctx, event)
	}
}

func BenchmarkDispatcher_DispatchParallel(b *testing.B) {
	dispatcher := New()

	dispatcher.ListenFunc("test.event", func(ctx context.Context, event Event) error {
		return nil
	})

	ctx := context.Background()
	event := TestEvent{Name: "benchmark", Data: "data"}

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			dispatcher.Dispatch(ctx, event)
		}
	})
}
