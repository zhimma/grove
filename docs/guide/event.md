# 事件系统

本文档说明 Grove 中进程内事件系统的使用方式。

## 适用范围

`pkg/event` 适用于：

- 当前进程内的同步事件
- 当前进程内的异步事件

不适用于跨进程可靠投递。需要持久化、重试和削峰时，应使用队列系统。

## 最短路径

### 注册监听者

```go
p.Event.ListenFunc("order.created", func(ctx context.Context, event event.Event) error {
	return nil
})
```

### 分发事件

```go
if err := p.Event.Dispatch(ctx, OrderCreated{ID: orderID}); err != nil {
	return err
}
```

## 使用约定

- 事件名应稳定，例如 `order.created`、`user.disabled`。
- 同步事件适合请求内的轻量扩展。
- 异步事件适合不阻塞主流程的进程内任务。
- 监听者应尽量短小，避免在事件链中堆积大量阻塞逻辑。

## 边界

- 事件系统不代替消息队列。
- 事件顺序只在当前进程和当前 dispatcher 范围内成立。
- 事务提交前发送事件需要谨慎，避免事务回滚后事件已经消费。

## 相关文档

- [队列任务](./queue.md)
- [pkg 基础组件](./pkg-components.md)
