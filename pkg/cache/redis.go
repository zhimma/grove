package cache

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisStore struct {
	client *redis.Client
	prefix string
}

func NewRedisStore(client *redis.Client, prefix string) *RedisStore {
	return &RedisStore{
		client: client,
		prefix: strings.Trim(strings.TrimSpace(prefix), ":"),
	}
}

// Prefix exposes the effective namespace for diagnostics and contract tests;
// callers should still use Store keys rather than constructing prefixed keys.
func (r *RedisStore) Prefix() string {
	if r == nil {
		return ""
	}
	return r.prefix
}

func (r *RedisStore) Get(ctx context.Context, key string) ([]byte, bool, error) {
	if err := r.validate(); err != nil {
		return nil, false, err
	}
	ctx = normalizeContext(ctx)
	value, err := r.client.Get(ctx, r.prefixKey(key)).Bytes()
	if err == redis.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("redis get: %w", err)
	}
	return append([]byte(nil), value...), true, nil
}

func (r *RedisStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if err := r.validate(); err != nil {
		return err
	}
	ctx = normalizeContext(ctx)
	if ttl < 0 {
		ttl = 0
	}
	if err := r.client.Set(ctx, r.prefixKey(key), value, ttl).Err(); err != nil {
		return fmt.Errorf("redis set: %w", err)
	}
	return nil
}

func (r *RedisStore) Delete(ctx context.Context, key string) error {
	if err := r.validate(); err != nil {
		return err
	}
	ctx = normalizeContext(ctx)
	if err := r.client.Del(ctx, r.prefixKey(key)).Err(); err != nil {
		return fmt.Errorf("redis delete: %w", err)
	}
	return nil
}

func (r *RedisStore) Add(ctx context.Context, key string, value []byte, ttl time.Duration) (bool, error) {
	if err := r.validate(); err != nil {
		return false, err
	}
	ctx = normalizeContext(ctx)
	if ttl < 0 {
		ttl = 0
	}
	added, err := r.client.SetNX(ctx, r.prefixKey(key), value, ttl).Result()
	if err != nil {
		return false, fmt.Errorf("redis add: %w", err)
	}
	return added, nil
}

func (r *RedisStore) TTL(ctx context.Context, key string) (time.Duration, bool, error) {
	if err := r.validate(); err != nil {
		return 0, false, err
	}
	ctx = normalizeContext(ctx)
	ttl, err := r.client.PTTL(ctx, r.prefixKey(key)).Result()
	if err != nil {
		return 0, false, fmt.Errorf("redis ttl: %w", err)
	}
	switch ttl {
	case -2:
		return 0, false, nil
	case -1:
		return 0, true, nil
	default:
		if ttl <= 0 {
			return time.Nanosecond, true, nil
		}
		return ttl, true, nil
	}
}

func (r *RedisStore) prefixKey(key string) string {
	if r.prefix == "" {
		return key
	}
	return r.prefix + ":" + key
}

func (r *RedisStore) validate() error {
	if r == nil || r.client == nil {
		return fmt.Errorf("redis cache client is not configured")
	}
	return nil
}
