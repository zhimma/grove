package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"
)

// Store 缓存存储接口
type Store interface {
	// Get 获取缓存值
	Get(ctx context.Context, key string) (any, error)
	// GetString 获取字符串值
	GetString(ctx context.Context, key string) (string, error)
	// GetBytes 获取字节值
	GetBytes(ctx context.Context, key string) ([]byte, error)
	// GetInt 获取整数值
	GetInt(ctx context.Context, key string) (int64, error)
	// GetFloat 获取浮点数值
	GetFloat(ctx context.Context, key string) (float64, error)
	// GetBool 获取布尔值
	GetBool(ctx context.Context, key string) (bool, error)
	// GetJSON 获取并解析JSON
	GetJSON(ctx context.Context, key string, target any) error

	// Put 设置缓存值
	Put(ctx context.Context, key string, value any, seconds int) error
	// PutWithTTL 使用time.Duration设置缓存
	PutWithTTL(ctx context.Context, key string, value any, ttl time.Duration) error
	// PutForever 永久缓存
	PutForever(ctx context.Context, key string, value any) error

	// Increment 原子自增
	Increment(ctx context.Context, key string, value int64) (int64, error)
	// Decrement 原子自减
	Decrement(ctx context.Context, key string, value int64) (int64, error)

	// Forget 删除缓存
	Forget(ctx context.Context, key string) error
	// Flush 清空缓存
	Flush(ctx context.Context) error

	// Has 检查是否存在
	Has(ctx context.Context, key string) (bool, error)
	// TTL 获取剩余时间（秒）
	TTL(ctx context.Context, key string) (int, error)

	// Remember 获取或设置（缓存未命中时执行回调）
	Remember(ctx context.Context, key string, seconds int, callback func() (any, error)) (any, error)
	// RememberForever 永久缓存版本
	RememberForever(ctx context.Context, key string, callback func() (any, error)) (any, error)

	// Add 仅在不存在时设置（用于分布式锁）
	Add(ctx context.Context, key string, value any, seconds int) (bool, error)
	// Forever 是 PutForever 的别名
	Forever(ctx context.Context, key string, value any) error
}

// Manager 缓存管理器
type Manager struct {
	stores       map[string]Store
	defaultStore string
}

// NewManager 创建缓存管理器
func NewManager() *Manager {
	return &Manager{
		stores:       make(map[string]Store),
		defaultStore: "default",
	}
}

// Register 注册缓存存储
func (m *Manager) Register(name string, store Store) {
	storeName := normalizeStoreName(name)
	if storeName == "" || store == nil {
		return
	}
	m.stores[storeName] = store
}

// Store 获取指定存储
func (m *Manager) Store(name string) Store {
	store, _ := m.Get(name)
	return store
}

// Get 获取指定存储，缺失时返回明确错误。
func (m *Manager) Get(name string) (Store, error) {
	if m == nil {
		return nil, fmt.Errorf("cache manager is nil")
	}
	storeName := normalizeStoreName(name)
	if storeName == "" {
		storeName = normalizeStoreName(m.defaultStore)
	}
	store, ok := m.stores[storeName]
	if !ok || store == nil {
		return nil, fmt.Errorf("cache store %q is not configured", storeName)
	}
	return store, nil
}

// MustStore 获取指定存储，缺失时 panic，适合启动期配置错误快速失败。
func (m *Manager) MustStore(name string) Store {
	store, err := m.Get(name)
	if err != nil {
		panic(err)
	}
	return store
}

// Default 获取默认存储
func (m *Manager) Default() Store {
	return m.Store(m.defaultStore)
}

// SetDefault 设置默认存储名称
func (m *Manager) SetDefault(name string) {
	m.defaultStore = normalizeStoreName(name)
}

// Stores 获取所有存储名称
func (m *Manager) Stores() []string {
	names := make([]string, 0, len(m.stores))
	for name := range m.stores {
		names = append(names, name)
	}
	return names
}

// ==================== 便捷方法（使用默认存储）====================

var defaultManager *Manager

// Init 初始化默认管理器
func Init(manager *Manager) {
	defaultManager = manager
}

// Store 获取指定存储（全局）
func GetStore(name string) Store {
	if defaultManager == nil {
		return nil
	}
	return defaultManager.Store(name)
}

// DefaultStore 获取默认存储（全局）
func DefaultStore() Store {
	if defaultManager == nil {
		return nil
	}
	return defaultManager.Default()
}

// ==================== 辅助函数 ====================

// encodeValue 编码值为字节
func encodeValue(value any) ([]byte, error) {
	switch v := value.(type) {
	case string:
		return []byte(v), nil
	case []byte:
		return v, nil
	case nil:
		return nil, nil
	default:
		// 使用JSON编码
		return json.Marshal(value)
	}
}

// decodeValue 解码字节到目标类型
func decodeValue(data []byte, target any) error {
	if target == nil {
		return nil
	}

	// 直接类型匹配
	targetType := reflect.TypeOf(target)
	if targetType.Kind() != reflect.Ptr {
		return fmt.Errorf("target must be a pointer")
	}

	// 尝试直接赋值
	targetValue := reflect.ValueOf(target)
	elemType := targetType.Elem()

	switch elemType.Kind() {
	case reflect.String:
		targetValue.Elem().SetString(string(data))
		return nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		// 先尝试JSON解析
		var num int64
		if err := json.Unmarshal(data, &num); err != nil {
			return fmt.Errorf("decode int: %w", err)
		}
		targetValue.Elem().SetInt(num)
		return nil
	case reflect.Float32, reflect.Float64:
		var num float64
		if err := json.Unmarshal(data, &num); err != nil {
			return fmt.Errorf("decode float: %w", err)
		}
		targetValue.Elem().SetFloat(num)
		return nil
	case reflect.Bool:
		var b bool
		if err := json.Unmarshal(data, &b); err != nil {
			return fmt.Errorf("decode bool: %w", err)
		}
		targetValue.Elem().SetBool(b)
		return nil
	default:
		// 使用JSON解码
		return json.Unmarshal(data, target)
	}
}

// secondsToTTL 将秒转换为time.Duration
func secondsToTTL(seconds int) time.Duration {
	if seconds <= 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

func normalizeStoreName(name string) string {
	return strings.TrimSpace(strings.ToLower(name))
}
