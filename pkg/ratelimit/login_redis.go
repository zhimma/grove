package ratelimit

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

var redisLoginAllowScript = redis.NewScript(`
local minute_count = redis.call('INCR', KEYS[1])
if minute_count == 1 then
  redis.call('EXPIRE', KEYS[1], 60)
end
local burst_count = redis.call('INCR', KEYS[2])
if burst_count == 1 then
  redis.call('EXPIRE', KEYS[2], 1)
end
if minute_count > tonumber(ARGV[1]) then
  local ttl = redis.call('TTL', KEYS[1])
  if ttl < 1 then ttl = 1 end
  return {0, ttl}
end
if burst_count > tonumber(ARGV[2]) then
  local ttl = redis.call('TTL', KEYS[2])
  if ttl < 1 then ttl = 1 end
  return {0, ttl}
end
return {1, 0}
`)

var redisLoginFailureScript = redis.NewScript(`
if redis.call('EXISTS', KEYS[2]) == 1 then
  local ttl = redis.call('TTL', KEYS[2])
  if ttl < 1 then ttl = 1 end
  return ttl
end
local failures = redis.call('INCR', KEYS[1])
if failures == 1 then
  redis.call('EXPIRE', KEYS[1], ARGV[2])
end
if failures >= tonumber(ARGV[1]) then
  redis.call('SET', KEYS[2], '1', 'EX', ARGV[2])
  redis.call('DEL', KEYS[1])
  return tonumber(ARGV[2])
end
return 0
`)

type RedisLoginGuard struct {
	client redis.UniversalClient
	cfg    LoginConfig
	prefix string
}

func NewRedisLoginGuard(client redis.UniversalClient, cfg LoginConfig) *RedisLoginGuard {
	return &RedisLoginGuard{
		client: client,
		cfg:    normalizeLoginConfig(cfg),
		prefix: "grove:login:",
	}
}

func (g *RedisLoginGuard) Allow(ctx context.Context, key string) error {
	result, err := redisLoginAllowScript.Run(
		ctx,
		g.client,
		[]string{g.key(key, "minute"), g.key(key, "burst")},
		g.cfg.AttemptsPerMinute,
		g.cfg.Burst,
	).Slice()
	if err != nil {
		return fmt.Errorf("redis login rate limit: %w", err)
	}
	if len(result) != 2 {
		return fmt.Errorf("redis login rate limit: unexpected result")
	}
	allowed, ok := result[0].(int64)
	if !ok {
		return fmt.Errorf("redis login rate limit: unexpected allowed value")
	}
	if allowed == 1 {
		return nil
	}
	retrySeconds, ok := result[1].(int64)
	if !ok || retrySeconds < 1 {
		retrySeconds = 1
	}
	return &LimitError{retryAfter: time.Duration(retrySeconds) * time.Second}
}

func (g *RedisLoginGuard) Locked(ctx context.Context, key string) (time.Duration, error) {
	ttl, err := g.client.TTL(ctx, g.key(key, "lock")).Result()
	if err != nil {
		return 0, fmt.Errorf("redis login lock: %w", err)
	}
	if ttl <= 0 {
		return 0, nil
	}
	return ttl, nil
}

func (g *RedisLoginGuard) Failure(ctx context.Context, key string) (time.Duration, error) {
	lockSeconds := int64(g.cfg.LockDuration / time.Second)
	if lockSeconds < 1 {
		lockSeconds = 1
	}
	retrySeconds, err := redisLoginFailureScript.Run(
		ctx,
		g.client,
		[]string{g.key(key, "failures"), g.key(key, "lock")},
		g.cfg.FailureLimit,
		lockSeconds,
	).Int64()
	if err != nil {
		return 0, fmt.Errorf("redis login failure state: %w", err)
	}
	return time.Duration(retrySeconds) * time.Second, nil
}

func (g *RedisLoginGuard) Reset(ctx context.Context, key string) error {
	if err := g.client.Del(ctx, g.key(key, "failures"), g.key(key, "lock")).Err(); err != nil {
		return fmt.Errorf("redis reset login failure state: %w", err)
	}
	return nil
}

func (g *RedisLoginGuard) key(key, suffix string) string {
	return g.prefix + strings.TrimSpace(key) + ":" + suffix
}
