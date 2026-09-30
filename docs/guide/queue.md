# 队列任务

`pkg/job` 基于 Asynq，通过 Redis 投递、延迟和重试任务，由 Worker 消费。任务名与 payload 放在 `internal/jobtask` 等生产者/消费者共同可见的位置，处理逻辑放在 `app/worker/internal/handler`。

## 启用

```yaml
redis:
  enabled: true
  addr: 127.0.0.1:6379
job:
  enabled: true
  concurrency: 10
  queues:
    default: 5
    critical: 3
    low: 1
```

启用队列必须启用 Redis，再运行 `make run.worker`。只使用 Scheduler 的 Worker 可以不启用队列；多实例调度另有[共享锁要求](scheduler.md#多实例)。

## 投递

在装配层把 `*job.Client` 注入 service，调用时携带 context：

```go
id, err := client.Enqueue(ctx, jobtask.TaskEcho, jobtask.EchoPayload{
    Message: "hello",
}, asynq.Queue("default"), asynq.MaxRetry(3))
if err != nil {
    return err
}
```

`jobtask` 指 `internal/jobtask`，队列选项来自 `github.com/hibiken/asynq`。入队失败应由业务明确处理，不能把尚未成功投递的任务告知客户端为成功。

## 消费

注册示例见 `app/worker/internal/handler/echo.go`：

```go
err := server.Register(jobtask.TaskEcho, func(ctx context.Context, task *asynq.Task) error {
    var payload jobtask.EchoPayload
    if err := job.ParsePayload(task, &payload); err != nil {
        return err
    }
    return handleMessage(ctx, payload.Message)
})
```

任务处理返回 error，由队列按选项决定重试。需要幂等的写入由业务保证；队列不是业务事务的一部分，数据库提交和入队之间的失败窗口需按场景处理。

Payload 保持紧凑，不放长期凭据或完整业务文件。需要同步执行当前请求的扩展逻辑时使用[事件](event.md)，需要按时间触发时使用[计划任务](scheduler.md)。
