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
    Schedule: scheduler.EveryMinuteSchedule,
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
- `Mutex: true` 阻止重入：同进程靠原子标记，跨实例靠 Redis 锁。
- `Run(name)` 会返回 Job 的原始错误；Mutex 任务重入返回 `scheduler.ErrTaskRunning`。
- 服务停止会取消 root context；Job 必须监听 context，才能及时结束。
- `Stop()` 有最大等待时间并返回错误，无法强制终止忽略 context 的 goroutine。

## 多实例

`Mutex: true` 的任务在整个部署内只会有一个实例执行，前提是 **Redis 已启用**：Worker 启动时会把 Redis 缓存 store 作为集群锁传给调度器。

| Redis | Mutex 语义 | 可部署副本数 |
| --- | --- | --- |
| 启用 | 集群级互斥 | 任意 |
| 未启用 | 仅进程内互斥 | 1 |

Redis 未启用时调度器没有任何跨实例共享状态，多副本会各跑各的——这时只能部署一个 Worker。

锁的边界：

- 锁 TTL 默认 15 分钟，用于兜住崩溃的 Worker；超过 TTL 仍未结束的任务，锁会被其他实例接管。
- 释放是「读 token 比对后删除」，不是原子 CAS。任务运行时长接近 TTL 时才需要考虑换 Lua 脚本。
- 不设 `Mutex` 的任务不加锁，每个实例都会执行。

## 相关文档

- [队列任务](./queue.md)
- [pkg 基础组件](./pkg-components.md)
