# Grove 升级与交付清单

> 类型：工作清单，保留原路径与任务 ID，便于持续勾选；不是当前架构事实来源。
> 状态：主要开发已落地，部分范围已收敛，外部验收尚未完成。
> 范围：作为整仓 fork / clone 的 Go 单体脚手架，维护开发体验、基础组件、前后端一致性和交付能力。
> 依据：当前源码、配置、测试与实际命令输出；概览见[当前状态](../status.md)，用法见[文档中心](../README.md)。
> 退出条件：本次交付范围的本地检查、对应提交的 CI、隔离环境和浏览器验收都有证据；暂缓项继续明确标注，不冒充完成。

## 1. 状态约定

- `[x]`：该条明确写出的实现范围已经落地；不代表部署验收通过。
- `[ ] 待验收`：代码或脚本存在，还需要真实执行证据。
- `[ ] 部分完成`：原任务只有部分范围实现，拆分列出已完成与剩余部分。
- `[ ] 暂缓`：尚未实现，等待实际使用场景；不能算作已完成。
- 历史单测数量、覆盖率、文件行数、提交数不作为当前基线。测试通过记录须注明执行范围；`SKIP` 不算验收成功。

当前保留三入口单体、显式装配和精确依赖。`pkg/` 是仓库内跨服务基础层，不对外发布，也不得反向依赖 `internal/`；实际边界见[架构](../architecture.md)。

## 2. 已实现范围

### 基础安全与工程能力

- [x] 配置模板凭据留空、服务级校验、JWT 校验、Console 权限缺失拒绝、API 受保护路由授权。依据：[配置加载](../../internal/config/load.go)、[JWT](../../pkg/auth/token.go)、[Console 鉴权](../../app/console/internal/middleware/admin_auth.go)、[API 路由](../../app/api/internal/router/router.go)。
- [x] 存储默认私有、上传校验、统一响应/错误/验证、审计日志契约。用法：[基础组件](../guide/pkg-components.md)、[响应与错误](../04-响应与错误处理规范.md)、[日志](../guide/logging.md)。
- [x] Scheduler panic 隔离、readiness 与资源关闭生命周期。依据：[Scheduler](../../pkg/scheduler/scheduler.go)、[readiness](../../internal/readiness/)、[Provider](../../internal/provider/)。

### Phase 1 / 2：分层与多实例基础（T1–T5）

- [x] **T1**：服务装配位于 `internal/server`，不再把它当作公共 `pkg/server`。
- [x] **T2**：路由元数据统一到实例级 Catalog，删除进程级双轨路径。依据：[路由组件](../../pkg/route/)。
- [x] **T3**：路由注册统一使用 `RegisterXxxRoutes`，清理 `WithDeps` 后缀。依据：[Console router](../../app/console/internal/router/router.go)。
- [x] **T4**：Casbin 定时重新加载策略，关闭时停止后台任务；有传播延迟，不是即时通知。依据：[RBAC](../../pkg/rbac/casbin.go)。
- [x] **T5**：Worker 可注入共享 Redis store，对 `Mutex` 任务争用同名锁；无共享锁时仅进程内互斥。锁不续期，默认 TTL 为 15 分钟，释放是 Get+Delete，不能据此承诺 exactly-once。依据：[WithScheduler](../../internal/provider/provider.go)、[Scheduler](../../pkg/scheduler/scheduler.go)；[使用边界](../guide/scheduler.md#多实例)。

### Phase 5：计划任务后台管理（S1–S7）

- [x] **S1**：`console_scheduled_tasks` 模型与 PostgreSQL/MySQL 迁移；表中只保存调度参数与上次结果。真实 up/down 验收见 R2。
- [x] **S2**：代码内任务注册表；真实任务为清理过期 Console Session。
- [x] **S3**：Worker 补齐数据库行并周期对账，不覆盖已保存的调度设置；执行后写回结果。
- [x] **S4**：手动触发通过数据库请求标记与条件 UPDATE 认领；停用或未注册的任务会给出跳过结果。
- [x] **S5**：Console 列表、编辑调度、启停、执行一次接口，接入权限和 OpenAPI；不提供后台新建任务体。
- [x] **S6**：管理页面复用 ResourcePage，提供编辑、启停、执行和上次结果展示。
- [x] **S7**：[计划任务指南](../guide/scheduler.md)与[新增模块指南](../03-console-新增模块指南.md)已说明接入方式。

证据：[共享模型](../../internal/model/console_scheduled_task.go)、[Worker 注册与对账](../../app/worker/internal/task/)、[Console handler](../../app/console/internal/handler/scheduled_task.go)、[前端页面与测试](../../web/admin-vben/apps/console/src/views/system/scheduled-tasks/)。当前只保存上次执行摘要，没有完整执行历史；代码中已删除的任务行也没有前端“未注册”标记。

### Phase 6：fork 质量治理（A/B/C/D）

- [x] **A1/A2/A4**：清理文档私人路径、生成器读取目标 `go.mod`、提供 [fork 指南](../guide/fork.md)。**A3 已纠正为分类说明**：`users` 是可选保留的终端用户管理能力，`articles` / starter / echo 是示例，不再按“users 必须解耦”开任务。
- [x] **B1/B2/B3**：移除误导性错误值和无调用响应别名，`response.Fail` 接收 `error`。依据：[errx](../../pkg/errx/)、[response](../../pkg/response/)。
- [x] **C1/C2/C3/C4**：明确内部基础层定位、移除未接入的 transaction manager、收敛构造方式、统一 `any`。单一实现用具体类型，真实多实现边界仍保留接口。
- [x] **C5**：系统管理菜单统一归属，共享 ResourcePage 位于 `components/`。依据：[系统路由](../../web/admin-vben/apps/console/src/router/routes/modules/system.ts)、[ResourcePage](../../web/admin-vben/apps/console/src/components/resource-page/)。
- [x] **D1/D2/D3、T10**：密码哈希归入 `pkg/password`，补充 request / route / permission / errx 的行为测试，不按覆盖率机械补测试。
- [x] **D4**：Go/Node 由 mise 固定，pnpm 由 corepack 按前端 `packageManager` 选择；命令入口见 [Makefile](../../Makefile)。

### Phase 7：开发体验与组件完善（G1–G14）

- [x] **G1/G1b**：`make:module --fields` 生成后端迁移、模型、CRUD、测试和 OpenAPI；存在 Console 契约文件时还生成前端 API、页面、路由与契约登记。使用方式见[生成器指南](../03-console-新增模块指南.md#用生成器起步)。
- [x] **G2**：日志按大小轮转、按保留期清理。依据：[logger](../../pkg/logger/logger.go)，配置见[日志配置](../guide/configuration.md#log)。
- [x] **G3**：分页下沉到 `pkg/pagination`，Console service 显式接收分页策略。依据：[pagination](../../pkg/pagination/)、[Console router](../../app/console/internal/router/router.go)。
- [x] **G4 / T8 的数据库夹具部分**：共用 `OpenDB` 与 `CreateCasbinTable`，自动隔离并关闭测试数据库；生成的 service 测试也使用它们。依据：[testkit](../../internal/testkit/testkit.go)、[生成模板](../../cmd/grove/templates.go)。
- [x] **G5/G6**：API 的 handler/service/middleware 移入自身 `internal/`；访问器、构造函数和主要类型命名按 Go 风格收敛。`storage.Manager` 保留原名。
- [x] **G7**：`key:generate` 生成并打印随机密钥，不覆盖配置文件。依据：[CLI](../../cmd/grove/main.go)。
- [x] **G10**：`make dev.api/dev.console/dev.worker` 使用固定版本 Air 热重载。
- [x] **G11 的配置部分**：Compose 提供 PostgreSQL/Redis 与可选 MySQL，Makefile 提供启停入口；真实启动见 R1。依据：[compose.yaml](../../compose.yaml)。
- [x] **G12/G13**：清理前端第三方统计、旧业务地址和假通知；ResourcePage 支持扩展操作与单元格，计划任务页已复用。复杂会话/日志页面保留专用实现。
- [x] **G14**：本地与 CI 使用固定版本的 lint / govulncheck；Go 已升至 1.27.1，lint 配置已迁移到 v2。漏洞结论限定为当次扫描的可达调用链，详见下方记录。

## 3. 部分完成与暂缓项

### T8：测试辅助能力

- [x] 共用数据库夹具和 Casbin 表结构，替换重复建库逻辑；与 G4 为同一交付，不重复开任务。
- [ ] **暂缓**：原计划中的通用模型工厂与 HTTP 测试助手没有实现。现有源码只有 `OpenDB` 与 `CreateCasbinTable`；后续出现重复需求再抽取。

### T9：真实模块接入验证

- [x] 生成器回归在临时仓库生成 Invoice 等模块，并验证后端编译、CRUD、迁移文件规则及前后端契约。依据：[TestMakeModuleOutputPassesTheProjectGates](../../cmd/grove/main_test.go)。
- [ ] **待验收**：在一个实际 fork 项目中完成真实业务模块接入，包括持久化、权限、页面操作与业务规则；记录生成后手动修改的文件、原因和验收结果。

生成器测试不启动 PostgreSQL/MySQL，也不执行生成页面的 Vue typecheck/build 或浏览器操作。因此 T9 原始的真实业务验收仍属部分完成。

### 组件扩展

- [ ] **T6/G9 暂缓：邮件**。当前没有发信组件；`net/mail` 的邮箱格式解析不等于发送邮件。真实发送场景明确后再设计驱动和失败策略。
- [ ] **T7 暂缓：通知**。当前没有站内信/多通道通知服务；已删除的前端假通知不算能力实现。
- [ ] **G8 暂缓：通用接口限流**。当前 `pkg/ratelimit` 提供登录保护，尚非所有接口的 IP/用户配额中间件；等待正式 API 和配额策略。

继续保留已接受的边界：不引入运行时服务容器、Facade、通用 Repository、BaseService、插件或微服务拆分；i18n、多租户等按实际项目需求单独决定。借鉴其他框架的能力和开发体验，实现保持 Go 的显式组合。

## 4. 交付验收清单

以下均未取得本次交付的完整外部证据。勾选时记录提交 SHA、环境、实际命令/操作、结果；缺少前提时保留未勾选。

- [x] **R0：MySQL CI 跳过行为**。已与 PostgreSQL 对齐；在 `CI=true`、无效 Docker socket 下实测失败而非跳过。MySQL 就绪改为 SQL 检查，PostgreSQL 夹具补齐合法生产 CORS。
- [x] **R1：Compose 与镜像**。OrbStack 上以独立项目/随机本机端口验证 Compose 三个依赖健康；三个服务镜像均构建并验证 readiness、UID 10001、只读根文件系统、只读配置挂载、日志/存储可写挂载与 SIGTERM 退出码 0。
- [x] **R2：PostgreSQL/MySQL 生命周期**。2026-09-30 在 OrbStack 上分别运行 `-count=1` 的真实集成测试，完整 up/down、seed、dirty 状态、约束以及冲突回滚拒绝均通过，未跳过。
- [ ] **R3：Redis 与多 Worker（部分通过）**。Redis Store contract 和单 Worker 的页面手动任务执行已在隔离环境通过；队列消费、多 Worker 互斥/超时和跨实例策略刷新仍需按[Staging 清单](../deployment/staging-checklist.md)验收。任务仍须幂等。
- [ ] **R4：浏览器与运行流程（部分通过）**。本机已验证真实登录、主要页面、系统/站点配置保存、手动任务执行，以及三个镜像 readiness/优雅退出；令牌刷新/退出、最小权限角色、上传下载、任务编辑/启停及生产反向代理仍需完整验收。
- [ ] **R5：镜像扫描**。为三个实际镜像保存扫描结果和 digest；当前 CI 配置只有镜像构建，没有自动镜像漏洞扫描任务。
- [ ] **R6：对应提交的 CI**。经用户授权提交和推送后，保存对应提交的 pipeline 结果；配置了 Job 不等于 Job 已执行成功。
- [ ] **R7：真实业务接入**。完成 T9 剩余验收，确认生成器之外还需手写哪些业务逻辑。

### 数据库与 Redis 检查入口

以下命令只针对隔离测试环境；Redis contract 使用专用测试库。

```bash
GROVE_INTEGRATION_DB=postgres go test -tags=integration ./tests/integration -v
GROVE_INTEGRATION_DB=mysql go test -tags=integration ./tests/integration -v
CACHE_REDIS_ADDR=127.0.0.1:6379 CACHE_REDIS_DB=15 \
  go test -tags=integration ./pkg/cache -run '^TestRedisStoreContract$' -v
```

**执行边界**：PostgreSQL/MySQL 都只在本地允许因 Docker 不可用而跳过；CI 下容器不可用会失败。Redis 缺少 `CACHE_REDIS_ADDR` 时仍会跳过，验收须明确指定隔离实例地址并检查实际日志。

## 5. 验证记录与证据边界

### Go 1.27.1 升级验收（2026-09-30）

以下是本会话上一轮 Go 升级的实际执行记录；本次仅整理文档，不将其写成重新运行的结果。

- [x] Go 1.27.1 已安装；`go.mod`、mise、Docker 和两套 CI 已对齐，移除旧 `toolchain` 指令。四个本机产物经 `go version` 确认为 Go 1.27.1。
- [x] golangci-lint 升至 v2.14.0；保留原检查范围，修正新版检查发现的反射常量与冗余嵌入字段访问。业务依赖版本与 `go.sum` 未变。
- [x] `go mod tidy -diff`、`make test`、`go test -race ./...`、`make quality.go.vet quality.go.lint build` 通过；最后一批等价写法修正后另验 `go test -race ./pkg/migrate ./cmd/grove`。
- [x] `make quality.govuln` 通过：0 个代码可达漏洞，仍报告未触达的依赖漏洞，不宣称所有依赖无漏洞。
- [x] `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags=-s ./app/api/cmd ./app/console/cmd ./app/worker/cmd ./cmd/grove` 通过。
- [x] lint 配置校验、Air v1.67.4 的编译与 `-v`、`make docs.check quality.go.fmt`、`git diff --check` 通过。
- [ ] Docker 镜像和真实 PostgreSQL/MySQL/Redis：当时 Docker daemon 未运行，仍待 R1–R3 验收。

上述 Go 命令通过 `mise exec -- env GOTOOLCHAIN=local` 使用仓库固定工具链。版本依据保留为 [Go 发布说明](https://go.dev/doc/go1.27)与 [golangci-lint 迁移指南](https://golangci-lint.run/docs/product/migration-guide/)。

### 前端与历史验证

前一开发阶段记录了前端 typecheck、lint、unit、循环依赖检查、构建及生成模块后的手工门禁通过。这里只保留验证范围，不沿用旧测试数量、覆盖率或“全部完成”的结论。本次 Go 升级与文档整理均未重新执行完整前端门禁；浏览器和真实部署仍待 R4。

### 本次文档整理

- [x] T8/G4 合并完成范围；T9 区分自动生成回归与真实业务验收；T6/G9、T7、G8 标为暂缓。
- [x] 删除过期“当前基线”数字和已修复问题表；已实现内容链接到对应源码与指南。
- [x] 新增状态入口，修正 Scheduler 多实例、生成器测试范围、MySQL `SKIP` 和命令使用说明。
- [x] 文档本地链接与标题锚点、`make docs.check`、`git diff --check` 通过；非 Markdown 差异摘要与本轮开始时一致，保留已有 Go 升级改动。

### 本轮真实运行验证（2026-09-30）

- PostgreSQL / MySQL：仓库真实生命周期测试均通过，含回滚前拒绝冲突、保持数据/索引/迁移状态、显式处理测试数据后完整 down。
- Redis：隔离实例上的 `TestRedisStoreContract` 通过；未连接用户已有数据库。
- 生成器：临时仓库生成 `HTTPClient` 后，后端编译/CRUD/契约检查及前端 typecheck/build 通过。
- 浏览器：生产构建在本机 Chrome 中通过真实登录、管理员/用户/角色/会话/操作日志/登录日志/文章页面，以及系统/站点配置表单保存；任务从页面请求后由真实 Worker 执行成功，无未捕获的页面错误。
- 运行验证暴露并修复两处装配问题：路由 glob 将测试模块打入生产包；Worker 未装配已配置的数据库。均补充针对性回归。
- Go：`make test test.race quality.go.vet quality.go.lint build` 通过。前端：typecheck、lint、unit、循环依赖和构建通过。
- 三个服务镜像：最终串行构建与运行全部通过，包含 UID 10001、只读根文件系统、挂载配置/可写日志与存储、真实数据库/Redis readiness、SIGTERM 退出码 0。首次并行运行时 OrbStack 曾退出（构建码 137），失败尝试未计为通过。
- 本地验证标签为 `grove-{api,console,worker}:structure-review`；最终镜像 ID 分别以 `7b5a798266e6`、`a68e45208423`、`b9106a8da130` 开头。镜像留作本地复核，临时容器、数据卷与网络已清理。
- Dockerfile 的服务参数位于依赖层之后，三个服务共用模块与编译缓存；构建上下文排除本地二进制和临时工具产物。

以上是本机隔离环境的证据；远端 CI、镜像安全扫描、真实业务接入及完整 Staging 仍按 R3–R7 逐项验收。

## 6. 收尾顺序

### 本轮结构优化（2026-09-30，已完成）

- [x] O1：生成器缩写、特殊文件后缀和转换后名称冲突；生成物构建回归。
- [x] O2：PostgreSQL 生产配置夹具、MySQL SQL 就绪与 CI 失败语义；回滚前校验旧约束，不删除冲突数据。
- [x] O3：前端 API 按资源拆分，共用分页类型；生成器同步。
- [x] O4：页面目录按业务域统一，保留路由名与菜单 key；ResourcePage 自定义表单改为显式传入。
- [x] O5：CLI/config 按职责拆文件，transaction 文件准确命名，echo 协议移出通用 job 包。
- [x] O6：规范与结构指南同步；Go/前端门禁、真实数据库与 Redis、三个服务镜像构建和运行验证通过，详见本轮真实运行记录。
- [x] O7（运行验证发现）：路由 glob 排除测试文件，防止 Vitest 进入生产包；Worker 补齐数据库装配，无数据库时不创建对账器，已补回归。真实浏览器已验证任务手动执行成功；镜像验证见 O6。

本轮不包含暂缓组件、远端发布与真实业务接入；已有 Go 升级改动保持，未经授权不提交或推送。

O1–O7、R0–R2 已完成；下一步补齐 R3/R4 剩余的完整环境验收、R5 镜像扫描，并在用户授权提交推送后取得 R6 的 CI 记录；T9/R7 在真实接入项目中验证。暂缓组件不提前填上完成标记，也不据此继续扩大当前交付范围。
