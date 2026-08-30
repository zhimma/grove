# pkg 基础组件

`pkg` 是当前框架的基础组件层，目标是提供稳定、清晰、可复用的能力。它可以借鉴 Laravel 的 Cache、Event、Schedule、Storage 这些使用体验，但在 Go 里仍以显式依赖为主。

## 使用原则

- `internal/provider.Provider` 只在启动、server 和 router 装配边界获取组件；service、handler、job 只接收实际需要的依赖。
- 下文中的 `p` 代表装配层变量。业务对象应在装配层完成 `p.*` 解析后，通过构造函数接收具体 store、dispatcher、manager 或数据库连接。
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
return cache.SetJSON(ctx, store, "dashboard:summary", summary, time.Minute)
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
- Store 只提供字节、命中状态和 TTL 契约；业务类型使用 `cache.GetJSON/SetJSON/RememberJSON`。

## Event

事件组件用于进程内领域事件，不替代队列系统。

推荐写法：

```go
dispatcher := p.Event
if err := dispatcher.ListenFunc("order.created", func(ctx context.Context, event event.Event) error {
	return nil
}); err != nil {
	return err
}
return dispatcher.Dispatch(ctx, OrderCreated{ID: orderID})
```

约定：

- 同步事件用于当前请求内的轻量扩展。
- `Dispatch` 返回全部 listener error；panic 可通过 `ListenerPanicError` 观察。
- `DispatchAsync` 等待入队或 context 取消，`TryDispatchAsync` 队列满返回 `ErrQueueFull`。
- 异步任务保留 context values，但解除请求 cancel/deadline。
- 异步执行错误通过 Config `ErrorHandler` 上报。
- 需要持久化、重试、削峰时使用 `pkg/job`。
- `Close()` 拒绝新事件并排空已接受事件，Provider 关闭时会调用。

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
- `Timeout` 为单次执行派生 deadline，Stop 会取消所有任务的 root context。
- `Remove(name)` 会真正移除 cron entry，移除后不会再被调度。
- Scheduler 只在 `worker` 进程且 `scheduler.enabled=true` 时创建和启动。
- `Start()`、`Stop()` 和手动 `Run()` 都返回 error，调用方不得忽略关闭超时。
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
- 文件默认私有；local 磁盘只有同时设置 `public: true` 和 `serve_static: true` 才会注册静态路由并返回直链。
- 私有文件通过受保护的 Console 下载接口读取；S3 直链是否公开由对象存储策略决定，Grove 不把 JWT secret 当作本地文件签名密钥。
- 上传策略会限制扩展名、探测后的 MIME 和单文件大小；请求总量由全局 `server.max_body_bytes` 限制，文件以安全 ID 生成实际对象 key。
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
