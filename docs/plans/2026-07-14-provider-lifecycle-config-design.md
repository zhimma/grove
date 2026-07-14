# Task 17：Provider 生命周期与服务配置设计

## Status

Accepted

## Context

配置加载当前使用 `yaml.Unmarshal`，未知字段和拼写错误会被静默忽略。`Validate(service)` 基本不使用 service 参数，production Console 在数据库或 console Casbin 未配置时仍能启动，最终由路由返回 503。Worker 的 Job 与 Scheduler 都关闭时仍可创建空进程并等待信号。

Provider Close 通过固定字段顺序手工关闭组件，新增 Option 容易漏接生命周期，也不一定与创建顺序相反。Logger 打开的文件没有保存和关闭路径。HTTP Server Start 先启动后台 goroutine再返回；端口绑定失败不会从 Start 返回，而是在后台 `Fatal` 终止进程。

## Requirements

- YAML 未知字段、拼写错误和多文档输入必须加载失败。
- `Validate(service)` 检查 api、console、worker 的真实运行依赖。
- production Console 缺少数据库或 console 权限执行器时启动前失败。
- Worker 没有启用 Job 或 Scheduler 时明确失败，不运行空进程。
- Provider 组件创建成功后立即登记 closer，Close 按创建逆序执行且幂等。
- Logger、Cache、Event、Scheduler、Job、Redis、Database 都有关闭路径。
- HTTP Start 同步完成 bind，监听失败直接返回；后台 Serve 错误可观察且不调用 Fatal。

## Decision

### 1. 严格 YAML Decoder

配置主体使用 `yaml.NewDecoder` 并启用 `KnownFields(true)`。只允许一个 YAML document；第二个 document 直接报错。env 展开和 dotenv 顺序保持不变，严格校验发生在展开后的最终文本上。

`config.example.yaml` 测试会从模板提取所有环境变量并显式清空，再通过真实 `LoadWithOptions` 加载，证明默认值、引号和特殊字符在干净环境可用。

### 2. 按服务验证真实依赖

公共校验继续覆盖端口、JWT、CORS、Storage、Redis/Job 和 timezone。

- `api`：允许数据库、Redis、Job 和 Casbin 按需关闭；启用的 Casbin 必须引用已启用数据库。
- `console`：development/test 保留轻量启动能力；production 必须启用 default database 和 `casbin.enforcers.console`，且 enforcer 引用已启用数据库。
- `worker`：Job 与 Scheduler 至少启用一个；Job 仍要求 Redis。
- 未知非空 service 名直接返回错误，避免拼写导致校验旁路。

已启用 PostgreSQL 配置校验 driver、host、port、user 和 dbname，避免直到 GORM open 才暴露明显空配置。

### 3. Provider 使用逆序 Closer 栈

Provider 增加私有 closer 栈。Logger 初始化成功后首先登记；每个 Option 在资源创建成功并赋值后立即登记关闭函数。Close 使用 `sync.Once`，从栈尾向前执行，并用组件名包装后通过 `errors.Join` 返回全部错误。

预期创建/关闭示例：

```text
create: logger -> database -> redis -> job -> cache -> event
close:  event -> cache -> job -> redis -> database -> logger
```

Option 初始化失败时调用同一个 Close，已创建资源自动逆序回滚，不再维护另一套清理分支。

### 4. Logger 管理 Writer 生命周期

Logger 保存当前 managed writer。文件 writer 支持并发 Write/Close；Init 原子替换 logger 后关闭旧 writer，Close 恢复 stdout logger 并关闭当前文件。Logger 访问通过读写锁复制 zerolog Logger，消除 Init 与日志读取的数据竞争。

Provider 将 `logger.Close` 登记为第一个 closer，因此最后关闭，保证其他组件停止日志都已完成。

### 5. HTTP Start 先 Bind 再 Serve

`CoreServer.Start` 先调用 `net.Listen`。端口占用、地址非法等 bind 错误同步返回，成功后才启动 Serve goroutine。后台 Serve 错误写入只读 error channel并记录普通 error 日志，不调用 Fatal。

API/Console main 同时等待 OS signal 和 Serve error。`CoreServer.Stop` 先 Shutdown HTTP，再关闭 Provider，并聚合错误；外部 cleanup 仍可调用，因为 Provider Close 幂等。

### 6. Worker 禁止空运行

Config worker 校验在 Job/Scheduler 都关闭时失败。`WorkerApp.NewServer` 再做运行时防御，直接返回 `ErrWorkerDisabled`，覆盖测试或手工构造绕过 Load 的情况。

## Failure Modes And Mitigations

- 配置拼写错误：KnownFields 在启动前返回字段路径错误。
- Option 中途失败：Provider 逆序回滚已经登记的资源。
- 多个 closer 失败：全部执行，最终返回 errors.Join。
- Logger 重初始化：旧 managed writer 在替换后关闭，不遗留文件描述符。
- HTTP bind 失败：Start 不创建 Serve goroutine，调用方可打印错误并退出。
- Serve 运行时失败：Errors channel 唤醒 main，执行正常 Stop/Close。
- Worker 空配置：Load 或 NewServer 明确失败，不进入信号等待。

## Consequences

### Positive

- 配置错误从运行期 503/空进程前移到启动期。
- 新组件只需在创建处登记 closer，不再修改集中式字段关闭列表。
- HTTP 启动成功意味着端口已经绑定成功。
- Logger 文件描述符与全局状态有明确生命周期。

### Negative

- 旧配置中的未知字段会变成启动错误，需要修正拼写或删除废弃字段。
- production Console 的最小配置要求提高，必须显式提供数据库和权限配置。
- Start 仍采用后台 Serve，因此运行期错误通过 Errors channel而不是 Start 返回。

## Alternatives Considered

### 继续手工维护 Provider.Close 字段列表

拒绝。创建和关闭逻辑分离，新增 Option 很容易漏清理或顺序错误。

### 让 HTTP Start 阻塞运行

拒绝。现有 main 需要同时处理 signal；同步 bind + 后台 Serve error channel 能保留当前启动模型并修复错误可见性。

### production Console 缺配置时继续返回 503

拒绝。这会让错误部署看似健康，直到用户访问特定路由才暴露。

### Worker 空配置时仅打印 warning

拒绝。空进程持续占用部署资源且 readiness 含义错误。

## Verification

- KnownFields、未知嵌套字段、多 document、干净环境 config.example 测试。
- api/console/worker 服务矩阵、production Console DB/Casbin、worker 空配置测试。
- Provider closer 逆序、错误聚合、初始化失败回滚、幂等 Close 测试。
- Logger 文件写入/关闭和 race 测试。
- HTTP bind 失败、成功监听、Serve error channel、大小写 production Gin mode 测试。
- Worker disabled NewServer 测试。
- 全量 test/race/vet/build。
