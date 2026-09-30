# 事件系统

`pkg/event` 提供当前进程内的同步和异步事件。它适合业务扩展点，不提供跨进程持久化、自动重试或消息可靠性；这些场景使用 `pkg/job` 或后续 Outbox。

## 注册监听器

```go
if err := p.Event.ListenFunc("order.created", func(ctx context.Context, raw event.Event) error {
    created := raw.(OrderCreated)
    return updateDashboard(ctx, created.OrderID)
}); err != nil {
    return err
}
```

事件名、监听器和处理函数都会校验。`Dispatcher` 关闭后不能再注册监听器。

## 同步分发

```go
if err := p.Event.Dispatch(ctx, OrderCreated{OrderID: orderID}); err != nil {
    return err
}
```

`Dispatch` 按注册顺序执行当前监听器快照。一个监听器失败或触发 panic 不会阻止后续监听器；最终通过 `errors.Join` 返回完整错误集合。

监听器的 panic 会转换为 `*event.ListenerPanicError`：

```go
var panicErr *event.ListenerPanicError
if errors.As(err, &panicErr) {
    logger.Error().Bytes("stack", panicErr.Stack).Msg("event listener panic")
}
```

需要知道当前请求内所有副作用是否成功时，使用同步分发。

## 异步分发

### 等待入队

```go
if err := p.Event.DispatchAsync(ctx, OrderCreated{OrderID: orderID}); err != nil {
    return err
}
```

`DispatchAsync` 会等待有界队列可写；队列持续满时由传入的 `context` 控制等待上限。返回 `nil` 只表示当前进程已经接受事件，不表示监听器执行成功。

### 非阻塞尝试

```go
err := p.Event.TryDispatchAsync(ctx, OrderCreated{OrderID: orderID})
if errors.Is(err, event.ErrQueueFull) {
    // 明确降级、记录指标或改用持久队列
}
```

`TryDispatchAsync` 不等待。队列满返回 `ErrQueueFull`，Dispatcher 已关闭返回 `ErrClosed`，不会静默丢弃。

### Context 语义

异步任务通过 `context.WithoutCancel` 保留请求 ID、追踪信息等上下文值，但不继承 HTTP 请求的取消信号和截止时间。已成功入队的事件不会因为响应结束立即取消。

监听器仍应自行设置外部调用的超时，不能把请求上下文的截止时间当作后台任务期限。

## 异步错误观察

异步监听器的执行错误无法通过入队调用返回。需要指标、告警或测试观察时，创建 `Dispatcher` 时配置 `ErrorHandler`：

```go
dispatcher := event.New(event.Config{
    QueueSize: 500,
    WorkerNum: 8,
    ErrorHandler: func(ctx context.Context, raw event.Event, err error) {
        metrics.EventFailures.Add(ctx, 1)
        logger.Error().Err(err).Str("event", raw.EventName()).Msg("async event failed")
    },
})
```

默认 `ErrorHandler` 写入结构化错误日志。`ErrorHandler` 自身的 panic 会被隔离，不会终止事件处理协程。

## 关闭

```go
if err := dispatcher.Close(); err != nil {
    return err
}
```

`Close` 会：

- 拒绝新的注册和分发；
- 排空已经接受的异步事件；
- 等待正在执行的同步 `Dispatch`；
- 等待所有事件处理协程退出；
- 支持并发和重复调用。

Provider 会在关闭基础设施依赖前关闭事件分发器。

## 使用边界

- 异步队列只存在于当前进程，进程崩溃会丢失未处理事件。
- 同一个事件内的监听器按注册顺序执行，不同事件可由多个协程并行处理。
- 事件对象入队后不得继续修改；`Dispatcher` 复制监听器列表，不复制业务载荷。
- `Close` 只应由应用生命周期调用，不要在监听器或 `ErrorHandler` 内重入关闭同一个 `Dispatcher`。
- 事务提交前不要发送不可撤销事件，避免事务回滚后监听器已经执行。
- 需要持久化、重试、削峰或跨服务投递时使用队列；如需 Outbox，须另行实现。

## 相关文档

- [队列任务](./queue.md)
- [pkg 基础组件](./pkg-components.md)
