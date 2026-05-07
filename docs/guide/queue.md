# 队列任务

本文档说明 Grove 中基于 Asynq 的队列任务能力。

## 适用范围

队列系统适用于：

- 异步任务投递
- 延迟任务
- 重试任务
- worker 独立消费

## 最短路径

### 启用配置

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

### 投递任务

```go
task, err := job.NewTask("echo", map[string]any{"message": "hello"})
if err != nil {
	return err
}

_, err = p.JobClient.Enqueue(task)
if err != nil {
	return err
}
```

### worker 处理任务

任务处理逻辑放在 `app/worker/handler`，并由 worker 服务启动时注册。

## 使用约定

- 任务名应稳定，统一放在 `pkg/job/tasks.go` 或对应任务定义位置。
- 主流程是否忽略入队失败，应由业务明确决定。
- 需要重试、延迟或指定队列时，应显式传入任务选项。
- 未启用 Redis 时不应启动 worker。

## 边界

- 队列系统不负责业务补偿策略。
- 任务 payload 应保持紧凑，避免传入过大对象。
- 需要幂等的任务必须由业务层自行保证幂等。

## 相关文档

- [计划任务](./scheduler.md)
- [事件系统](./event.md)
