package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisStore Redis缓存实现
type RedisStore struct {
	client *redis.Client
	prefix string
}

// NewRedisStore 创建Redis缓存
func NewRedisStore(client *redis.Client, prefix string) *RedisStore {
	return &RedisStore{
		client: client,
		prefix: prefix,
	}
}

// prefixKey 添加前缀
func (r *RedisStore) prefixKey(key string) string {
	if r.prefix == "" {
		return key
	}
	return r.prefix + ":" + key
}

// Get 获取缓存值
func (r *RedisStore) Get(ctx context.Context, key string) (any, error) {
	data, err := r.GetBytes(ctx, key)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// GetString 获取字符串值
func (r *RedisStore) GetString(ctx context.Context, key string) (string, error) {
	val, err := r.client.Get(ctx, r.prefixKey(key)).Result()
	if err == redis.Nil {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("redis get: %w", err)
	}
	return val, nil
}

// GetBytes 获取字节值
func (r *RedisStore) GetBytes(ctx context.Context, key string) ([]byte, error) {
	val, err := r.client.Get(ctx, r.prefixKey(key)).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("redis get: %w", err)
	}
	return []byte(val), nil
}

// GetInt 获取整数值
func (r *RedisStore) GetInt(ctx context.Context, key string) (int64, error) {
	val, err := r.client.Get(ctx, r.prefixKey(key)).Result()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("redis get: %w", err)
	}

	// 尝试JSON解析
	var num int64
	if err := json.Unmarshal([]byte(val), &num); err != nil {
		return 0, fmt.Errorf("unmarshal int: %w", err)
	}
	return num, nil
}

// GetFloat 获取浮点数值
func (r *RedisStore) GetFloat(ctx context.Context, key string) (float64, error) {
	val, err := r.client.Get(ctx, r.prefixKey(key)).Result()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("redis get: %w", err)
	}

	var num float64
	if err := json.Unmarshal([]byte(val), &num); err != nil {
		return 0, fmt.Errorf("unmarshal float: %w", err)
	}
	return num, nil
}

// GetBool 获取布尔值
func (r *RedisStore) GetBool(ctx context.Context, key string) (bool, error) {
	val, err := r.client.Get(ctx, r.prefixKey(key)).Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("redis get: %w", err)
	}

	var b bool
	if err := json.Unmarshal([]byte(val), &b); err != nil {
		return false, fmt.Errorf("unmarshal bool: %w", err)
	}
	return b, nil
}

// GetJSON 获取并解析JSON
func (r *RedisStore) GetJSON(ctx context.Context, key string, target any) error {
	data, err := r.GetBytes(ctx, key)
	if err != nil {
		return err
	}
	if data == nil {
		return nil
	}
	return json.Unmarshal(data, target)
}

// Put 设置缓存值
func (r *RedisStore) Put(ctx context.Context, key string, value any, seconds int) error {
	return r.PutWithTTL(ctx, key, value, secondsToTTL(seconds))
}

// PutWithTTL 使用time.Duration设置缓存
func (r *RedisStore) PutWithTTL(ctx context.Context, key string, value any, ttl time.Duration) error {
	data, err := encodeValue(value)
	if err != nil {
		return fmt.Errorf("encode value: %w", err)
	}

	prefixedKey := r.prefixKey(key)

	if ttl > 0 {
		err = r.client.Set(ctx, prefixedKey, data, ttl).Err()
	} else {
		err = r.client.Set(ctx, prefixedKey, data, 0).Err()
	}

	if err != nil {
		return fmt.Errorf("redis set: %w", err)
	}
	return nil
}

// PutForever 永久缓存
func (r *RedisStore) PutForever(ctx context.Context, key string, value any) error {
	return r.PutWithTTL(ctx, key, value, 0)
}

// Forever 是 PutForever 的别名
func (r *RedisStore) Forever(ctx context.Context, key string, value any) error {
	return r.PutForever(ctx, key, value)
}

// Increment 原子自增
func (r *RedisStore) Increment(ctx context.Context, key string, value int64) (int64, error) {
	prefixedKey := r.prefixKey(key)

	// 尝试使用INCRBY
	result, err := r.client.IncrBy(ctx, prefixedKey, value).Result()
	if err != nil {
		return 0, fmt.Errorf("redis incrby: %w", err)
	}

	return result, nil
}

// Decrement 原子自减
func (r *RedisStore) Decrement(ctx context.Context, key string, value int64) (int64, error) {
	return r.Increment(ctx, key, -value)
}

// Forget 删除缓存
func (r *RedisStore) Forget(ctx context.Context, key string) error {
	err := r.client.Del(ctx, r.prefixKey(key)).Err()
	if err != nil {
		return fmt.Errorf("redis del: %w", err)
	}
	return nil
}

// Flush 清空缓存（注意：只清空当前前缀的键）
func (r *RedisStore) Flush(ctx context.Context) error {
	// 使用Scan查找所有匹配的键
	pattern := r.prefixKey("*")
	iter := r.client.Scan(ctx, 0, pattern, 0).Iterator()

	var keys []string
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
		if len(keys) >= 100 {
			// 批量删除
			if err := r.client.Del(ctx, keys...).Err(); err != nil {
				return fmt.Errorf("redis del batch: %w", err)
			}
			keys = keys[:0]
		}
	}

	if err := iter.Err(); err != nil {
		return fmt.Errorf("redis scan: %w", err)
	}

	// 删除剩余的键
	if len(keys) > 0 {
		if err := r.client.Del(ctx, keys...).Err(); err != nil {
			return fmt.Errorf("redis del remaining: %w", err)
		}
	}

	return nil
}

// Has 检查是否存在
func (r *RedisStore) Has(ctx context.Context, key string) (bool, error) {
	n, err := r.client.Exists(ctx, r.prefixKey(key)).Result()
	if err != nil {
		return false, fmt.Errorf("redis exists: %w", err)
	}
	return n > 0, nil
}

// TTL 获取剩余时间（秒）
func (r *RedisStore) TTL(ctx context.Context, key string) (int, error) {
	duration, err := r.client.TTL(ctx, r.prefixKey(key)).Result()
	if err != nil {
		return -1, fmt.Errorf("redis ttl: %w", err)
	}

	if duration < 0 {
		// -1 表示永不过期，-2 表示不存在
		return int(duration), nil
	}

	return int(duration.Seconds()), nil
}

// Remember 获取或设置
func (r *RedisStore) Remember(ctx context.Context, key string, seconds int, callback func() (any, error)) (any, error) {
	// 先尝试获取
	value, err := r.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if value != nil {
		return value, nil
	}

	// 执行回调
	result, err := callback()
	if err != nil {
		return nil, err
	}

	// 缓存结果
	if err := r.Put(ctx, key, result, seconds); err != nil {
		// 记录日志但不返回错误
		fmt.Printf("failed to cache remember result: %v\n", err)
	}

	return result, nil
}

// RememberForever 永久缓存版本
func (r *RedisStore) RememberForever(ctx context.Context, key string, callback func() (any, error)) (any, error) {
	return r.Remember(ctx, key, 0, callback)
}

// Add 仅在不存在时设置
func (r *RedisStore) Add(ctx context.Context, key string, value any, seconds int) (bool, error) {
	data, err := encodeValue(value)
	if err != nil {
		return false, fmt.Errorf("encode value: %w", err)
	}

	prefixedKey := r.prefixKey(key)

	// 使用SET NX
	var setErr error
	if seconds > 0 {
		setErr = r.client.SetNX(ctx, prefixedKey, data, secondsToTTL(seconds)).Err()
	} else {
		setErr = r.client.SetNX(ctx, prefixedKey, data, 0).Err()
	}

	if setErr != nil {
		return false, fmt.Errorf("redis setnx: %w", setErr)
	}

	// 检查是否设置成功
	exists, err := r.Has(ctx, key)
	if err != nil {
		return false, err
	}

	return exists, nil
}
