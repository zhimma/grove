# 计划任务

本文档说明 Grove 中计划任务组件的使用方式。

## 适用范围

`pkg/scheduler` 在 Worker 中运行定时任务，例如：

- 定时清理
- 定时报表
- 定时同步

多 Worker 时可以注入共享 Redis 锁，具体条件和保证范围见下方[多实例](#多实例)。

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
- 手动执行：登记 `run_requested_at`，后续对账认领并执行；正常情况下等待约一个周期，实际延迟还取决于 Worker 状态和任务执行耗时。多副本用条件 UPDATE 认领同一请求标记。

首次登记后，**重新部署不会覆盖运维改过的调度**——补行只针对表中缺失的任务。

### 注意事项

- 调度表达式在 Console 保存前用 `scheduler.ValidateSchedule` 校验，与运行时使用相同的 cron 表达式解析器。
- 停用的任务不能手动执行；已有待执行请求时不能重复提交。
- 无法执行的手动请求（任务已停用或已从代码中删除）会被清空，状态记为 `skipped` 并记录原因，不会一直显示“待执行”。
- 未配置数据库时，Scheduler 仍运行代码内直接注册的任务，只是无法在后台管理，Worker 启动时会告警。

## 使用约定

- 任务名必须唯一。
- 定时任务应短小、幂等。
- `Mutex: true` 阻止重入：同进程靠原子标记，跨实例靠 Redis 锁。
- `Run(name)` 会返回 Job 的原始错误；Mutex 任务重入返回 `scheduler.ErrTaskRunning`。
- 服务停止会取消 root context；Job 必须监听 context，才能及时结束。
- `Stop()` 有最大等待时间并返回错误，无法强制终止忽略 context 的 goroutine。

## 多实例

`internal/provider.WithScheduler` 在 Redis 客户端和 Redis 缓存 store 可用时注入共享锁。`Mutex: true` 的任务先检查进程内重入，再尝试占用共享锁；没有拿到锁的本次执行会被跳过。

| 共享锁 | Mutex 语义 | 部署约束 |
| --- | --- | --- |
| 已注入共享 Redis store | 锁有效期内协调同名任务的并发执行 | 多 Worker 仍须满足下方边界并经过真实环境验收 |
| 未注入 | 仅进程内互斥 | 按单 Worker 使用 |

Redis 未启用时调度器没有任何跨实例共享状态，多副本会各跑各的——这时只能部署一个 Worker。

锁的边界：

- 锁有效期（TTL）默认 15 分钟，不自动续期；超过有效期仍未结束的任务可能与其他实例重叠执行。
- 释放锁时先读取并比对令牌，再删除锁；这不是原子比较删除，读取与删除之间存在竞争窗口。
- 不设 `Mutex` 的任务不加锁，每个实例都会执行。
- 互斥只约束重叠执行，不记录某个调度时刻是否已经完成；锁释放后另一个实例仍可能取得锁。因此不保证每次调度恰好执行一次，任务须幂等。
- 后台只保存上次执行结果；目前没有完整运行历史，也没有向 Console 暴露“任务行已无代码 handler”的独立标记。

## 相关文档

- [队列任务](./queue.md)
- [pkg 基础组件](./pkg-components.md)
