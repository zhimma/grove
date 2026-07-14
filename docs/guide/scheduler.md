# 计划任务

本文档说明 Grove 中计划任务组件的使用方式。

## 适用范围

`pkg/scheduler` 适用于单进程定时任务，例如：

- 定时清理
- 定时报表
- 定时同步

## 最短路径

### 启用配置

```yaml
scheduler:
  enabled: true
  timezone: Asia/Shanghai
```

也可以使用 `SCHEDULER_ENABLED` 和 `SCHEDULER_TIMEZONE` 覆盖。Scheduler 只由 `worker` 进程承载，API 和 Console 不会自动启动它。

### 注册任务

```go
err := p.Scheduler.EveryMinute("sync_stats", scheduler.JobFunc(func(ctx context.Context) error {
	return syncStats(ctx)
}))
if err != nil {
	return err
}
```

需要单次执行超时时使用完整 Task：

```go
err := p.Scheduler.Register(&scheduler.Task{
    Name:     "sync_stats",
    Schedule: scheduler.CronExpression.EveryMinute,
    Mutex:    true,
    Timeout:  2 * time.Minute,
    Job: scheduler.JobFunc(func(ctx context.Context) error {
        return syncStats(ctx)
    }),
})
```

### 启动任务调度

在 Worker 创建阶段完成任务注册，`WorkerApp.Start` 会启动已启用的 Scheduler。

## 使用约定

- 任务名必须唯一。
- 定时任务应短小、幂等。
- 需要防止同进程重入时，应使用组件提供的互斥能力。
- `Run(name)` 会返回 Job 的原始错误；Mutex 任务重入返回 `scheduler.ErrTaskRunning`。
- 服务停止会取消 root context；Job 必须监听 context，才能及时结束。
- `Stop()` 有最大等待时间并返回错误，无法强制终止忽略 context 的 goroutine。

## 边界

- 多实例部署下的全局互斥不由调度器隐式保证。
- 跨实例去重或分布式锁应由 Redis、数据库或外部协调系统负责。
- 默认只建议一个 Worker 实例启用 Scheduler；多实例启用时必须接受重复执行或由业务显式协调。

## 相关文档

- [队列任务](./queue.md)
- [pkg 基础组件](./pkg-components.md)
