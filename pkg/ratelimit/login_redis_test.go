package ratelimit

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestRedisLoginGuardSharesStateAcrossInstances(t *testing.T) {
	addr := os.Getenv("GROVE_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("GROVE_TEST_REDIS_ADDR is not set")
	}
	client := redis.NewClient(&redis.Options{Addr: addr})
	t.Cleanup(func() { _ = client.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("ping redis: %v", err)
	}

	cfg := LoginConfig{
		AttemptsPerMinute: 2,
		Burst:             2,
		FailureLimit:      2,
		LockDuration:      30 * time.Second,
	}
	first := NewRedisLoginGuard(client, cfg)
	second := NewRedisLoginGuard(client, cfg)
	key := LoginKey(fmt.Sprintf("redis-test-%d", time.Now().UnixNano()), "192.0.2.1")
	t.Cleanup(func() {
		_ = client.Del(
			context.Background(),
			first.key(key, "minute"),
			first.key(key, "burst"),
			first.key(key, "failures"),
			first.key(key, "lock"),
		).Err()
	})

	if err := first.Allow(ctx, key); err != nil {
		t.Fatalf("first allow: %v", err)
	}
	if err := second.Allow(ctx, key); err != nil {
		t.Fatalf("second allow through another instance: %v", err)
	}
	if err := first.Allow(ctx, key); RetryAfter(err) <= 0 {
		t.Fatalf("shared rate state should reject third request: %v", err)
	}
	if retry, err := first.Failure(ctx, key); err != nil || retry != 0 {
		t.Fatalf("first failure: retry=%s err=%v", retry, err)
	}
	if retry, err := second.Failure(ctx, key); err != nil || retry <= 0 {
		t.Fatalf("second instance should create shared lock: retry=%s err=%v", retry, err)
	}
	if retry, err := first.Locked(ctx, key); err != nil || retry <= 0 {
		t.Fatalf("first instance should observe shared lock: retry=%s err=%v", retry, err)
	}
}
