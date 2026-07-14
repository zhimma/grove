# Task 16：Event 异步投递语义设计

## Status

Accepted

## Context

当前 `Dispatch` 会记录监听器错误但始终返回 nil，调用方无法判断同步副作用是否成功。`DispatchAsync` 使用 non-blocking send，队列满时静默丢弃并返回 nil；异步任务直接持有请求 context，请求结束后监听器常收到已取消 context。监听器 panic 只写日志，`executeListener` 仍返回 nil。

Dispatcher Close 与入队通过同一把锁规避 send-on-closed-channel，但 API 没有区分“等待入队”和“尽力入队”，错误和背压语义不明确。全局 Dispatcher 也没有并发保护。

## Requirements

- `Dispatch` 同步执行全部监听器，并返回完整错误集合。
- `DispatchAsync` 等待单个事件入队或 context 取消，不静默丢弃。
- `TryDispatchAsync` 非阻塞，队列满返回 `ErrQueueFull`。
- 异步任务保留 context values，但不继承请求取消和 deadline。
- listener panic 转换成可检查错误。
- Close 拒绝新事件，并排空所有已接受事件。
- 保持进程内组件定位，不扩展为持久消息总线。

## Decision

### 1. 同一个 Dispatcher 同时支持同步与异步

移除按实例区分的 `Async` 模式。`Dispatch` 始终同步，`DispatchAsync/TryDispatchAsync` 始终走有界队列。`New` 使用默认 queue/worker 配置，`NewAsync` 保留为显式容量构造的兼容入口。

异步队列的一个 item 代表一次事件分发，并保存分发时的监听器快照。这样一次入队要么接受全部监听器，要么完全失败，不会出现部分监听器已入队、部分因 context 取消丢失。

### 2. 明确背压 API

- `DispatchAsync(ctx, event)`：持有关闭读锁等待队列可写；context 取消时返回 `ctx.Err()`。
- `TryDispatchAsync(ctx, event)`：立即尝试；队列满返回 `ErrQueueFull`。
- Dispatcher 已关闭时，同步和异步分发都返回 `ErrClosed`。

队列容量和 worker 数量仍由 Config 控制，非法非正值使用默认值。

### 3. 异步 context 保留值但解除请求生命周期

入队任务使用 `context.WithoutCancel`。request ID、trace 关联值等仍可读取，但请求取消和 deadline 不会导致已接受事件在后台立即失败。nil context 规范化为 `context.Background()`。

这不创建持久化保证；进程崩溃仍会丢失内存队列。

### 4. 错误聚合与 panic 类型化

同步分发继续执行所有监听器，并通过 `errors.Join` 返回每个监听器错误。panic 转换为 `ListenerPanicError`，包含事件名、recover value 和 stack，可通过 `errors.As` 检查。

异步调用只返回入队错误，执行期错误通过 Config `ErrorHandler` 回调上报。默认 handler 写结构化错误日志；业务需要指标、告警或测试观察时可注入回调。错误 handler 自身 panic 会被隔离并记录，不能杀死 worker。

### 5. Close 是有序生命周期屏障

Dispatcher 使用一把生命周期读写锁协调 Dispatch/入队与 Close：

- 接受事件前在读锁内检查 closed 并登记 active work。
- Close 获取写锁，先标记 closed，再关闭 queue，确保之后没有 send-on-closed-channel。
- worker range queue，完成所有已接受任务。
- Close 等待同步 dispatch、异步任务和 worker 全部退出后返回；并发和重复 Close 等待同一个完成信号。

由于 `event.New()` 现在同时支持异步队列，Provider Close 同步关闭 Event，避免常驻 worker 泄漏；Task 17 再统一为逆序 closers。

## Failure Modes And Mitigations

- 队列满：阻塞 API 等待或响应 context；try API 返回 `ErrQueueFull`。
- 请求已取消：已成功入队任务仍保留 values 并继续执行。
- listener error/panic：同步返回聚合错误；异步进入 ErrorHandler。
- listener 永久阻塞：Close 会等待；进程内 Event 不提供强制终止 goroutine。
- ErrorHandler panic：recover 后记录，不影响 worker 继续消费。
- Close 与并发入队：写锁等待已进入的 send 完成或 context 取消，然后关闭队列。

## Consequences

### Positive

- 调用方可以明确选择背压或 best-effort try，不再静默丢事件。
- 同步事件错误可参与事务或业务流程判断。
- 异步事件不会因 HTTP 请求结束立即取消，同时保留追踪值。
- Close 成为可验证的排空屏障。

### Negative

- `Dispatch` 过去被忽略的 listener error 现在会返回，调用方必须正确处理。
- 默认 Dispatcher 创建固定数量 worker；Provider 必须关闭 Event。
- ErrorHandler 是进程内观测机制，不提供持久失败队列或重试。

## Alternatives Considered

### 队列满继续记录日志并丢弃

拒绝。调用方无法区分成功与丢失，且错误日志不能构成投递契约。

### 每个 listener 单独入队

拒绝。一次事件可能部分成功入队，context 取消后状态不可解释。

### DispatchAsync 返回 Future/Receipt

暂不采用。当前需求只需要明确入队和集中错误观察；Future 会扩大 API 和调用复杂度。需要等待处理结果时应使用同步 Dispatch。

### 直接引入持久消息队列

拒绝。`pkg/event` 定位是进程内扩展点；可靠跨进程投递继续使用 `pkg/job` 或后续 Outbox。

## Verification

- 同步多监听器错误聚合、panic error 和继续执行测试。
- blocking enqueue context 取消、try queue full 和关闭拒绝测试。
- `WithoutCancel` 保留 value、解除 deadline/cancel 测试。
- async ErrorHandler 接收 listener error/panic 测试。
- Close 排空、并发 Close、全局 Dispatcher race 测试。
- `go test -race ./pkg/event -count=20`、全量 test/race/vet/build。
