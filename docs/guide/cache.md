# 缓存系统

本文档说明 Grove 中缓存组件的使用方式与边界。

## 适用范围

当前缓存能力由 `pkg/cache` 提供，支持：

- 内存缓存
- Redis 缓存

缓存实例通常由启动或路由装配层从 `internal/provider.Provider` 获取，再通过构造函数传给实际使用它的 service；业务 service 不持有完整 Provider。

## 最短路径

### 配置 Redis

```yaml
redis:
  enabled: true
  addr: 127.0.0.1:6379
  password: ""
  db: 0
```

### 在装配层获取缓存

```go
store, err := p.Cache.Get("memory")
if err != nil {
	return err
}

if err := cache.SetJSON(ctx, store, "dashboard:summary", summary, time.Minute); err != nil {
	return err
}
```

### 读取缓存

```go
value, found, err := cache.GetJSON[DashboardSummary](ctx, store, "dashboard:summary")
if err != nil {
	return err
}
if !found {
	// cache miss
}
_ = value
```

## 关键约定

- 由装配层通过 `p.Cache.Get(name)` 获取缓存实例，并把返回的存储对象传给业务对象。
- `Get(name)` 返回明确错误；不要依赖 `nil` 表示缓存未配置。
- 缓存键命名应带业务前缀，例如 `dashboard:summary`、`user:123`。
- Redis 适合多实例部署；内存缓存只适合单进程本地缓存。
- 框架默认会把 Redis 读写错误返回给调用方，不自动回退到进程内缓存或伪装成业务成功；如果某个业务允许降级，必须在业务服务层显式定义降级数据、告警和一致性边界。
- `TTL` 返回 `found=false` 表示不存在；永不过期的键返回 `ttl=0, found=true`。
- `Add` 是条件写入，不是完整分布式锁；需要锁所有权、续租或隔离令牌（fencing token）时使用独立组件。
- 内存和 Redis 实现提供 `CompareAndDelete`，仅在值仍匹配时原子删除。该能力用于锁的安全释放，不会自动续租；普通 `Store` 接口仍只约定键值缓存操作。

## 常见场景

### 读取后回填

```go
value, err := cache.RememberJSON(ctx, store, "user:123", 5*time.Minute, func(ctx context.Context) (User, error) {
	return s.loadUser(ctx, "123")
})
if err != nil {
	return err
}
_ = value
```

### 删除缓存

```go
if err := store.Delete(ctx, "user:123"); err != nil {
	return err
}
```

## 边界

- Grove 不在框架层提供缓存标签、缓存事件或跨服务缓存协议。
- 缓存只负责键值存取，不承担权限、分页或复杂查询逻辑。
- 分布式缓存一致性由业务自行处理。

## 相关文档

- [配置说明](./configuration.md)
- [pkg 基础组件](./pkg-components.md)
