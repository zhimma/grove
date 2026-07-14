package ratelimit

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestNewLoginGuardUsesLocalGuardForNilRedisClient(t *testing.T) {
	var client *redis.Client
	guard := NewLoginGuard(LoginConfig{}, client)
	if _, ok := guard.(*LocalLoginGuard); !ok {
		t.Fatalf("nil Redis client must use local guard, got %T", guard)
	}
}

func TestLoginKeyNormalizesAccountAndIP(t *testing.T) {
	first := LoginKey(" Admin@Example.COM ", "2001:0db8:0:0:0:0:0:1")
	second := LoginKey("admin@example.com", "2001:db8::1")
	if first != second {
		t.Fatalf("expected normalized keys to match: %q != %q", first, second)
	}
	if strings.Contains(first, "admin") || strings.Contains(first, "2001:db8") {
		t.Fatalf("login key must not expose account or IP: %q", first)
	}
	if first == LoginKey("other@example.com", "2001:db8::1") {
		t.Fatal("different accounts must use isolated keys")
	}
	if first == LoginKey("admin@example.com", "192.0.2.1") {
		t.Fatal("different client IPs must use isolated keys")
	}
}

func TestLocalLoginGuardRateLimit(t *testing.T) {
	guard := NewLocalLoginGuard(LoginConfig{
		AttemptsPerMinute: 60,
		Burst:             2,
		FailureLimit:      3,
		LockDuration:      time.Minute,
	})
	ctx := context.Background()
	key := LoginKey("admin", "192.0.2.1")

	if err := guard.Allow(ctx, key); err != nil {
		t.Fatalf("first request should pass: %v", err)
	}
	if err := guard.Allow(ctx, key); err != nil {
		t.Fatalf("second request should pass: %v", err)
	}
	err := guard.Allow(ctx, key)
	if err == nil {
		t.Fatal("third immediate request should be rate limited")
	}
	if retry := RetryAfter(err); retry <= 0 {
		t.Fatalf("rate limit error must include retry-after, got %s", retry)
	}
}

func TestLocalLoginGuardLocksAndResetsFailures(t *testing.T) {
	guard := NewLocalLoginGuard(LoginConfig{
		AttemptsPerMinute: 60,
		Burst:             5,
		FailureLimit:      2,
		LockDuration:      time.Minute,
	})
	ctx := context.Background()
	key := LoginKey("admin", "192.0.2.1")

	if retry, err := guard.Failure(ctx, key); err != nil || retry != 0 {
		t.Fatalf("first failure must not lock: retry=%s err=%v", retry, err)
	}
	if retry, err := guard.Failure(ctx, key); err != nil || retry <= 0 {
		t.Fatalf("second failure must lock: retry=%s err=%v", retry, err)
	}
	if retry, err := guard.Locked(ctx, key); err != nil || retry <= 0 {
		t.Fatalf("expected active lock: retry=%s err=%v", retry, err)
	}
	if err := guard.Reset(ctx, key); err != nil {
		t.Fatalf("reset failures: %v", err)
	}
	if retry, err := guard.Locked(ctx, key); err != nil || retry != 0 {
		t.Fatalf("reset must clear lock: retry=%s err=%v", retry, err)
	}
}

func TestLocalLoginGuardFailureStateIsIsolated(t *testing.T) {
	guard := NewLocalLoginGuard(LoginConfig{
		AttemptsPerMinute: 60,
		Burst:             5,
		FailureLimit:      1,
		LockDuration:      time.Minute,
	})
	ctx := context.Background()
	lockedKey := LoginKey("admin", "192.0.2.1")
	otherIP := LoginKey("admin", "192.0.2.2")
	otherAccount := LoginKey("other", "192.0.2.1")

	if retry, err := guard.Failure(ctx, lockedKey); err != nil || retry <= 0 {
		t.Fatalf("expected first key to lock: retry=%s err=%v", retry, err)
	}
	for _, key := range []string{otherIP, otherAccount} {
		if retry, err := guard.Locked(ctx, key); err != nil || retry != 0 {
			t.Fatalf("unrelated key must remain unlocked: retry=%s err=%v", retry, err)
		}
	}
}

func TestLocalLoginGuardConcurrentFailures(t *testing.T) {
	guard := NewLocalLoginGuard(LoginConfig{
		AttemptsPerMinute: 600,
		Burst:             100,
		FailureLimit:      5,
		LockDuration:      time.Minute,
	})
	ctx := context.Background()
	key := LoginKey("admin", "192.0.2.1")

	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = guard.Failure(ctx, key)
		}()
	}
	wg.Wait()

	if retry, err := guard.Locked(ctx, key); err != nil || retry <= 0 {
		t.Fatalf("concurrent failures must leave key locked: retry=%s err=%v", retry, err)
	}
}
