package scheduler

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	s, err := NewDefault()
	if err != nil {
		t.Fatalf("create scheduler failed: %v", err)
	}

	if s == nil {
		t.Fatal("scheduler should not be nil")
	}
}

func TestScheduler_RegisterAndRun(t *testing.T) {
	s, _ := NewDefault()

	executed := make(chan bool, 1)

	err := s.RegisterFunc("test_task", "* * * * * *", func(ctx context.Context) error {
		executed <- true
		return nil
	})

	if err != nil {
		t.Fatalf("register failed: %v", err)
	}

	// 手动运行
	go s.Run("test_task")

	select {
	case <-executed:
		// 成功
	case <-time.After(2 * time.Second):
		t.Fatal("task not executed")
	}
}

func TestScheduler_Mutex(t *testing.T) {
	s, _ := NewDefault()

	executionCount := 0
	executing := make(chan bool, 1)

	err := s.Register(&Task{
		Name:     "mutex_task",
		Schedule: "* * * * * *",
		Mutex:    true,
		Job: JobFunc(func(ctx context.Context) error {
			executionCount++
			executing <- true
			time.Sleep(100 * time.Millisecond)
			<-executing
			return nil
		}),
	})

	if err != nil {
		t.Fatalf("register failed: %v", err)
	}

	// 并发运行两次
	go s.Run("mutex_task")
	go s.Run("mutex_task")

	time.Sleep(50 * time.Millisecond)

	// 由于互斥锁，应该只有一个在执行
	if executionCount != 1 {
		t.Fatalf("expected 1 execution with mutex, got %d", executionCount)
	}
}

func TestScheduler_DuplicateTask(t *testing.T) {
	s, _ := NewDefault()

	err := s.RegisterFunc("dup_task", "0 0 0 * * *", func(ctx context.Context) error {
		return nil
	})

	if err != nil {
		t.Fatalf("first register should succeed: %v", err)
	}

	// 重复注册
	err = s.RegisterFunc("dup_task", "0 0 0 * * *", func(ctx context.Context) error {
		return nil
	})

	if err == nil {
		t.Fatal("duplicate register should fail")
	}
}

func TestScheduler_Tasks(t *testing.T) {
	s, _ := NewDefault()

	s.RegisterFunc("task1", "0 0 0 * * *", func(ctx context.Context) error { return nil })
	s.RegisterFunc("task2", "0 0 0 * * *", func(ctx context.Context) error { return nil })

	tasks := s.Tasks()
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}
}

func TestScheduler_Remove(t *testing.T) {
	s, _ := NewDefault()

	s.RegisterFunc("remove_task", "0 0 0 * * *", func(ctx context.Context) error { return nil })

	err := s.Remove("remove_task")
	if err != nil {
		t.Fatalf("remove failed: %v", err)
	}

	// 再次移除应该失败
	err = s.Remove("remove_task")
	if err == nil {
		t.Fatal("remove non-existent task should fail")
	}
}

func TestScheduler_RemoveStopsCronEntry(t *testing.T) {
	s, _ := NewDefault()

	var count atomic.Int64
	if err := s.EverySecond("remove_running_task", JobFunc(func(ctx context.Context) error {
		count.Add(1)
		return nil
	})); err != nil {
		t.Fatalf("register failed: %v", err)
	}

	s.Start()
	defer s.Stop()

	deadline := time.After(2500 * time.Millisecond)
	for count.Load() == 0 {
		select {
		case <-deadline:
			t.Fatal("task did not run before remove")
		default:
			time.Sleep(20 * time.Millisecond)
		}
	}

	if err := s.Remove("remove_running_task"); err != nil {
		t.Fatalf("remove failed: %v", err)
	}

	afterRemove := count.Load()
	time.Sleep(1500 * time.Millisecond)
	if got := count.Load(); got != afterRemove {
		t.Fatalf("removed task should not run again, before=%d after=%d", afterRemove, got)
	}
}

func TestScheduler_ConvenienceMethods(t *testing.T) {
	s, _ := NewDefault()

	job := JobFunc(func(ctx context.Context) error { return nil })

	// 测试便捷方法
	tests := []struct {
		name string
		fn   func() error
	}{
		{"every_second", func() error { return s.EverySecond("s1", job) }},
		{"every_minute", func() error { return s.EveryMinute("m1", job) }},
		{"every_five", func() error { return s.EveryFiveMinutes("f1", job) }},
		{"hourly", func() error { return s.Hourly("h1", job) }},
		{"daily", func() error { return s.Daily("d1", job) }},
		{"daily_at", func() error { return s.DailyAt("da1", 8, 30, job) }},
		{"weekly", func() error { return s.Weekly("w1", job) }},
		{"monthly", func() error { return s.Monthly("mo1", job) }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.fn(); err != nil {
				t.Fatalf("%s failed: %v", test.name, err)
			}
		})
	}
}

func TestCronExpression(t *testing.T) {
	// 验证预定义表达式
	if CronExpression.EveryMinute != "0 * * * * *" {
		t.Fatalf("unexpected every minute expression: %s", CronExpression.EveryMinute)
	}

	if CronExpression.Daily != "0 0 0 * * *" {
		t.Fatalf("unexpected daily expression: %s", CronExpression.Daily)
	}
}

func TestGlobalScheduler(t *testing.T) {
	// 重置
	defaultScheduler = nil

	// 未初始化时调用应该失败
	err := RegisterFunc("test", "0 0 0 * * *", func(ctx context.Context) error { return nil })
	if err == nil {
		t.Fatal("should fail when not initialized")
	}

	// 初始化
	s, _ := NewDefault()
	Init(s)

	// 现在应该可以注册
	err = RegisterFunc("test", "0 0 0 * * *", func(ctx context.Context) error { return nil })
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}
}

func TestScheduler_StartStop(t *testing.T) {
	s, _ := NewDefault()

	// 注册一个每秒执行的任务
	counter := 0
	s.RegisterFunc("counter", "* * * * * *", func(ctx context.Context) error {
		counter++
		return nil
	})

	// 启动
	s.Start()

	// 等待几秒
	time.Sleep(3 * time.Second)

	// 停止
	s.Stop()

	// 验证任务被执行了
	if counter < 2 {
		t.Fatalf("expected at least 2 executions, got %d", counter)
	}
}

// BenchmarkScheduler_Register 基准测试
func BenchmarkScheduler_Register(b *testing.B) {
	s, _ := NewDefault()
	job := JobFunc(func(ctx context.Context) error { return nil })

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.RegisterFunc(fmt.Sprintf("task_%d", i), "0 0 0 * * *", job)
	}
}
