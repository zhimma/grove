package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/zhimma/grove/pkg/logger"
)

// MemoryStore 内存缓存实现
type MemoryStore struct {
	data   map[string]*memoryItem
	mutex  sync.RWMutex
	stopCh chan struct{}
}

// memoryItem 内存缓存项
type memoryItem struct {
	value      []byte
	expiration int64 // UnixNano，0表示永不过期
}

// IsExpired 检查是否过期
func (item *memoryItem) IsExpired() bool {
	if item.expiration == 0 {
		return false
	}
	return time.Now().UnixNano() > item.expiration
}

// NewMemoryStore 创建内存缓存
func NewMemoryStore() *MemoryStore {
	store := &MemoryStore{
		data:   make(map[string]*memoryItem),
		stopCh: make(chan struct{}),
	}

	// 启动清理协程
	go store.gc()

	return store
}

// Close 关闭内存缓存
func (m *MemoryStore) Close() {
	close(m.stopCh)
}

// gc 定期清理过期项
func (m *MemoryStore) gc() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.cleanup()
		case <-m.stopCh:
			return
		}
	}
}

// cleanup 清理过期项
func (m *MemoryStore) cleanup() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	now := time.Now().UnixNano()
	for key, item := range m.data {
		if item.expiration > 0 && now > item.expiration {
			delete(m.data, key)
		}
	}
}

// Get 获取缓存值
func (m *MemoryStore) Get(ctx context.Context, key string) (any, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	item, exists := m.data[key]
	if !exists || item.IsExpired() {
		return nil, nil
	}

	return item.value, nil
}

// GetString 获取字符串值
func (m *MemoryStore) GetString(ctx context.Context, key string) (string, error) {
	data, err := m.GetBytes(ctx, key)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// GetBytes 获取字节值
func (m *MemoryStore) GetBytes(ctx context.Context, key string) ([]byte, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	item, exists := m.data[key]
	if !exists || item.IsExpired() {
		return nil, nil
	}

	// 返回副本
	result := make([]byte, len(item.value))
	copy(result, item.value)
	return result, nil
}

// GetInt 获取整数值
func (m *MemoryStore) GetInt(ctx context.Context, key string) (int64, error) {
	data, err := m.GetBytes(ctx, key)
	if err != nil {
		return 0, err
	}
	if data == nil {
		return 0, nil
	}

	var num int64
	if err := json.Unmarshal(data, &num); err != nil {
		return 0, fmt.Errorf("unmarshal int: %w", err)
	}
	return num, nil
}

// GetFloat 获取浮点数值
func (m *MemoryStore) GetFloat(ctx context.Context, key string) (float64, error) {
	data, err := m.GetBytes(ctx, key)
	if err != nil {
		return 0, err
	}
	if data == nil {
		return 0, nil
	}

	var num float64
	if err := json.Unmarshal(data, &num); err != nil {
		return 0, fmt.Errorf("unmarshal float: %w", err)
	}
	return num, nil
}

// GetBool 获取布尔值
func (m *MemoryStore) GetBool(ctx context.Context, key string) (bool, error) {
	data, err := m.GetBytes(ctx, key)
	if err != nil {
		return false, err
	}
	if data == nil {
		return false, nil
	}

	var b bool
	if err := json.Unmarshal(data, &b); err != nil {
		return false, fmt.Errorf("unmarshal bool: %w", err)
	}
	return b, nil
}

// GetJSON 获取并解析JSON
func (m *MemoryStore) GetJSON(ctx context.Context, key string, target any) error {
	data, err := m.GetBytes(ctx, key)
	if err != nil {
		return err
	}
	if data == nil {
		return nil
	}

	return json.Unmarshal(data, target)
}

// Put 设置缓存值
func (m *MemoryStore) Put(ctx context.Context, key string, value any, seconds int) error {
	return m.PutWithTTL(ctx, key, value, secondsToTTL(seconds))
}

// PutWithTTL 使用time.Duration设置缓存
func (m *MemoryStore) PutWithTTL(ctx context.Context, key string, value any, ttl time.Duration) error {
	data, err := encodeValue(value)
	if err != nil {
		return fmt.Errorf("encode value: %w", err)
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	expiration := int64(0)
	if ttl > 0 {
		expiration = time.Now().Add(ttl).UnixNano()
	}

	m.data[key] = &memoryItem{
		value:      data,
		expiration: expiration,
	}

	return nil
}

// PutForever 永久缓存
func (m *MemoryStore) PutForever(ctx context.Context, key string, value any) error {
	return m.PutWithTTL(ctx, key, value, 0)
}

// Forever 是 PutForever 的别名
func (m *MemoryStore) Forever(ctx context.Context, key string, value any) error {
	return m.PutForever(ctx, key, value)
}

// Increment 原子自增
func (m *MemoryStore) Increment(ctx context.Context, key string, value int64) (int64, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	item, exists := m.data[key]
	if !exists || item.IsExpired() {
		// 新建
		newValue := value
		data, _ := json.Marshal(newValue)
		m.data[key] = &memoryItem{
			value:      data,
			expiration: 0,
		}
		return newValue, nil
	}

	// 解析当前值
	var current int64
	if err := json.Unmarshal(item.value, &current); err != nil {
		return 0, fmt.Errorf("unmarshal current value: %w", err)
	}

	newValue := current + value
	data, _ := json.Marshal(newValue)
	item.value = data

	return newValue, nil
}

// Decrement 原子自减
func (m *MemoryStore) Decrement(ctx context.Context, key string, value int64) (int64, error) {
	return m.Increment(ctx, key, -value)
}

// Forget 删除缓存
func (m *MemoryStore) Forget(ctx context.Context, key string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	delete(m.data, key)
	return nil
}

// Flush 清空缓存
func (m *MemoryStore) Flush(ctx context.Context) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.data = make(map[string]*memoryItem)
	logger.Info().Msg("内存缓存已清空")
	return nil
}

// Has 检查是否存在
func (m *MemoryStore) Has(ctx context.Context, key string) (bool, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	item, exists := m.data[key]
	if !exists || item.IsExpired() {
		return false, nil
	}
	return true, nil
}

// TTL 获取剩余时间（秒）
func (m *MemoryStore) TTL(ctx context.Context, key string) (int, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	item, exists := m.data[key]
	if !exists || item.IsExpired() {
		return -1, nil
	}

	if item.expiration == 0 {
		return -1, nil // 永不过期
	}

	remaining := item.expiration - time.Now().UnixNano()
	if remaining <= 0 {
		return -1, nil
	}

	return int(remaining / int64(time.Second)), nil
}

// Remember 获取或设置
func (m *MemoryStore) Remember(ctx context.Context, key string, seconds int, callback func() (any, error)) (any, error) {
	// 先尝试获取
	value, err := m.Get(ctx, key)
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
	if err := m.Put(ctx, key, result, seconds); err != nil {
		logger.Warn().Err(err).Str("key", key).Msg("缓存 remember 结果写入失败")
	}

	return result, nil
}

// RememberForever 永久缓存版本
func (m *MemoryStore) RememberForever(ctx context.Context, key string, callback func() (any, error)) (any, error) {
	return m.Remember(ctx, key, 0, callback)
}

// Add 仅在不存在时设置
func (m *MemoryStore) Add(ctx context.Context, key string, value any, seconds int) (bool, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 检查是否存在且未过期
	item, exists := m.data[key]
	if exists && !item.IsExpired() {
		return false, nil
	}

	// 设置新值
	data, err := encodeValue(value)
	if err != nil {
		return false, fmt.Errorf("encode value: %w", err)
	}

	expiration := int64(0)
	if seconds > 0 {
		expiration = time.Now().Add(secondsToTTL(seconds)).UnixNano()
	}

	m.data[key] = &memoryItem{
		value:      data,
		expiration: expiration,
	}

	return true, nil
}
