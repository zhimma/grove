# Task 13：Cache 契约与生命周期设计

## Status

Accepted

## Context

当前 Cache Store 同时暴露 `any`、字符串、数字、JSON、Remember、Increment、Flush 等大量方法，不同后端对未命中和 TTL 的返回值不一致。`Remember` 在未命中时返回 callback 原类型，命中后返回 `[]byte`，调用方无法依赖稳定类型。MemoryStore 启动 GC goroutine，但 Provider 关闭时没有关闭 Cache Manager。

## Decision

### 1. Store 只保留稳定字节契约

```go
type Store interface {
    Get(ctx context.Context, key string) ([]byte, bool, error)
    Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
    Delete(ctx context.Context, key string) error
    Add(ctx context.Context, key string, value []byte, ttl time.Duration) (bool, error)
    TTL(ctx context.Context, key string) (time.Duration, bool, error)
}
```

- `found=false` 唯一表示 key 不存在或已过期。
- 永久 key 返回 `ttl=0, found=true`。
- 有过期时间的 key 返回正 duration 和 `found=true`。
- `ttl<=0` 表示永久保存。
- `Get` 和 `Set` 均复制 byte slice，调用方不能修改 Store 内部值。
- `Delete` 对不存在 key 幂等成功。
- `Add` 只保证当前 Store 的条件写入语义，不命名为锁，也不承诺续租、所有权或 fencing token。

### 2. 类型转换使用包级泛型函数

提供：

```go
SetJSON[T](ctx, store, key, value, ttl)
GetJSON[T](ctx, store, key) (T, bool, error)
RememberJSON[T](ctx, store, key, ttl, loader) (T, error)
```

- JSON 编解码失败直接返回错误。
- loader 成功但缓存写入失败也返回错误，不隐藏基础设施故障。
- `RememberJSON` 在 singleflight 回调内二次检查缓存，抑制同进程同 store、同 key、同类型的击穿。
- 等待 singleflight 的调用方可通过 context 提前退出。

### 3. Memory 与 Redis 统一 TTL

- Memory 过期项在 Get/TTL/Add 时惰性删除，后台 GC 只负责回收长期不访问的过期项。
- Redis 使用 `PTTL`：`-2` 映射为 `found=false`，`-1` 映射为永久 key。
- RedisStore 不关闭共享 Redis client；client 生命周期仍由 Provider 管理。

### 4. Manager 负责 Store 生命周期

- Manager 的注册表增加读写锁。
- `Close()` 调用实现 `Close() error` 的 Store，并聚合错误。
- MemoryStore `Close()` 等待 GC goroutine 退出，且可重复调用。
- Provider 先关闭 Cache Manager，再关闭共享 Redis client 和数据库。

## Consequences

### Positive

- Memory/Redis 未命中和永久 TTL 语义一致。
- Store 接口小且稳定，业务类型不会污染基础存储契约。
- Remember 命中前后始终返回同一泛型类型。
- 服务关闭后不残留 Memory GC goroutine。

### Negative

- 旧的 `Put/GetString/Remember/Increment/Flush` API 会删除，需要调用方迁移；当前仓库没有业务调用点。
- `RememberJSON` 只抑制当前进程击穿，不替代分布式锁。

## Verification

- MemoryStore 契约、byte slice 隔离、TTL、Add、context 和 Close 测试。
- fake Store 验证 JSON 编解码、错误传播和并发 loader 只执行一次。
- 本机临时 Redis 进程验证与 Memory 相同的 missing/permanent/expiring/Add/Delete 语义。
- Provider Close 测试确认 Cache Manager 被调用。
