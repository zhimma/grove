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

## 后台管理

需要在后台调整执行时机的任务，注册到 `app/worker/internal/task` 的注册表，而不是直接调 `Scheduler.Register`。

```go
// app/worker/internal/task/registry.go
defined := []Definition{
	{
		Name:        "console.purge-expired-sessions",
		DisplayName: "清理过期后台会话",
		Schedule:    "0 17 3 * * *", // 首次出现时的默认值
		Mutex:       true,
		Timeout:     5 * time.Minute,
		Job:         scheduler.JobFunc(newPurgeExpiredSessions(dbs).Run),
	},
}
```

Worker 会为每个定义在 `console_scheduled_tasks` 补一行，此后按表里的值调度。

### 代码与数据库各管什么

| 归属 | 内容 |
| --- | --- |
| 代码 | 任务名 → handler 函数 |
| 数据库 | 调度表达式、启停、互斥、超时、上次执行结果 |
| Console | 改表达式、启停、手动执行一次、看上次结果 |

**Console 不能新建任务。** 行由 Worker 按代码注册表补齐；表里没有任何字段能存放任务内容，因此后台改不出一个新任务来执行。表里若残留代码中已删除的任务，Worker 每轮会告警并跳过。

### 生效时机

Worker 每 30 秒与表对账一次，所以：

- 改调度表达式、启停：下一轮对账生效。
- 手动执行：登记 `run_requested_at`，下一轮对账被认领并执行，因此点击后最多等一个周期。多副本下用条件 UPDATE 抢占，只有一个实例会执行。

首次登记后，**重新部署不会覆盖运维改过的调度**——补行只针对表中缺失的任务。

### 注意事项

- 调度表达式在 Console 保存前用 `scheduler.ValidateSchedule` 校验，与运行中的 cron 是同一个解析器，因此存得进就跑得了。
- 停用的任务不能手动执行；已有待执行请求时不能重复提交。
- 跑不了的手动请求（任务已停用、代码中已删除）会被清空并记为 `skipped` + 原因，不会一直显示"待执行"。
- 未配置数据库时，Scheduler 仍运行代码内直接注册的任务，只是无法在后台管理，Worker 启动时会告警。

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
