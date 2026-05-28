# pkg 基础组件

`pkg` 是当前框架的基础组件层，目标是提供稳定、清晰、可复用的能力。它可以借鉴 Laravel 的 Cache、Event、Schedule、Storage 这些使用体验，但在 Go 里仍以显式依赖为主。

## 使用原则

- 业务代码优先通过 `internal/provider.Provider` 获取组件。
- 全局 helper 只作为启动期或简单场景的便捷入口，不作为复杂业务的默认写法。
- 组件日志统一走 `pkg/logger`，底层是 zerolog；日志文案尽量使用中文，字段名保持英文 snake_case。
- 组件错误应尽量返回明确错误，不用 `nil` 表示配置错误。

## Cache

缓存组件由 `cache.Manager` 管理多个 store。

推荐写法：

```go
store, err := p.Cache.Get("memory")
if err != nil {
	return err
}
return store.Put(ctx, "dashboard:summary", summary, 60)
```

兼容写法：

```go
store := p.Cache.Store("memory")
if store == nil {
	return errx.ServiceUnavailable().WithMessage("缓存未配置")
}
```

约定：

- `Get(name)` 返回明确错误。
- `MustStore(name)` 只建议用于启动期快速失败。
- store 名称会归一化为小写并去除前后空格。

## Event

事件组件用于进程内领域事件，不替代队列系统。

推荐写法：

```go
dispatcher := p.Event
dispatcher.ListenFunc("order.created", func(ctx context.Context, event event.Event) error {
	return nil
})
return dispatcher.Dispatch(ctx, OrderCreated{ID: orderID})
```

约定：

- 同步事件用于当前请求内的轻量扩展。
- 异步事件用于进程内异步处理，不保证跨进程可靠投递。
- 需要持久化、重试、削峰时使用 `pkg/job`。
- `Close()` 是幂等的，服务关闭时可以安全调用。

## Scheduler

计划任务组件基于 `robfig/cron`，适合单进程定时任务。

推荐写法：

```go
err := p.Scheduler.EveryMinute("sync_stats", scheduler.JobFunc(func(ctx context.Context) error {
	return syncStats(ctx)
}))
```

约定：

- 任务名必须唯一。
- `Mutex` 可防止同一个进程内的任务重叠执行。
- `Remove(name)` 会真正移除 cron entry，移除后不会再被调度。
- 多实例部署下的全局互斥需要 Redis/DB 锁，本组件不隐式实现。

## Storage

存储组件由 `storage.Manager` 管理多个 disk。

推荐写法：

```go
file, err := p.Storage.SaveUploadedFile(ctx, "local", "avatars", header)
if err != nil {
	return err
}
```

约定：

- disk 名称会归一化为小写并去除前后空格。
- 空 disk 名称表示默认 disk。
- 上传目录会做路径清理，避免 `../` 逃逸。
- S3/ST​S 用于直传场景；普通后台上传优先走 server 模式。

## Database

数据库组件当前以 Postgres 为主驱动，支持默认库和命名资源库。

推荐写法：

```go
db := p.DB.Default()
ordersDB, err := p.DB.Get("orders")
```

约定：

- `Default()` 适合大多数单库业务。
- `Get(name)` 用于明确的多数据源场景。
- 框架层不预设读写分离或区域路由。

## 不照搬 Laravel 的边界

- 不引入隐式容器解析。
- 不把全局 facade 作为业务默认写法。
- 不在组件层隐藏数据库、缓存、队列的失败。
- 不为了“像框架”而增加 Repository、DTO、VO 等额外层。
