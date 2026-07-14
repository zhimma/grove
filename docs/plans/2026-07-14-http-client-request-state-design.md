# Task 14：HTTP Client 请求级状态设计

## Status

Accepted

## Context

当前 `Client` 同时保存基础 URL、请求头、查询参数、重试次数和钩子，并通过链式方法原地修改这些字段。共享 Provider 中的 Client 被并发复用时，请求状态会串扰并产生 data race。普通响应和下载会无上限读入内存，multipart 文件也会先完整读入内存；重试等待使用 `time.Sleep`，不能及时响应 context 取消。

## Requirements

- Client 可安全地被多个 goroutine 共享。
- header、query、body、hook、响应限制和 retry 都属于一次 Request。
- GET、HEAD、OPTIONS 可按请求策略重试；非幂等请求必须显式配置 retry policy 或 idempotency key。
- 重试等待和传输必须响应 context 取消。
- 普通响应必须有确定的内存上限，上传和下载必须支持流式 IO。
- 不引入通用中间件容器、Factory 或 Java 风格层次。

## Decision

### 1. Client 只保存不可变默认值

`Client` 只保存 `transport`、`baseURL` 和默认 `timeout`。`BaseURL`、`Timeout`、`WithTransport` 返回新的浅拷贝，不修改原 Client；Transport 由调用方负责并发安全。

默认 Transport 基于标准库 `http.Transport`，显式配置连接池、dial、TLS handshake、response header 和 idle timeout，并保留环境代理支持。

### 2. RequestBuilder 拥有全部请求状态

`NewRequest` 为每次请求创建独立的 header、query、body、hook、retry 和响应大小限制。Builder 可链式修改自身，但不会写入共享 Client。

普通 `Body` 支持字符串、字节、`io.Reader` 和 JSON 值；`JSON`、`Form` 提供显式编码。字节和 JSON body 可重放，任意 `io.Reader` 默认只可消费一次。

### 3. 重试是显式请求策略

`WithRetry(count, delay)` 仅作用于当前 Request。GET、HEAD、OPTIONS 默认属于可安全重试方法；POST、PUT、PATCH、DELETE 只有显式调用 `WithRetry` 或设置 `WithIdempotencyKey` 后才允许重试。

重试 transport error 和 5xx，4xx 不重试。每次重试重新创建 request body；不可重放 body 在需要重试时返回明确错误。backoff 使用 timer 和 context select。

### 4. 普通响应有上限，流式路径不缓冲整个 payload

普通响应默认最多读取 10 MiB，并允许 Request 进一步收紧。超过限制时返回 `ErrResponseTooLarge`，同时保留状态码和响应头。

`DownloadToFile` 使用临时文件加 `io.Copy`，成功后 rename，失败不留下半文件。`Stream` 逐块交给 handler。multipart 使用 `io.Pipe`，文件路径在发送时打开并流式复制，不使用 `os.ReadFile` 或完整 `bytes.Buffer`。

### 5. HTTP 状态与传输错误分离

4xx 返回可检查的 `Response`，不作为 transport error。5xx 在重试耗尽后返回最后一个 `Response` 和错误，避免丢失上游状态和响应体。hook 仅属于当前 Request，并在普通响应读取完成后执行。

## Failure Modes And Mitigations

- context 取消：停止 backoff、请求传输、stream 和 multipart pipe。
- 响应过大：限制读取到 `limit+1`，返回 `ErrResponseTooLarge`。
- 上传文件打开或读取失败：通过 pipe error 传给 transport，请求失败。
- 下载中断：删除临时文件，不覆盖现有目标文件。
- 非可重放 body 触发重试：返回 `ErrBodyNotReplayable`，不发送空 body。

## Consequences

### Positive

- Provider 中的 Client 可安全并发复用，请求状态不串扰。
- retry、幂等性和 context 语义清晰。
- 普通响应、上传和下载有明确的内存边界。
- API 仍保持轻量链式风格，但状态所有权符合 Go 的共享约束。

### Negative

- Client 级 `WithHeader/WithQueryParam/WithRetry/BeforeRequest/AfterResponse` 删除，调用方需迁移到 `NewRequest`。
- 流式 `io.Reader` 请求体不能自动重试，除非调用方改用可重放 body。
- `DownloadToFile` 需要目标目录内创建临时文件。

## Alternatives Considered

### 为 Client 增加 mutex

拒绝。mutex 只能消除 data race，不能阻止并发请求互相覆盖 header、query 和 retry 状态。

### 每次请求自动 Clone Client

拒绝。它保留了错误的状态归属，容易在新增字段时漏复制，并继续鼓励把认证头等动态请求数据放进共享 Client。

### 引入通用 middleware/interceptor pipeline

暂不采用。当前需求用请求级 hook 即可完成，额外抽象不符合 YAGNI。

## Verification

- 并发 RequestBuilder 状态隔离和 `go test -race`。
- safe/unsafe method retry、idempotency key、body replay 和 context 取消。
- 普通响应大小边界、最终 5xx response 保留。
- multipart 文件路径流式读取、下载临时文件清理和 stream handler 错误传播。
- 默认 Transport 关键 timeout 和连接池配置。
