//go:build integration

package cache

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestRedisStoreContract(t *testing.T) {
	addr := os.Getenv("CACHE_REDIS_ADDR")
	if addr == "" {
		t.Skip("CACHE_REDIS_ADDR is not configured")
	}
	db, _ := strconv.Atoi(os.Getenv("CACHE_REDIS_DB"))
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: os.Getenv("CACHE_REDIS_PASSWORD"),
		DB:       db,
	})
	t.Cleanup(func() { _ = client.Close() })
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("ping redis: %v", err)
	}
	prefix := "grove-cache-contract-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	store := NewRedisStore(client, prefix)
	for _, key := range []string{"missing", "permanent", "expiring", "add"} {
		key := key
		t.Cleanup(func() { _ = store.Delete(context.Background(), key) })
	}

	if value, found, err := store.Get(ctx, "missing"); err != nil || found || value != nil {
		t.Fatalf("missing get: value=%v found=%t err=%v", value, found, err)
	}
	if ttl, found, err := store.TTL(ctx, "missing"); err != nil || found || ttl != 0 {
		t.Fatalf("missing ttl: ttl=%v found=%t err=%v", ttl, found, err)
	}
	if err := store.Set(ctx, "permanent", []byte("value"), 0); err != nil {
		t.Fatalf("set permanent: %v", err)
	}
	if ttl, found, err := store.TTL(ctx, "permanent"); err != nil || !found || ttl != 0 {
		t.Fatalf("permanent ttl: ttl=%v found=%t err=%v", ttl, found, err)
	}
	if err := store.Set(ctx, "expiring", []byte("value"), 120*time.Millisecond); err != nil {
		t.Fatalf("set expiring: %v", err)
	}
	if ttl, found, err := store.TTL(ctx, "expiring"); err != nil || !found || ttl <= 0 {
		t.Fatalf("expiring ttl: ttl=%v found=%t err=%v", ttl, found, err)
	}
	time.Sleep(160 * time.Millisecond)
	if _, found, err := store.Get(ctx, "expiring"); err != nil || found {
		t.Fatalf("expired get: found=%t err=%v", found, err)
	}
	if added, err := store.Add(ctx, "add", []byte("first"), 0); err != nil || !added {
		t.Fatalf("add new: added=%t err=%v", added, err)
	}
	if added, err := store.Add(ctx, "add", []byte("second"), 0); err != nil || added {
		t.Fatalf("add existing: added=%t err=%v", added, err)
	}
	if err := store.Delete(ctx, "add"); err != nil {
		t.Fatalf("delete: %v", err)
	}
}
