# 缓存系统

本文档说明 Grove 中缓存组件的使用方式与边界。

## 适用范围

当前缓存能力由 `pkg/cache` 提供，支持：

- 内存缓存
- Redis 缓存

缓存实例通常通过 `internal/provider.Provider` 获取。

## 最短路径

### 配置 Redis

```yaml
redis:
  enabled: true
  addr: 127.0.0.1:6379
  password: ""
  db: 0
```

### 通过 Provider 获取缓存

```go
store, err := p.Cache.Get("memory")
if err != nil {
	return err
}

if err := store.Put(ctx, "dashboard:summary", summary, 60); err != nil {
	return err
}
```

### 读取缓存

```go
value, err := store.Get(ctx, "dashboard:summary")
if err != nil {
	return err
}
_ = value
```

## 关键约定

- 业务代码优先通过 `p.Cache.Get(name)` 获取缓存实例。
- `Get(name)` 返回明确错误；不要依赖 `nil` 表示缓存未配置。
- 缓存键命名应带业务前缀，例如 `dashboard:summary`、`user:123`。
- Redis 适合多实例部署；内存缓存只适合单进程本地缓存。
- 缓存失败不应伪装成业务成功，应按场景决定降级或直接返回错误。

## 常见场景

### 读取后回填

```go
value, err := store.Remember(ctx, "user:123", 300, func() (any, error) {
	return s.loadUser(ctx, "123")
})
if err != nil {
	return err
}
_ = value
```

### 删除缓存

```go
if err := store.Forget(ctx, "user:123"); err != nil {
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
