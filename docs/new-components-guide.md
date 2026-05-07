# 新组件使用指南

本文档保留为“新增基础组件时的接入参考”。当前以真实实现为准，不再展示历史接口。

## 当前推荐接入方式

基础组件优先通过 `internal/provider.Provider` 注入到服务层，不直接在 handler 或全局变量里初始化。

```go
base, cleanup, err := server.NewCoreServer(
    cfg,
    "api",
    cfg.Port,
    provider.WithDatabase(),
    provider.WithRedis(),
    provider.WithStorage(),
)
if err != nil {
    return nil, nil, err
}
defer cleanup()
```

如果某个服务已经有稳定的预设组合，优先使用：

```go
provider.APIOptions()
provider.ConsoleOptions()
provider.WorkerOptions()
```

## HTTP Client

```go
client := httpclient.New().
    BaseURL("https://api.example.com").
    WithHeader("Accept", "application/json").
    WithRetry(3, time.Second)

resp, err := client.Get("/users")
```

## Cache

```go
store, err := p.Cache.Get("memory")
if err != nil {
    return err
}

if err := store.Put(ctx, "user:1", user, 3600); err != nil {
    return err
}
```

## Event

```go
p.Event.ListenFunc("order.created", func(ctx context.Context, event event.Event) error {
    return nil
})

err := p.Event.Dispatch(ctx, OrderCreated{ID: orderID})
```

## Scheduler

```go
err := p.Scheduler.EveryMinute("sync_stats", scheduler.JobFunc(func(ctx context.Context) error {
    return syncStats(ctx)
}))
```

## 接入原则

- 优先注入，不依赖隐式全局容器
- 组件错误应直接返回，不用 `nil` 表示配置异常
- 日志统一走 `pkg/logger`
- 可复用能力尽量沉淀到 `pkg/*`，业务拼装放在 `internal/*`

更完整的用法请继续阅读：

- `docs/guide/pkg-components.md`
- `docs/guide/httpclient.md`
- `docs/guide/cache.md`
- `docs/guide/event.md`
- `docs/guide/scheduler.md`
