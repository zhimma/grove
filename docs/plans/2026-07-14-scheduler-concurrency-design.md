# Task 15：Scheduler 并发、配置与取消设计

## Status

Accepted

## Context

当前 Scheduler 注册时保存外部 `*Task` 指针，调用方后续修改会改变已注册任务。`executeTask` 读取 `running` map 时没有持有 Scheduler 锁，测试计数器也存在数据竞争。任务使用 `context.Background()`，服务停止不能取消任务；`Stop` 无超时，遇到不响应取消的任务会永久阻塞。

配置文档已经声明 `scheduler.enabled/timezone`，但配置结构、Provider 选项集和服务启动链均未接入，运行时 `Provider.Scheduler` 始终为 nil。

## Requirements

- 注册时校验 nil task、空 name、空 schedule、nil job 和非法 timeout。
- 注册后复制 Task 配置，外部修改不影响运行时。
- tasks、entries、running 和生命周期状态全部通过同一把锁或原子状态访问。
- Stop 取消所有任务，并在最大等待时间后返回错误。
- Task 可配置单次执行 timeout。
- Scheduler 的配置与真实服务启动链一致。
- 多实例部署不隐式实现分布式互斥。

## Decision

### 1. 内部记录不暴露外部 Task 指针

注册时规范化并复制 `Task`，内部保存 `scheduledTask`。每个任务使用独立原子状态记录 active count 和 Mutex 占用；`IsRunning` 不再通过 `sync.Mutex.TryLock` 推断状态。

`Tasks()` 返回排序后的名称，调用方获得稳定结果。`Remove` 在统一锁内删除 task、entry 和 state，并调用 cron 的真实 `Remove`。

### 2. Scheduler 持有 root context

`New` 创建 root context 和 cancel。每次执行继承 root context；`Task.Timeout > 0` 时再派生 timeout context。Stop 先 cancel root context，再停止 cron。

Mutex 任务通过 compare-and-swap 防止重叠；手动 `Run` 在任务已运行时返回明确的 `ErrTaskRunning`。`Run` 返回任务本身的 error，不再只表示查找成功。

### 3. Stop 有确定上限且可重复等待

Scheduler Config 增加内部使用的 `StopTimeout`，默认 30 秒。首次 Stop 标记 stopped、cancel root context、调用 cron Stop，并等待 cron wrapper 与手动 Run 全部退出。

超过上限返回 `ErrStopTimeout`，不假装任务已经退出；后台等待继续进行，调用方释放阻塞任务后再次 Stop 可得到成功。Stop 幂等，停止后不允许 Start、Register 或 Run。

### 4. 仅 Worker 进程承载 Scheduler

应用配置增加：

```yaml
scheduler:
  enabled: false
  timezone: Local
```

支持 `SCHEDULER_ENABLED` 和 `SCHEDULER_TIMEZONE`。Provider 的 `WithScheduler` 读取配置，并使用 server shutdown timeout 作为 Stop 上限。

Scheduler 只加入 `WorkerOptions`，由 `WorkerApp.Start` 启动。API 和 Console 不自动承载计划任务，避免水平扩容后每个 Web 实例重复执行。需要定时任务时，在 Worker 创建阶段注册后再启动。

## Failure Modes And Mitigations

- Job 忽略 context：Stop 到达上限后返回 `ErrStopTimeout`，进程关闭策略由上层决定。
- Job timeout：Job 收到 deadline context；无法强制杀死 Go goroutine。
- Stop 与 Run/Register 并发：统一生命周期锁先标记 stopped，禁止新的 WaitGroup Add。
- Remove 与运行中任务并发：已开始的执行允许完成或响应取消，后续调度和手动 Run 不可达。
- 多 Worker 实例：仍可能重复执行；文档要求部署层只启用一个 Scheduler 实例或由业务使用外部协调。

## Consequences

### Positive

- 消除 Scheduler 自身和测试中的已知 race。
- 服务停止、任务 timeout 和手动 Run 的 error 语义可验证。
- 配置、Provider 和 Worker 启动链闭环，功能不再只存在于文档。
- 内部状态小而明确，没有引入通用任务容器或分布式锁抽象。

### Negative

- `Start` 和 `Stop` 改为返回 error，调用方需要处理。
- Stop 超时后无法强制终止不合作的 Job。
- Scheduler 停止后不可重启，需要重新创建实例。

## Alternatives Considered

### 只修测试计数器 race

拒绝。生产代码仍存在 map 访问、外部指针、取消和无限 Stop 问题。

### 在 API 和 Console 都自动启用

拒绝。Web 服务通常水平扩容，会在每个实例重复执行相同计划任务。

### 为每个任务启动独立管理 goroutine

暂不采用。cron wrapper、root context、WaitGroup 和原子状态已经覆盖当前需求，额外 supervisor 层不符合 YAGNI。

## Verification

- `go test ./pkg/scheduler -race -count=20`。
- 注册校验、Task 复制、Mutex、IsRunning、timeout、Stop cancel/timeout、Remove 和全局实例测试。
- config 默认值、YAML/env override 和非法 timezone 测试。
- Provider 仅在 enabled 时创建 Scheduler，Worker Start/Stop 启动链测试。
- 全量 test、vet、build 和 diff check。
