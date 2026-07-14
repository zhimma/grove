# Task 22：Readiness、OpenTelemetry 与安全 CI 设计

## Status

Accepted

## Context

当前所有 HTTP 服务只有 `/health`，它只能证明进程能返回请求，无法区分数据库、Redis 或任务队列不可用。Provider 已持有真实运行依赖，但没有统一的 readiness 检查接口。

项目已有 OpenTelemetry core、SDK 和 `otelhttp` 间接依赖，但没有初始化 tracer/meter provider，也没有 Gin、GORM、Redis 或 Asynq instrumentation。HTTP、数据库和任务执行缺少统一指标，CI 只运行普通 Go test、PostgreSQL integration、build 和前端 typecheck，尚未运行 race、vet、漏洞扫描、前端单测和 Console production build。

## Requirements

- `/health/live` 只证明进程存活，不访问外部依赖。
- `/health/ready` 按当前服务已启用依赖检查所有数据库、Redis 和任务队列后端。
- readiness 失败返回 503，并给出依赖名称和安全的错误摘要，不回显 DSN、密码或 token。
- OpenTelemetry trace 覆盖 Gin、GORM、Redis、HTTP Client 和 Asynq。
- 指标至少覆盖 HTTP 数量/延迟/错误、DB pool、Job 执行数量/延迟/结果。
- metrics 使用标准 Prometheus exposition，trace 支持 OTLP HTTP exporter。
- Observability 关闭时不改变业务语义，关闭顺序纳入 Provider 生命周期。
- CI 增加 race、vet、govulncheck、前端 unit test 和 Console build。
- Dependabot 只创建 Go、pnpm 和 GitHub Actions 更新 PR，不自动合并。

## Decision

### 1. 单一 `internal/observability.Runtime`

Runtime 初始化 OpenTelemetry resource、TracerProvider、MeterProvider、Prometheus exporter 和 W3C TraceContext/Baggage propagator。Provider 最先创建 Runtime，并登记 shutdown closer；其他组件从 Runtime 获取 middleware、hook 或 meter。

默认开启本地 Prometheus `/metrics`。OTLP trace exporter 仅在配置 endpoint 时启用；未配置 endpoint 时仍建立上下文传播和 instrumentation，但不向外发送 span。采样率使用 0 到 1 的显式配置。

### 2. 使用现有组件扩展点接入 trace

- Gin：全局 middleware 提取上游 trace context，创建 server span，记录 method、route、status 和 request duration。
- GORM：注册 create/query/update/delete/row/raw callback，只记录 operation、table、rows 和错误，不记录 SQL 或 bind values。
- Redis：使用 `redis.Hook` 记录 command name、pipeline size、duration 和错误，不记录参数值。
- HTTP Client：Provider 创建的 client 使用 `otelhttp.Transport`，自动注入 trace headers；自定义 transport 仍由调用方显式选择是否包装。
- Asynq：client enqueue 创建 producer span；server middleware 创建 consumer span并记录执行结果。trace context 写入 JSON object 的保留 `_grove_trace` 字段，旧 worker 的 Go JSON decoder会忽略该字段，已有无 trace 任务继续兼容。

### 3. Readiness 由 Provider 暴露真实检查集合

Provider 根据成功初始化的组件构建检查：

- 每个 GORM connection 调用底层 `sql.DB.PingContext`。
- Redis 调用 `PING`。
- Job 启用时复用 Redis 后端探测并单独报告 `queue`，让部署平台能识别任务依赖。

HTTP CoreServer 注册 `/health/live`、`/health/ready` 和兼容 `/health`。Worker 复用同一 health server，在独立 `worker_port` 暴露 readiness 和 Job metrics；`/health` 暂时等价于 live，并在部署文档中标记为兼容入口，新探针必须使用 live/ready。

每个 check 使用短超时并并行执行，ready 只有全部必需依赖成功时返回 200。

### 4. Prometheus 指标由 OTel MeterProvider 导出

Prometheus exporter作为 MeterProvider reader，`/metrics` 使用官方 `promhttp.Handler`。核心 instruments：

- `grove.http.server.requests`
- `grove.http.server.duration`
- `grove.http.server.errors`
- `grove.db.pool.open`、`in_use`、`idle`、`wait_count`
- `grove.job.executions`、`grove.job.duration`

标签只使用低基数值：service、method、route、status、database、task type、result。禁止使用 URL 原始 path、用户 ID、request ID、SQL 或错误文本作为 metric label。

### 5. 安全 CI 与依赖治理

CI 后端顺序增加 route contract、race、vet、govulncheck。govulncheck 使用官方安装命令固定到可升级的 `latest` runner 版本；扫描失败阻断合并。

前端继续使用仓库锁定的 pnpm，增加 `test:unit` 和 `build:console`。Dependabot 分别监控 gomod、pnpm 和 github-actions，每周创建 PR，限制同时打开数量，不配置自动合并。

## Failure Modes And Mitigations

- DB/Redis 卡住：每个 readiness check 有独立短超时，总请求也有上限。
- readiness 暴露内部信息：响应只包含依赖逻辑名和归一化状态，详细错误写服务日志。
- OTLP collector 不可用：batch exporter异步重试，不影响业务请求；shutdown 有超时。
- Worker health listener 或 Asynq server 异常退出：错误进入 WorkerApp error channel，由 main 统一记录并执行正常 shutdown，不在后台 goroutine 调用 Fatal。
- trace label 泄露敏感值：数据库和 Redis instrumentation 不记录 SQL、参数或完整命令。
- metric cardinality 爆炸：HTTP 使用 Gin route pattern，不使用原始 URL；Job 只使用注册 task type。
- OTel 初始化失败：配置明确启用且 exporter 创建失败时启动失败，避免静默无观测。
- 旧队列任务没有 trace 字段：consumer 创建新的 root span，payload 仍正常解析。
- 旧 worker 消费新任务：保留字段位于 JSON object 顶层，旧 struct decoder忽略未知字段。

## Consequences

### Positive

- 部署平台可以正确区分存活和可接流量状态。
- HTTP、DB、Redis、外部 HTTP 和 Job 形成统一 trace 链。
- Prometheus 和 OTLP 使用行业标准接口，无自定义监控协议。
- CI 同时覆盖并发、静态分析、已知漏洞和前端 production 构建。

### Negative

- 增加 OTel OTLP/Prometheus exporter 和 Prometheus client 依赖。
- GORM callback 和 Redis hook 需要随上游版本升级维护兼容测试。
- Asynq trace propagation 只适用于 JSON object payload；非 object payload仍有 producer/consumer span，但不能跨进程关联。

## Alternatives Considered

### 只提供 `/health` 并在内部顺便 Ping 依赖

拒绝。liveness 访问外部依赖会在数据库故障时触发无意义重启，放大故障。

### 使用自定义 JSON 指标接口

拒绝。会增加采集器适配成本，Prometheus exporter 已是标准且维护成本更低。

### 记录完整 SQL、Redis 命令和 URL

拒绝。可能泄露凭据和业务数据，也会制造高基数标签。

### 一次性引入所有 contrib instrumentation

暂不采用。Gin、GORM、Redis 和 Asynq 都能用当前扩展点完成小型 instrumentation；只对 HTTP transport 使用已存在的 `otelhttp`，减少依赖面和黑盒行为。

## Verification

- live 不访问依赖、ready success/failure/timeout 和错误脱敏测试。
- Gin route pattern、status、trace propagation 和 metrics 测试。
- GORM callback 不记录 SQL、DB pool observable gauges 测试。
- Redis hook command-only span 测试。
- HTTP client trace header 注入测试。
- Asynq payload兼容、trace extraction、job result metrics 测试。
- 本机 PostgreSQL readiness integration 测试。
- `go test ./...`、`go test -race ./...`、`go vet ./...`、`govulncheck ./...`、`make build`。
- Console 本地 Vitest、vue-tsc 和 production build。
