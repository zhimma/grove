package cache

import (
	"context"
	"testing"
	"time"
)

func TestMemoryStore(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	ctx := context.Background()

	t.Run("Put and Get", func(t *testing.T) {
		err := store.Put(ctx, "key1", "value1", 60)
		if err != nil {
			t.Fatalf("put failed: %v", err)
		}

		val, err := store.GetString(ctx, "key1")
		if err != nil {
			t.Fatalf("get failed: %v", err)
		}
		if val != "value1" {
			t.Fatalf("expected value1, got %s", val)
		}
	})

	t.Run("Expiration", func(t *testing.T) {
		err := store.Put(ctx, "key2", "value2", 1) // 1秒过期
		if err != nil {
			t.Fatalf("put failed: %v", err)
		}

		// 立即获取应该存在
		exists, _ := store.Has(ctx, "key2")
		if !exists {
			t.Fatal("key should exist immediately")
		}

		// 等待过期
		time.Sleep(2 * time.Second)

		exists, _ = store.Has(ctx, "key2")
		if exists {
			t.Fatal("key should be expired")
		}
	})

	t.Run("JSON", func(t *testing.T) {
		type User struct {
			Name  string `json:"name"`
			Email string `json:"email"`
		}

		user := User{Name: "John", Email: "john@example.com"}
		err := store.Put(ctx, "user", user, 60)
		if err != nil {
			t.Fatalf("put failed: %v", err)
		}

		var retrieved User
		err = store.GetJSON(ctx, "user", &retrieved)
		if err != nil {
			t.Fatalf("get json failed: %v", err)
		}

		if retrieved.Name != "John" || retrieved.Email != "john@example.com" {
			t.Fatalf("unexpected value: %+v", retrieved)
		}
	})

	t.Run("Increment", func(t *testing.T) {
		// 初始值
		val, err := store.Increment(ctx, "counter", 1)
		if err != nil {
			t.Fatalf("increment failed: %v", err)
		}
		if val != 1 {
			t.Fatalf("expected 1, got %d", val)
		}

		// 再次自增
		val, err = store.Increment(ctx, "counter", 5)
		if err != nil {
			t.Fatalf("increment failed: %v", err)
		}
		if val != 6 {
			t.Fatalf("expected 6, got %d", val)
		}
	})

	t.Run("Remember", func(t *testing.T) {
		callCount := 0
		callback := func() (any, error) {
			callCount++
			return "computed", nil
		}

		// 第一次调用
		val, err := store.Remember(ctx, "remember_key", 60, callback)
		if err != nil {
			t.Fatalf("remember failed: %v", err)
		}
		if val != "computed" {
			t.Fatalf("expected computed, got %v", val)
		}
		if callCount != 1 {
			t.Fatalf("callback should be called once, got %d", callCount)
		}

		// 第二次调用（应该从缓存获取）
		val, err = store.Remember(ctx, "remember_key", 60, callback)
		if err != nil {
			t.Fatalf("remember failed: %v", err)
		}
		if callCount != 1 {
			t.Fatalf("callback should not be called again, got %d", callCount)
		}
	})

	t.Run("Add", func(t *testing.T) {
		// 第一次添加应该成功
		added, err := store.Add(ctx, "add_key", "value", 60)
		if err != nil {
			t.Fatalf("add failed: %v", err)
		}
		if !added {
			t.Fatal("add should return true for new key")
		}

		// 第二次添加应该失败
		added, err = store.Add(ctx, "add_key", "new_value", 60)
		if err != nil {
			t.Fatalf("add failed: %v", err)
		}
		if added {
			t.Fatal("add should return false for existing key")
		}

		// 值应该保持不变
		val, _ := store.GetString(ctx, "add_key")
		if val != "value" {
			t.Fatalf("value should not change, got %s", val)
		}
	})

	t.Run("Flush", func(t *testing.T) {
		store.Put(ctx, "flush1", "value1", 60)
		store.Put(ctx, "flush2", "value2", 60)

		err := store.Flush(ctx)
		if err != nil {
			t.Fatalf("flush failed: %v", err)
		}

		exists1, _ := store.Has(ctx, "flush1")
		exists2, _ := store.Has(ctx, "flush2")

		if exists1 || exists2 {
			t.Fatal("all keys should be flushed")
		}
	})
}

func TestManager(t *testing.T) {
	manager := NewManager()

	// 注册内存存储
	memoryStore := NewMemoryStore()
	defer memoryStore.Close()

	manager.Register("memory", memoryStore)
	manager.SetDefault("memory")

	// 测试获取
	store := manager.Store("memory")
	if store == nil {
		t.Fatal("should get memory store")
	}

	defaultStore := manager.Default()
	if defaultStore != store {
		t.Fatal("default should be memory store")
	}

	// 测试全局初始化
	Init(manager)

	if DefaultStore() == nil {
		t.Fatal("global default should not be nil")
	}
}

func TestMemoryStoreCloseIsIdempotent(t *testing.T) {
	store := NewMemoryStore()

	store.Close()
	store.Close()
}

func TestManagerGetReturnsExplicitErrors(t *testing.T) {
	manager := NewManager()

	if _, err := manager.Get("missing"); err == nil {
		t.Fatal("expected missing store error")
	}

	memoryStore := NewMemoryStore()
	defer memoryStore.Close()

	manager.Register(" Memory ", memoryStore)
	store, err := manager.Get("memory")
	if err != nil {
		t.Fatalf("get normalized store failed: %v", err)
	}
	if store != memoryStore {
		t.Fatal("expected registered memory store")
	}

	manager.SetDefault(" MEMORY ")
	defaultStore, err := manager.Get("")
	if err != nil {
		t.Fatalf("get default store failed: %v", err)
	}
	if defaultStore != memoryStore {
		t.Fatal("expected normalized default store")
	}
}

func TestManagerMustStorePanicsWhenMissing(t *testing.T) {
	manager := NewManager()

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for missing cache store")
		}
	}()

	_ = manager.MustStore("missing")
}

func BenchmarkMemoryStore_Get(b *testing.B) {
	store := NewMemoryStore()
	defer store.Close()
	ctx := context.Background()

	store.Put(ctx, "key", "value", 3600)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.Get(ctx, "key")
	}
}

func BenchmarkMemoryStore_Put(b *testing.B) {
	store := NewMemoryStore()
	defer store.Close()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.Put(ctx, "key", "value", 3600)
	}
}
