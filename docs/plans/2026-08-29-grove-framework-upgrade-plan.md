
# Grove Web 框架组件与工程规范升级 Implementation Plan

> **For Claude:** Use an executing-plans workflow to implement this plan task-by-task.

**Goal:** 在不破坏现有 api / console / worker 模块化单体边界的前提下，把 Grove 从“已有较完整脚手架”升级为可持续开发的 Go Web 框架基线：补齐日志、请求校验、文件上传、统一返回和错误处理等常用组件，修复当前安全与契约断点，统一目录、命名、代码编写和抽象规则，并建立可验证的分阶段升级路线。

**Architecture:** 保留显式组合和模块化单体。启动入口负责配置、资源创建和生命周期；handler 只做 HTTP 适配；service 负责业务流程、事务和副作用；model 负责共享模型与查询辅助；pkg 只放已经证明可复用的基础能力。借鉴 Laravel 的开发体验和能力分类，不复制 Facade、运行时容器、隐式魔法、通用 Repository 或继承式领域层。

**Tech Stack:** Go 1.25.12、Gin、GORM、PostgreSQL/MySQL、Redis、Casbin、Asynq、robfig/cron、OpenTelemetry、Vue 3、Vben Admin、pnpm。

---

## 0. 文档状态、范围与结论口径

- 审查日期：2026-08-30；本轮实施记录同步更新于 2026-08-30。
- 覆盖范围：根目录 Go 服务、cmd/grove、internal、pkg、database、web/admin-vben、配置、Makefile、CI 和现有文档。
- 事实来源：当前 checkout 的源码、配置模板、测试、命令输出和 Git 状态。旧的 foundation roadmap 和 design 文档只作为历史背景；若与当前代码不一致，以当前代码为准。
- 本文是一份“当前审查汇总 + 目标规范 + 实施记录”；代码改动保留在当前工作树，未提交 Git commit。计划中的 Deferred 项仍按真实业务触发条件管理。
- “缺失多少”按能力域统计，不按 Laravel 的类、Facade 或包数量逐项仿制。下表把 30 个常用能力域分成：8 个已有可用基础、16 个已有实现但契约/安全/一致性不足、4 个通用能力尚缺、2 个按真实业务触发后再做。这是一种工程成熟度分类，不是产品功能承诺。

### 当前一句话判断

Grove 已经具备可运行的 Go 管理后台脚手架和不少基础组件，短板不是“从零开始缺少框架”，而是组件之间的契约还没有完全闭合：认证、权限、日志、上传、返回协议、路由清单、Provider 装配和前后端 OpenAPI 存在漂移或安全边界缺口。先完成 P0/P1 加固，再做结构收敛，最后按业务触发扩展邮件、Webhook、广播等能力。

### 推荐的总决策

1. **继续使用模块化单体**，暂不拆微服务，也不做插件系统。
2. **保留 Go 的显式依赖**：新代码传入精确依赖，Provider 只在启动和装配边界出现。
3. **把常用组件做成小而稳定的契约**，而不是把所有功能塞进一个“大框架对象”。
4. **先修现有实现和文档漂移**，再迁移目录；不做一次性全仓重命名。
5. **每个能力都要有失败语义、超时、关闭、日志脱敏和测试**，否则不算“框架组件完成”。

---

## 1. 当前项目地图与运行链路

~~~text
config.yaml
    │
    ▼
app/{api,console,worker}/cmd
    │  config.Load + internal/provider 装配
    ▼
CoreServer → 通用 middleware → router → handler → service → model/GORM
    │                                               │
    ├─ API      :8080                                ├─ PostgreSQL / MySQL
    ├─ Console  :8081                                ├─ Redis / Cache
    └─ Worker   :8082                                ├─ JWT / Casbin / Storage
                                                    ├─ Asynq / Scheduler / Event
                                                    └─ Readiness / Metrics / Trace
~~~

### 目录职责（当前结构仍然保留）

| 目录 | 当前职责 | 本次决策 |
| --- | --- | --- |
| app/api | 对外 API 示例、公开/受保护路由 | 保留；新业务按领域拆 handler/service |
| app/console | 管理后台后端、认证、RBAC、系统配置、上传、日志 | 保留；优先修复契约漂移 |
| app/worker | Asynq 消费者和 Scheduler | 保留；明确 panic、超时和多实例边界 |
| cmd/grove | 迁移、seed、生成器、检查命令 | 保留；补 doctor 和安全生成语义 |
| internal | 只能仓库内复用的配置、装配、模型和基础设施 | 保留；缩小 Provider 可见面 |
| pkg | 可复用基础组件 | 只放稳定且与业务无关的能力 |
| database | migrations 与 seeds | 保留；加强方言、碰撞和回滚安全 |
| web/admin-vben | 管理后台前端 monorepo | 保留；以 OpenAPI/contract 消除漂移 |

### 已核对的规模与工程现状

- Go 文件约 188 个、约 3.2 万行；_test.go 约 68 个（以 2026-08-30 checkout 统计为准）。
- go.mod 模块为 github.com/zhimma/grove，代码要求 Go 1.25；仓库配置要求使用 Go 1.25.12。
- 当前 Git 工作树在审查开始时干净；最近提交集中在 foundation roadmap，未发现 tag。
- 远端是 GitLab；已新增 `.gitlab-ci.yml` 作为 canonical CI，`.github/workflows/ci.yml` 保留同等基础门禁作为镜像，远端实际 pipeline 仍需在 GitLab 上触发确认。
- 没有 .codegraph/，本次按源码和调用方做定向核对。

### 本轮验证口径

本轮代码改动后的本地验证记录如下：

~~~bash
go test ./...
go test -race ./...
go vet ./...
go mod verify
make build
CI=true pnpm --dir web/admin-vben install --frozen-lockfile
CI=true pnpm --dir web/admin-vben lint
CI=true pnpm --dir web/admin-vben check:circular
CI=true pnpm --dir web/admin-vben --filter @grove/console typecheck
CI=true pnpm --dir web/admin-vben test:unit
CI=true pnpm --dir web/admin-vben check:console-api-contract
CI=true pnpm --dir web/admin-vben build:console
golangci-lint run
gofmt -l $(rg --files -g '*.go' -g '!web/**' | sort)
git diff --check
~~~

结果：后端测试、race、vet、构建、模块完整性、前端 lint、0 条循环依赖、类型检查、322 个前端单元测试、前端合同测试、前端构建和 golangci-lint 均通过。`govulncheck` 本机未安装；Docker daemon 不可用，Testcontainers/真实 PostgreSQL、MySQL、Redis 未在本地执行。线上数据库迁移、真实 OAuth/对象存储、浏览器 E2E、生产 TLS/反向代理仍需 staging/生产环境验收。

---

## 2. Laravel 风格常用能力盘点

Laravel 的价值在于“能力有明确入口、约定一致、开发者容易发现”。Grove 应吸收这种体验，但将实现映射为 Go 的显式组件。状态含义：

- **✅ 已具备**：当前可用，后续只需维护。
- **△ 需补强**：已有代码，但存在安全、契约、并发、可观测性或文档问题；进入本路线图。
- **✗ 缺失**：通用 Web 项目常见，但当前没有可复用基线。
- **⏸ 暂缓**：不是框架底座，只有真实业务触发才做。

| # | 能力域 | 状态 | 当前实现/证据 | 升级决策 |
| ---: | --- | :---: | --- | --- |
| 1 | HTTP Server、路由、中间件、健康端点 | ✅ | pkg/server、pkg/route、internal/middleware、三个 app 入口 | 保留；路由清单改为实例级 |
| 2 | 配置加载与环境覆盖 | △ | internal/config 支持 YAML 和环境展开，但文档写法与实现不完全一致，模板存在固定凭据风险 | P0 清理模板并统一来源说明 |
| 3 | 结构化日志、访问日志、审计日志 | △ | pkg/logger 可用；Console 日志接口与前端调用不一致，审计 query 可能留存敏感值 | P0/P1 统一契约、脱敏和保留策略 |
| 4 | 统一成功响应、错误映射、panic 出口 | △ | pkg/response、pkg/errx 和文档已有；缺少跨入口 contract 强制，错误码仍需收敛 | P1 建立唯一出口和 contract tests |
| 5 | Request/Query/URI/JSON 参数校验 | ✅ | pkg/validation 提供绑定和字段错误映射 | 保留；补未知字段、body 上限和跨服务使用规范 |
| 6 | Request ID、身份和上下文元数据 | ✅ | pkg/request 与通用中间件已存在 | 保留；统一日志、响应和异步任务传播 |
| 7 | JWT access/refresh、签发和解析 | △ | pkg/auth 已实现 HMAC JWT；解析未严格固定 issuer/algorithm，API 未拒绝 Console token | P0 固化算法/issuer/audience 和 user type |
| 8 | 数据库 Session、Cookie、CSRF/浏览器策略 | △ | Console Session 已持久化；前端 token 存储策略和 SecureLS 密钥配置不完整 | P1 明确 SPA token 或安全 Cookie 单一策略 |
| 9 | RBAC、策略同步、路由权限 | △ | Casbin 和 Console 权限中间件存在；API 路由未实际挂 PermissionSet.Require，多实例无 watcher，非生产有 fail-open | P0/P1 修复挂载、失效和 fail-closed |
| 10 | GORM、连接池、多数据库资源 | ✅ | pkg/database、internal/model 支持 PostgreSQL/MySQL 资源 | 保留；补 DSN 转义、连接超时、方言观测 |
| 11 | Migration、seed、回滚 | ✅ | pkg/migrate、database/migrations、database/seeds、CLI 已有 | 保留；修复文件名碰撞和失败清理 |
| 12 | Cache（memory/Redis）、命名空间、TTL | △ | pkg/cache 与 Provider 默认 store 已有；跨服务前缀只取 app name，配置分页等键未完全落地 | P1 明确 namespace、降级和指标 |
| 13 | Storage、文件上传、公开/私有下载 | △ | pkg/storage 支持 local/S3/STS；Console 将所有 local disk 静态暴露，缺少 public/private 标志 | P0 先封闭静态目录，再定义上传/下载契约 |
| 14 | Queue、重试、消费者、关闭 | ✅ | Asynq、worker、任务注册和生命周期已有 | 保留；补失败任务运维和 dead-letter 视图 |
| 15 | Scheduler、互斥、超时、panic 恢复 | △ | pkg/scheduler 基于 cron；未配置 cron.WithChain(cron.Recover(...))，Mutex 仅进程内 | P0 补恢复；分布式锁按需实现 |
| 16 | 进程内 Event、异步投递 | ✅ | pkg/event 已有同步/异步语义和关闭流程 | 保留；不替代持久化 outbox |
| 17 | HTTP Client、超时、重试、SSRF 策略 | △ | pkg/httpclient 已有请求状态设计；仍需统一超时、重试、幂等和出站限制 | P1 收敛契约 |
| 18 | Metrics、Trace、Readiness、DB 观测 | △ | internal/observability、internal/readiness、OTel 已接入；MySQL 被硬编码成 PostgreSQL，readiness 可能泄漏 goroutine | P1 修正标签、取消和端点策略 |
| 19 | CLI、迁移、seed、模块生成器 | ✅ | cmd/grove 已替代旧命令并提供生成器 | 保留；补 doctor、原子写入和命令 contract |
| 20 | OpenAPI、前端类型/客户端生成 | △ | Console contract 文档存在，但前端日志接口已漂移，未形成单一生成源 | P1 选定 OpenAPI source + 生成/契约检查 |
| 21 | 管理后台页面、权限菜单、状态管理 | △ | Vben Admin 可构建；生产 API、旧 alias、SecureLS、模板元数据和后端接口有漂移 | P1 先清漂移，再做体验增强 |
| 22 | 单元、集成、race、lint、漏洞和 E2E | △ | Go/前端单测和 race 基线好；lint、循环依赖、govulncheck、真实容器验证不完整 | P1 建立 canonical CI 门禁 |
| 23 | Mail、Notification、模板和渠道 | ✗ | 当前无通用发送/重试/模板契约 | 只有出现邮件/短信/站内信业务才立项 |
| 24 | WebSocket、Broadcast、实时订阅 | ✗ | 当前无通用连接管理和授权协议 | 由真实实时场景触发，暂不预埋 |
| 25 | Webhook 签名、重放防护、投递记录 | ✗ | 当前无统一 inbound/outbound webhook 组件 | 由外部回调场景触发 |
| 26 | 幂等键、Transactional Outbox、死信运维 | ✗ | Queue/Event 可用，但没有跨事务可靠投递基线 | 发生可靠性需求后单独设计，不在 P0 重写 Event |
| 27 | Rate limit、分布式锁、跨实例协调 | △ | 登录保护和进程内 Mutex 已有；通用跨实例锁没有 | 先记录边界；有多实例任务再接 Redis/DB adapter |
| 28 | 文件病毒扫描、图片处理、断点/分片上传 | ⏸ | 普通上传可用，但没有业务无关的处理流水线 | 有大文件/合规要求再做 |
| 29 | 部署制品、secret manager、环境检查 | △ | 配置和前端 Docker 有；没有完整后端 Docker/Compose，脚本会无条件 stop/rm/rmi | P1 先做安全、可重复部署 |
| 30 | Import/Export、全文搜索、密码重置、2FA、i18n | ⏸ | 均不是当前框架底座要求 | 由产品模块触发，禁止提前堆功能 |

### 结论：优先补什么

用户点名的日志、验证、文件上传、返回四类都**不是空白**，但只有“有代码”还不等于“可作为框架能力”：

- 日志：基础 logger 已有，需补请求关联、脱敏、保留和前后端合同。
- 验证：绑定器已存在，需补统一错误 envelope、body 限制、未知字段策略和跨服务规则。
- 文件上传：驱动已存在，需先解决 local disk 是否公开、路径/文件名/MIME/大小和私有下载授权。
- 返回：响应包和文档已存在，需强制所有 handler 走同一出口，并消除 List/Meta 与 {list,total} 等漂移。

---

## 3. 当前问题清单（按上线风险排序）

### P0：上线前必须处理

| 问题 | 影响 | 位置/证据 | 验收条件 |
| --- | --- | --- | --- |
| 配置模板带固定数据库、Redis、JWT 值；本地 config.yaml 为 0644 且含非空敏感配置 | 示例被复制到仓库、镜像或备份时可能泄露凭据 | config.example.yaml；internal/config/load_test.go 只拦截旧占位值 | 模板无固定 secret；敏感配置启动前校验；测试覆盖常见真实样式；已暴露值完成轮换（轮换本身需外部执行） |
| Local storage 目录被 Console server 对所有 disk 注册静态路由 | 私有文件可能无需认证直接下载 | app/console/internal/server/server.go；internal/config/types.go 缺少 Public/ServeStatic | 默认 private；public 明确声明；私有下载必须鉴权/签名；越权测试通过 |
| JWT 解析只检查 HMAC 类型，没有固定 issuer/algorithm/audience；API 不拒绝 Console user type | token 混用或算法/签发方边界不清 | pkg/auth/token.go；app/api/middleware/auth.go；Console middleware 有额外检查 | alg/iss/aud/sub/user_type 全部按配置校验；API/Console 互换 token 的测试失败 |
| API 权限中间件未实际挂载，Console 非生产 enforcer 缺失时 fail-open | 权限配置可能只是“看起来存在”，开发配置掩盖线上错误 | app/api/internal/router/router.go；app/api/middleware/permission.go；app/console/internal/middleware/admin_auth.go | 受保护路由显式声明权限；所有环境缺依赖都 fail-closed；route contract 覆盖 |
| Scheduler 没有 panic recovery | 一个计划任务 panic 可能终止 worker 进程 | pkg/scheduler/scheduler.go 未传 cron.WithChain(cron.Recover(...)) | 任务 panic 被记录并隔离；worker 继续服务；race/行为测试通过 |
| 审计日志原样保存大部分 query | token、签名、个人信息进入持久化审计记录 | app/console/internal/middleware/audit_log.go | 敏感键按 allowlist/denylist 脱敏；长度、结构和保留策略有测试 |

### P1：框架稳定性和维护成本

| 问题 | 影响 | 位置/证据 | 建议 |
| --- | --- | --- | --- |
| Console 日志后端只注册 GET，前端却调用 delete/clear/login detail；响应字段和筛选字段不一致 | 页面运行时 404、分页空白、类型失真 | app/console/internal/handler/log.go、app/console/internal/docs/contract.go、web/admin-vben/apps/console/src/api/log.ts、日志页面 | 以 OpenAPI/contract 选一个真相源，删除死服务或补齐真实 API |
| Provider 暴露 DB、Redis、Job、Storage、Cache、Observability 等大量可变字段 | service/handler 容易依赖整个 Service Locator，测试和演进困难 | internal/provider/provider.go；app/console/internal/router/router.go | 构造函数只接收实际依赖；Provider 只留在装配层；生成器同步更新 |
| 路由元数据使用包级 sync.Map | 多 engine、测试并行和热装配会互相污染 | pkg/route/wrap.go | 采用 engine-scoped catalog；移除生产 ResetForTest 依赖 |
| Casbin 只启动时 LoadPolicy，无 watcher/失效通知 | 多 Console 实例权限可能长期陈旧 | pkg/rbac/casbin.go | P1 先补明确刷新/版本策略；跨实例再接 watcher |
| DB Postgres DSN 用字符串拼接；连接启动无明确 bounded ping；MySQL 观测写死 postgres | 特殊密码连接失败，启动可能长时间阻塞，指标误导 | pkg/database/database.go；internal/observability/database.go | 使用结构化 DSN/超时；db.system 从 driver 派生；启动失败可诊断 |
| CoreServer 只设置 Read/Write/MaxHeaderBytes，IdleTimeout 和零值保护不完整 | 连接长期占用或显式零值削弱 HTTP 防护 | pkg/server/core.go、internal/config | 给 timeout 正数默认值并校验；按配置设置 IdleTimeout，补慢连接测试 |
| Local storage driver 复用 JWT secret 作为文件驱动 secret | 不同密钥域耦合，轮换 JWT 可能影响文件；泄露面扩大 | internal/provider/provider.go | 为 storage 使用独立配置 secret；旧值迁移有兼容窗口 |
| readiness 为每次自定义 check 再开 goroutine | check 不尊重 context 时可能留下永久 goroutine | internal/readiness/readiness.go 的 Checker.Run | 单次执行模型可取消、可限时，不留下探针 goroutine |
| Cache 默认 prefix 只使用 app name；配置中的分页上限未被使用 | 多服务键碰撞，配置看似可调但实际无效 | internal/provider/provider.go；internal/config/types.go；app/console/internal/service/common.go | namespace 至少包含环境/服务；分页统一读取配置并有上限 |
| migration 文件用秒级时间戳，失败时可能留下半成品 | 并发生成或失败重试破坏迁移顺序 | pkg/migrate/migrate.go | 唯一命名、原子写入、失败清理和重复测试 |
| GitLab 远端却只有 GitHub workflow；CI 未门禁 lint、循环依赖、diff check | 关键检查可能根本不执行，质量回归晚发现 | .github/workflows/ci.yml、远端地址 | 确认 canonical CI；补 Go/前端 lint、依赖图和 diff 门禁 |
| 前端生产 env、Vite proxy、Docker 脚本和包 alias 漂移；Docker script 无条件 stop/rm/rmi | 构建/部署在不同机器上不可重复或有破坏性 | web/admin-vben/.env.*、vite.config.mts、scripts/deploy/*、package.json | 固定入口和版本；脚本对不存在资源幂等；镜像有 .dockerignore |

### P2：可维护性和按需演进

- 删除或迁移无生产调用的旧 operation_log.go、login_log.go 服务，避免维护两套日志语义。
- 统一文件命名、请求/响应类型、列表分页和错误码；旧 Params/Return 只在触碰时迁移，不做全仓机械替换。
- 为 pkg/server 决定定位：若只服务本仓库，移入 internal；若要成为外部框架，先抽出不暴露 internal.Config/Provider 的公开 Kernel。
- 只有真实需求出现时再实现 Mail、Notification、WebSocket、Webhook、Outbox、2FA、全文搜索等能力。


---

## 4. 目标组件基线与建议契约

以下是“组件完成”的最低定义。具体 API 名称可以在实现任务中微调，但语义不能模糊。

### 4.1 日志：pkg/logger

**目标：** 结构化、可关联、可脱敏、可关闭；业务代码不直接依赖标准库 log。

- Logger 应支持从 context.Context 自动带出 request_id、trace_id、用户/服务身份等字段。
- 提供 Debug/Info/Warn/Error 和 With(fields...)；错误字段使用 error/cause，而不是拼接字符串。
- 对 password、token、cookie、authorization、签名、上传内容等字段做统一 redaction；禁止把完整 request body 写入日志。
- access log、业务 log、audit log 三者分开语义：访问记录不代替审计，审计不记录 secret。
- 组件由启动层创建并在关闭时 flush；禁止业务包持有可变全局 logger。
- 日志字段保持英文 snake_case，消息可中文；生产日志不得依赖 debug 文本做机器解析。

### 4.2 返回与错误：pkg/errx + pkg/response

统一 envelope（与现有文档兼容，但逐步收敛实现）：

~~~json
{
  "code": 0,
  "message": "ok",
  "data": {},
  "request_id": "req_xxx"
}
~~~

失败时使用稳定的 error_code、HTTP status 和可选字段错误：

~~~json
{
  "code": -1,
  "message": "请求参数校验失败",
  "data": {
    "error_code": "invalid_params",
    "errors": {"email": ["邮箱格式不正确"]}
  },
  "request_id": "req_xxx"
}
~~~

规则：

- handler 只调用 response.OK/Created/NoContent/Error 等少量入口，不直接散落 c.JSON。
- errx 保存稳定 code、HTTP status、用户 message、内部 cause；生产响应隐藏 cause，debug 只在显式开启时返回。
- 绑定格式错误通常是 400，字段语义校验通常是 422，认证/权限为 401/403，依赖不可用为 503；不能用 code=-1 代替正确 HTTP status。
- panic 由最外层 middleware 捕获并记录 request/trace 信息，响应统一为 500；不返回 stack trace。
- 成功的 list 统一一种分页形状（建议 list + meta），前后端不得同时维护 total 和 meta.total 两套协议。

### 4.3 请求验证：pkg/validation

- 统一提供 BindJSON、BindQuery、BindURI；handler 自己声明 Request 类型和标签。
- 错误映射使用稳定字段名和中文 label，不能把 validator 内部英文直接泄漏给用户。
- 配置未知字段策略：公共 API 默认拒绝未知 JSON 字段；兼容接口显式放宽并写测试。
- 所有请求先经过 body/header/上传总量限制，再进入 JSON decoder；限制值来自配置且必须有正数默认值。
- 跨字段和业务校验放在 service/input 的显式 Validate，不要把数据库查询塞进 validator。
- 验证失败不得写入数据库、投递任务或产生不可逆副作用。

### 4.4 文件上传与存储：pkg/storage

建议以输入/输出结构体固定边界：

~~~go
type UploadInput struct {
    Disk        string
    Directory   string
    Filename    string
    Content     io.Reader
    Size        int64
    ContentType string
    Private     bool
}

type Object struct {
    Disk        string
    Key         string
    Size        int64
    ContentType string
    ETag        string
    URL         string // 仅对明确 public 或短期 signed URL 填充
}
~~~

最低规则：

- 配置明确 public / private；默认 private。私有对象只能通过鉴权下载或短期签名 URL 读取。
- 使用流式写入和 MaxBytesReader；同时限制单文件、请求总量、扩展名、探测后的 MIME，不信任客户端 Content-Type。
- 文件名只用于展示；实际 key 使用安全 ID/日期目录，清理 ..、分隔符和控制字符，避免路径穿越和覆盖。
- 保存前后记录大小、checksum、实际 MIME 和 owner；错误时清理半成品对象。
- local disk 不得被通用静态路由默认暴露；需要公开资源时单独声明 disk 和路由。
- 病毒扫描、图片缩略图、分片上传属于按需扩展，不阻塞普通上传基线。

### 4.5 认证与授权：pkg/auth、pkg/permission、pkg/rbac

- 解析 JWT 时固定允许算法、issuer、audience、过期/生效时间和 subject；签发和解析共用配置但不复用无关加密密钥。
- API 与 Console 的 user type、issuer 和 audience 必须隔离；refresh token 必须持久化、轮换、撤销并防重放。
- 浏览器 token 存储只选择一种主策略（安全 Cookie 或明确的 SPA token 方案），记录 XSS/CSRF 边界；不要把“客户端加密存储”当成 XSS 防护。
- 路由声明权限，权限中间件实际挂载；依赖缺失在所有环境 fail-closed。Casbin 多实例刷新策略必须可观察。

### 4.6 数据库、缓存、任务和调度

- 数据库连接使用结构化 DSN、连接/迁移超时和 pool 配置；事务在 service 边界开始和提交，错误显式回滚。
- Cache 明确 store、namespace、TTL、序列化错误、降级行为和命中指标；不能静默把不同服务写入同一前缀。
- Job 定义稳定 TaskType、重试/退避、失败队列、幂等 claim、关闭超时和指标；不要为了重构换掉已有任务类型。
- Scheduler 每个 job 必须 recover panic、尊重 context 和 timeout；进程内 Mutex 不冒充分布式锁。

### 4.7 HTTP Client、观测与 CLI

- HTTP client 每次请求使用不可变 request state；默认 timeout，重试只对明确幂等方法和网络错误生效；出站 URL 经过 scheme/host/redirect/内网策略检查。
- Metrics/trace 从实际 driver、route 和错误 code 派生；readiness check 必须可取消、有限时，不为每次探针泄漏 goroutine。
- cmd/grove 提供可诊断的 doctor（配置、DB、Redis、迁移状态、写权限），生成文件原子写入且处理时间戳碰撞；CLI 输出用 fmt.Print*，运行时日志仍走 pkg/logger。

---

## 5. 目录、文件命名与模块边界规范

### 5.1 推荐目标树（增量采用，不要求一次搬完）

~~~text
grove/
├── app/
│   ├── api/
│   │   ├── cmd/
│   │   ├── handler/
│   │   ├── service/
│   │   ├── middleware/
│   │   └── internal/{router,server}/
│   ├── console/
│   │   ├── cmd/
│   │   ├── handler/
│   │   ├── service/
│   │   ├── middleware/
│   │   └── internal/{router,server}/
│   └── worker/
│       ├── cmd/
│       ├── handler/
│       └── internal/{server,consumer}/
├── cmd/grove/
├── internal/
│   ├── bootstrap/       # 启动公共装配
│   ├── config/          # 配置类型、加载、生产校验
│   ├── docsui/          # OpenAPI UI/页面
│   ├── middleware/      # 跨 app 的 HTTP 中间件
│   ├── model/           # 共享持久化模型和查询辅助
│   ├── observability/   # OTel、metrics、readiness
│   └── provider/        # 资源创建、依赖图、Close
├── pkg/
│   ├── auth/ cache/ database/ errx/ event/ httpclient/
│   ├── job/ logger/ migrate/ permission/ request/ response/
│   ├── route/ scheduler/ server/ storage/ transaction/ validation/
│   └── ...              # 仅已证明可复用的组件
├── database/{migrations,seeds}/
├── docs/{architecture.md,guide/,plans/}
└── web/admin-vben/
~~~

**迁移规则：** 现有路径已经被文档、生成器和调用方使用，不做全仓机械搬迁。新代码按上树放置；旧模块只有在修改时顺手收敛，并在同一 PR 更新 import、测试、OpenAPI 和文档。

### 5.2 放置决策

| 代码类型 | 放置 | 不应放置 |
| --- | --- | --- |
| HTTP 请求绑定、身份读取、响应输出 | app/<service>/handler | pkg、model、数据库层 |
| 业务流程、事务、任务投递 | app/<service>/service | handler、通用 model |
| 服务独有中间件 | app/<service>/middleware | 共享 internal/middleware |
| 启动、依赖创建、生命周期 | app/*/internal/server、internal/provider | service、handler |
| 共享 GORM 模型/查询辅助 | internal/model | pkg 中的业务模型 |
| 无业务语义的稳定基础能力 | pkg/<component> | utils、common、helpers 垃圾桶 |
| SQL schema 变更 | database/migrations | 启动时 AutoMigrate |
| 前端页面/API 类型 | web/admin-vben/apps/console/src | Go handler 内嵌前端逻辑 |

### 5.3 文件与符号命名

- Go 目录和 package 使用小写、单数、领域名：auth、storage、operationlog；避免 misc、common、utils。
- Go 文件使用小写 snake_case：auth_state.go、system_config.go、operation_log_handler.go；测试使用同名 _test.go。
- 一个主要领域类型对应清晰文件；不要用 service.go、model.go 承载多个不相关领域。
- 构造函数为 NewXxx；服务命名 XxxService，handler 命名 XxxHandler；输入/输出命名 CreateRoleInput、CreateRoleOutput。
- HTTP 类型与业务类型分离：CreateRoleRequest/Response 只属于 handler；service 使用 Input/Output；不要把 Gin binding tag 带进 model。
- 方法名表达动作：Create、Update、List、Get、Delete；避免 Do、HandleData、Process 等无语义动词。
- 路由名由 METHOD + full path 稳定生成；权限 key、菜单 key、任务 TaskType 一旦发布不得因文件重命名而改变。
- migration 使用 YYYYMMDDHHMMSS_name.up.sql / .down.sql；生成器必须防止同秒碰撞。
- 前端类型/API 名称与 OpenAPI operationId 对齐；不要同时保留旧 alias 和新 alias 作为两个入口。

---

## 6. Go 编码、面向对象与抽象规则

### 6.1 “面向对象”在 Go 中的落地方式

Grove 使用封装、组合、多态和依赖反转，但不模拟 Java/Laravel 的类继承体系：

- 用小型 struct + 方法封装状态和不变量；用组合表达能力复用。
- 默认依赖具体类型；只有存在多个实现、外部边界或真实替换需求时才定义接口。
- 接口由使用方定义，保持一个行为或一组紧密行为；不要为每个 struct 预先写 IUserService。
- 构造函数显式注入依赖；禁止运行时字符串反射解析、隐式 Facade、全局可变 singleton。
- 工厂、Strategy、State、Decorator 只有在当前代码已经出现多个变化点时引入；单一实现不增加模式层。

### 6.2 分层硬规则

1. **Handler**：绑定和校验 Request，读取 request.Identity/request ID，调用 service，调用统一 response；不直接查 DB、不启动 goroutine、不写权限决策。
2. **Service**：接收 context.Context，负责业务规则、事务、聚合、缓存失效、事件/任务投递；返回可测试的 Output 或 typed error。
3. **Model**：共享持久化结构和查询辅助；不依赖 Gin、HTTP status、用户权限或响应 envelope。
4. **Provider/Server**：只在启动和装配层创建连接、logger、storage、job、scheduler，并按逆序 Close；业务对象不接收完整 Provider。
5. **Middleware**：只做横切关注点（request id、auth、permission、limit、audit、observability）；不要把业务 CRUD 塞进中间件。

### 6.3 错误、并发、资源和安全

- 所有外部 I/O 接收 context；连接、HTTP、Redis、任务、文件和锁都要有 timeout/cancel 和明确 Close。
- 不忽略错误；生产代码不得空白标识符吞掉 Close、Rollback、文件删除或复制错误。确需忽略时写出原因并测试。
- goroutine 必须有 owner、退出条件和等待方式；请求结束后不得继续持有请求 context 做不可取消工作。
- SQL 参数化；排序字段、列名和资源名使用 allowlist；分页有最大值，深分页再按需引入 cursor。
- 密钥、token、Cookie、密码、签名和原始文件内容不进入日志、错误 message、审计 query 或 API response。
- 不使用 panic 表达业务错误；panic 只用于不可恢复的启动配置错误或由最外层统一恢复的程序缺陷。
- 全局变量只用于不可变常量/注册表；路由、权限、缓存和 logger 状态必须按实例隔离。

### 6.4 测试规则

- 先测行为和契约，再测实现细节；Request/Response、错误码、权限和任务语义必须有 contract tests。
- 并发、生命周期、refresh token、缓存、scheduler、上传等共享组件至少跑 race；真实数据库方言用集成测试验证。
- 不以“覆盖率数字”替代关键路径；优先覆盖失败、取消、超时、重试、回滚、越权和资源关闭。
- 前端 API 类型由同一合同生成或校验；页面单测不能替代后端 route/OpenAPI contract。

### 6.5 明确不采纳的设计

- 不引入 ServiceContainer、Facade、自动扫描注解、通用 Repository、BaseService、BaseModel。
- 不为了“像 Laravel”复制 Eloquent 魔法、隐式全局 DB、运行时字符串绑定或继承层级。
- 不把所有组件都抽成接口；没有第二实现和替换边界的接口只是噪音。
- 不在本路线图提前建设多租户、插件、工作流、Outbox、通知或 WebSocket 的空壳 API。

---

## 7. 分阶段实施路线

### 阶段与依赖

~~~text
Phase 1  安全与契约阻断项
   ↓
Phase 2  组件和装配收敛
   ↓
Phase 3  合同自动化、部署和可运维性
   ↓（真实业务触发）
Phase 4  通知 / Webhook / Outbox / 实时能力
~~~

每个任务独立 PR/commit；同一时间只推进一个会改变公共契约的任务。任务完成前必须更新对应 guide 和测试。旧的 docs/plans/2026-07-* 计划保留为历史，不与本文重复执行。

## Phase 1：安全与契约阻断项（建议先做）

### Task 1：清理配置模板与启动安全校验

**Status:** [x] Completed (2026-08-29)

**Files:**

- Modify: config.example.yaml
- Modify: internal/config/load.go
- Test: internal/config/load_test.go
- Update: docs/guide/configuration.md、docs/deployment/deploy.md

**Steps:**

1. 删除模板中的固定 DB/Redis/JWT 凭据，保留空值和明确的填写说明；不要把当前本地 config.yaml 的值复制到文档、镜像或 Git。
2. 统一 YAML、环境变量展开和文档对配置来源的说明；明确后端是否读取环境变量，不能让 .env 说明与代码冲突。
3. 生产校验增加 root 初始密码、服务 timeout、TLS/反向代理相关的可执行检查；错误信息只说明键名，不回显 secret。
4. 增加“常见固定凭据模式”回归测试，并检查模板文件权限/生成流程。

**Verification:**

~~~bash
go test ./internal/config
go vet ./internal/config
git diff --check
~~~

**Expected:** 测试拒绝固定凭据、生产弱配置和无效 timeout；模板不再包含可直接使用的 secret。

**Commit:** security(config): remove static credentials and tighten production checks

**Actual verification (2026-08-29):** `go test ./...`、`go vet ./...`、`go test -race ./internal/config ./cmd/grove` 和 `git diff --check` 通过；未提交 commit。生产环境的真实 secret 轮换仍需外部执行。

### Task 2：收紧 JWT、Session 和 API/Console 权限边界

**Status:** [x] Completed (2026-08-30)

**Files:**

- Modify: pkg/auth/token.go
- Modify: app/api/middleware/auth.go
- Modify: app/api/internal/router/router.go
- Modify: app/console/internal/middleware/admin_auth.go
- Modify: pkg/permission/permission.go、pkg/rbac/casbin.go
- Test: 对应 *_test.go、API/Console route contract tests
- Update: docs/02-console-架构与权限.md、docs/guide/permission.md

**Steps:**

1. 解析时固定允许算法、issuer、audience、时间窗口和 user type；增加错误分类和日志脱敏。
2. 明确 API 与 Console token 的 audience/issuer；补互换 token 必须失败的测试。
3. 在 API router 对需要权限的路由实际挂载 PermissionSet.Require，缺失 enforcer 在所有环境 fail-closed。
4. 为 Casbin 增加刷新/版本策略的最小接口；先保证单实例立即一致，多实例 watcher 作为后续任务。

**Verification:**

~~~bash
go test ./pkg/auth ./pkg/permission ./pkg/rbac ./app/api/... ./app/console/...
go test -race ./pkg/auth ./pkg/permission ./app/console/...
~~~

**Expected:** 算法混淆、错误签发方、跨服务 token、无权限中间件和 fail-open 场景均有回归测试。

**Commit:** security(auth): enforce token and permission boundaries

**Actual verification (2026-08-30):** `go test ./pkg/auth ./pkg/permission ./pkg/rbac ./app/api/... ./app/console/...`、`go test -race ./pkg/auth ./pkg/permission ./app/console/...` 和 API fail-closed route tests 通过；JWT 额外要求 `iat` 存在并有回归覆盖。Casbin 当前提供显式刷新/版本能力；跨实例 watcher 尚未引入，需按多实例业务触发。

### Task 3：封闭文件存储并建立上传/下载契约

**Status:** [x] Completed (2026-08-30)

**Files:**

- Modify: internal/config/types.go
- Modify: app/console/internal/server/server.go
- Modify: pkg/storage/*
- Modify: Console upload handler/service and tests
- Test: storage path/MIME/size/private-download tests
- Update: docs/guide/pkg-components.md、docs/guide/configuration.md

**Steps:**

1. 为 disk 增加明确 Public/ServeStatic 语义，默认 private；移除“遍历所有 local disk 即注册静态目录”的行为。
2. 统一 UploadInput/Object，限制 request body、文件大小、扩展名和探测 MIME；实际 key 不使用用户原始路径。
3. 私有对象只允许授权下载或短期签名 URL；上传失败清理半成品并记录 checksum/owner。
4. 增加 ../、绝对路径、伪造 MIME、超大文件、未授权下载和 public disk 的测试。

**Verification:**

~~~bash
go test ./pkg/storage ./app/console/...
go test -race ./pkg/storage ./app/console/...
~~~

**Expected:** 默认 disk 不可匿名访问，越权和路径逃逸测试失败（即被正确拒绝）。

**Commit:** security(storage): make file visibility explicit and uploads bounded

**Actual verification (2026-08-30):** `go test ./pkg/storage ./app/console/...`、`go test -race ./pkg/storage ./app/console/...` 通过；路径逃逸、伪造 MIME、大小限制、私有 local disk 和下载流测试通过。当前 Console 下载是管理员权限边界；对象 owner/租户 ACL 不在现有模型中，若业务引入用户私有文件需单独补授权字段和测试。


### Task 4：统一 response、error 和 validation 出口

**Status:** [x] Completed (2026-08-30)

**Files:**

- Modify: pkg/errx/*、pkg/response/*、pkg/validation/*
- Modify: 受影响的 app/api 和 app/console handlers
- Test: response/error/validation contract tests
- Update: docs/04-响应与错误处理规范.md、docs/development/error-handling.md

**Steps:**

1. 盘点直接 c.JSON、重复错误 envelope 和旧 Params/Return，只有在触碰的 handler 迁移到统一入口。
2. 固定 HTTP status、error_code、字段错误、request ID 和 debug/cause 的映射。
3. 统一 list 分页为一种形状，并让前端类型与后端 response 一致。
4. 补 JSON/query/URI 格式错误、422 字段错误、业务冲突、503 和 panic 的 contract tests。

**Verification:**

~~~bash
go test ./pkg/errx ./pkg/response ./pkg/validation ./app/api/... ./app/console/...
~~~

**Expected:** 同一类错误在所有入口拥有相同 status、error code 和 envelope；生产响应不含底层 cause。

**Commit:** feat(http): standardize response error and validation contracts

**Actual verification (2026-08-30):** `go test ./pkg/errx ./pkg/response ./pkg/validation ./app/api/... ./app/console/...` 和全量 `go test ./...` 通过；统一 envelope、字段错误、body limit、panic/503 映射有回归测试。

### Task 5：修复日志 API 漂移并补脱敏/保留策略

**Status:** [x] Completed (2026-08-30)

**Files:**

- Modify: app/console/internal/handler/log.go
- Modify: app/console/internal/service/*log*.go
- Modify: app/console/internal/docs/contract.go
- Modify: web/admin-vben/apps/console/src/api/log.ts 及日志页面
- Modify: app/console/internal/middleware/audit_log.go
- Test: backend route/handler tests and frontend unit tests
- Update: docs/guide/*log*（若不存在则在现有 Console 指南补章节）

**Steps:**

1. 选定后端真实能力为唯一 source：补齐前端确实需要的 delete/clear/detail，或删除前端死调用；不要保留两套未调用 service。
2. 统一列表响应（建议 list + meta）、筛选字段（success/status 的映射）和 operationId。
3. query、header、body 审计采用敏感键脱敏和长度上限；增加保留/清理策略的配置说明。
4. 在前端生成或校验 API 类型，修复页面对 res.total 的假设。

**Verification:**

~~~bash
go test ./app/console/...
cd web/admin-vben && pnpm test:unit -- --runInBand && pnpm typecheck
~~~

**Expected:** 日志页面所有请求都有后端路由和合同；敏感 query 不会持久化明文。

**Commit:** fix(console): align log contracts and redact audit inputs

**Actual verification (2026-08-30):** Console 日志路由/OpenAPI 与前端调用已统一为列表、详情和登录日志；审计 query/detail 脱敏、长度上限和合法 JSON fallback 测试通过，前端日志单测和合同测试通过。应用未自动清理审计记录，保留期仍由运维策略决定。

### Task 6：修复 Scheduler、readiness、数据库连接和观测标签

**Status:** [x] Completed (2026-08-30)

**Files:**

- Modify: pkg/scheduler/scheduler.go
- Modify: internal/readiness/readiness.go
- Modify: pkg/database/database.go
- Modify: internal/observability/database.go
- Test: scheduler panic/cancel/race；readiness timeout；Postgres/MySQL DSN tests
- Update: docs/guide/scheduler.md、docs/guide/database.md

**Steps:**

1. 为 cron chain 加 recovery，记录 job name/request-independent context，并保证单个 panic 不终止 worker。
2. 将 readiness check 约束为可取消、有限时的执行模型，避免每次探针无限新增 goroutine。
3. 用结构化 DSN 和 bounded connect/ping；从实际 driver 生成 db.system。
4. 明确进程内 Mutex 与未来分布式锁的边界，不在本任务偷偷引入 Redis 锁。

**Verification:**

~~~bash
go test ./pkg/scheduler ./internal/readiness ./pkg/database ./internal/observability
go test -race ./pkg/scheduler ./internal/readiness
~~~

**Expected:** panic、取消、连接超时和 MySQL 观测标签都有回归覆盖；无 Docker 时明确标记未做真实 DB 集成。

**Commit:** fix(runtime): isolate scheduler failures and bound readiness checks

**Actual verification (2026-08-30):** `go test ./pkg/scheduler ./internal/readiness ./pkg/database ./internal/observability`、`go test -race ./pkg/scheduler ./internal/readiness` 和全量 `go vet ./...` 通过；Scheduler panic recovery、readiness cancel/deadline、PostgreSQL/MySQL driver 标签均有覆盖。真实数据库连接仍需容器环境。

### Task 7：建立 canonical 质量门禁并清理破坏性脚本

**Status:** [x] Completed (2026-08-30)

**Files:**

- Add/Modify: 与实际平台对应的 .gitlab-ci.yml（若 GitLab 为 canonical）或明确 .github/workflows/ci.yml 镜像策略
- Add/Modify: .golangci.yml
- Modify: web/admin-vben/package.json、根 package.json、vite.config.mts
- Modify: web/admin-vben/scripts/deploy/*，新增 .dockerignore
- Update: docs/commands.md、docs/deployment/deploy.md

**Steps:**

1. 先确认 GitLab/GitHub 哪一个实际执行；只保留一个 canonical 状态入口。
2. 将 Go lint、frontend lint、循环依赖、git diff --check、govulncheck 纳入门禁，修复当前已知 errcheck/stylelint。
3. 删除或标记过时前端 alias；Makefile/package script 使用同一命令。
4. Docker 脚本对不存在容器/镜像幂等，不自动删除共享资源；固定可审计的 base image 版本并排除 node_modules/config secret。

**Verification:**

~~~bash
golangci-lint run
cd web/admin-vben && pnpm lint && pnpm check:circular
git diff --check
~~~

**Expected:** 本地和 canonical CI 对同一组检查给出相同结果；脚本首次运行和重复运行都安全。

**Commit:** ci: align quality gates and make deployment scripts safe

**Actual verification (2026-08-30):** 本地 `golangci-lint run`、前端 lint/循环依赖/类型检查/322 个单元测试/合同测试、`make admin.contract` 和 `git diff --check` 通过；GitLab canonical pipeline 与 GitHub 镜像配置已加入同一组门禁。远端 CI 尚未触发，`govulncheck` 本机未安装。


## Phase 2：组件契约与装配收敛

### Task 8：把路由元数据改为 engine-scoped catalog

**Status:** [x] Completed (2026-08-30)

**Files:** pkg/route/wrap.go、pkg/permission/*、pkg/server/*、各 app router、相关 tests。

**Steps:**

1. 用明确的 RouteCatalog 所有权记录 name/scope/ignore/method/path。
2. 将 catalog 注入 server/router/permission collector，删除生产代码对包级 map 的依赖。
3. 保留必要的测试构造辅助，但不得通过全局 reset 影响并行测试。
4. 验证 API、Console 两个 engine 同时创建时元数据不串线。

**Verification:** go test -race ./pkg/route ./pkg/permission ./app/api/... ./app/console/...

**Commit:** refactor(route): scope route metadata to each engine

**Actual verification (2026-08-30):** `pkg/route`、`pkg/permission`、API/Console route contract 和全量 race 测试覆盖 API/Console catalog 同时创建时不串线；包级旧 API 仅保留兼容测试/调用入口，生产装配已使用实例级 catalog。

### Task 9：缩小 Provider 和生成器的依赖面

**Status:** [x] Completed (2026-08-30)

**Files:** internal/provider/provider.go、app/console/internal/router/router.go、app/api/internal/router/router.go、cmd/grove/templates.go、各 handler/service 构造函数。

**Steps:**

1. 为每个 handler/service 列出实际依赖，先改新增/正在修改的模块。
2. 用小型 options struct 或显式参数替代传递完整 Provider；不创建通用容器。
3. 让 APIOptions/ConsoleOptions/WorkerOptions 成为唯一装配边界，保持 Close 逆序。
4. 更新生成器，使新模块天然遵守该规则，并加生成代码编译 contract。

**Verification:** go test ./internal/provider ./app/... ./cmd/grove && go vet ./...

**Commit:** refactor(di): narrow application dependencies at composition boundaries

**Actual verification (2026-08-30):** API/Console handler 已改为精确依赖，Provider 只在 router/server 装配层出现；生成器模板同步更新并通过 `go test ./cmd/grove ./app/... ./internal/provider`、`go vet ./...`。

### Task 10：统一配置、数据库、缓存和分页的可执行契约

**Status:** [x] Completed (2026-08-30)

**Files:** internal/config/types.go、internal/config/load.go、pkg/database/*、pkg/cache/*、app/console/internal/service/common.go、分页 handlers/tests。

**Steps:**

1. 移除未使用或命名含义不清的配置，或补上真实读取路径；每个配置键都有默认值、校验和文档。
2. Cache namespace 包含服务/环境可辨识部分，明确 Redis 不可用时的 fail/degrade 策略。
3. 所有列表使用统一 PageInput/PageMeta，读取 DefaultPerPage/MaxPerPage 并限制排序字段。
4. 增加 PostgreSQL/MySQL 配置和分页边界测试。

**Verification:** go test ./internal/config ./pkg/cache ./pkg/database ./app/console/...

**Commit:** feat(platform): make config cache database and pagination contracts executable

**Actual verification (2026-08-30):** 配置正数校验、数据库超时/DSN、Redis namespace、分页默认值/上限/排序白名单测试通过；缓存默认策略是 Redis 错误向上返回，不自动伪装为内存回退。

### Task 11：建立 OpenAPI 与前端 API 的单一合同

**Status:** [x] Completed (2026-08-30)

**Files:** app/api/internal/docs/*、app/console/internal/docs/contract.go、web/admin-vben/apps/console/src/api/*、contract scripts/CI。

**Steps:**

1. 决定 OpenAPI 是手写 source 还是从 Go 注解/测试生成，仓库只保留一个 source of truth。
2. 为 response/error/pagination/auth/log/upload 定义 schema 和 operationId。
3. 生成或校验 TypeScript client/types；禁止页面自行猜测字段。
4. 在 CI 比较路由、OpenAPI 和前端调用，变更必须显式更新合同。

**Verification:** go test ./app/api/... ./app/console/...、make admin.typecheck、前端 contract check。

**Commit:** feat(contract): connect OpenAPI schemas with console client types

**Actual verification (2026-08-30):** OpenAPI 继续作为 Go 侧手写 source；新增前端 `console-contract.json` 注册表及合同测试，逐项校验 operationId、method、path，`check:console-api-contract` 已接入 GitHub/GitLab 门禁；前端类型检查和构建通过。

### Task 12：清理死代码并增强迁移/生成器安全

**Status:** [x] Completed (2026-08-30)

**Files:** app/console/internal/service/operation_log.go、login_log.go、pkg/migrate/migrate.go、cmd/grove/main.go、cmd/grove/*_test.go。

**Steps:**

1. 用静态调用图确认旧日志 service 无生产调用；删除或明确标记 deprecated，并迁移唯一实现。
2. migration 生成使用唯一时间戳/冲突重试、临时文件 + rename；up/down 任一步失败都清理。
3. CLI seed 错误路径显式 rollback；copy/write/close 错误全部处理。
4. 为生成器重复运行、并发运行和部分失败增加测试。

**Verification:** go test ./pkg/migrate ./cmd/grove ./app/console/...、golangci-lint run。

**Commit:** chore: remove dead paths and make generators atomic

**Actual verification (2026-08-30):** 已删除无生产调用的旧日志 service；migration 与 module generator 使用锁、临时文件和原子发布，失败清理与并发生成测试通过，`go test ./pkg/migrate ./cmd/grove ./app/console/...` 通过。

## Phase 3：可运维交付与框架边界

### Task 13：补齐后端部署制品和真实环境验收

**Status:** [x] Completed (2026-08-30)

**Files:** Dockerfile（按服务拆分或明确单镜像策略）、docker/、docs/deployment/*、CI 配置、health/metrics 配置。

**Steps:**

1. 明确 API/Console/Worker 的镜像、端口、非 root 用户、只读文件系统和 graceful shutdown。
2. secret 通过受控配置/secret manager 注入，不进入镜像、日志和示例。
3. 提供 PostgreSQL、MySQL、Redis 的 staging smoke 流程；迁移、readiness、队列和静态文件策略逐项验收。
4. 记录“本地构建通过”和“真实环境验收通过”的区别。

**Verification:** make build、镜像扫描、staging migrate/status、真实健康检查和浏览器 E2E（需外部环境）。

**Commit:** ops: add reproducible backend deployment and staging checks

**Actual verification (2026-08-30):** 已提供三服务后端 Dockerfile、`.dockerignore`、只读文件系统运行说明、GitLab container build job 和 PostgreSQL/MySQL/Redis staging checklist；`make build` 与前端构建通过。本机 Docker daemon 不可用，镜像构建、扫描和 staging smoke 尚未执行。

### Task 14：按业务触发实现跨实例可靠性组件

**Status:** [ ] Deferred

**Trigger:** 出现跨事务事件、重复 webhook、跨实例 scheduler 或严格 exactly-once 业务要求。

**Possible files:** pkg/lock、pkg/idempotency、pkg/outbox、pkg/webhook、app/worker 或 app/console 运维页面。

**Rules:**

- 先写业务不变量、重试和失败恢复设计，再选 Redis/DB/outbox；不把进程内 Mutex 升级成“伪分布式锁”。
- 若现有任务已经有稳定 TaskType 或记录级 claim/fencing 语义，必须保持兼容；可靠性组件要有唯一键、过期、重放和观测指标。

**Verification:** 真实多实例、断电/重试/重复投递测试；不能只用单进程 unit test 宣称完成。

### Task 15：按业务触发实现 Mail/Notification/实时能力

**Status:** [ ] Deferred

**Trigger:** 产品明确需要邮件、短信、站内信、WebSocket/Broadcast 或多渠道模板。

**Rules:**

- 先确定同意、退订、模板版本、敏感内容和 provider quota；发送动作走 job，状态可追踪。
- 实时连接必须有身份、租户/资源授权、心跳、断线重连和 backpressure；不把它塞进现有 Event dispatcher。

**Verification:** provider sandbox、重试/退避、失败队列、脱敏和浏览器/移动端 E2E。

### Task 16：评估是否抽出公开 Framework Kernel

**Status:** [ ] Deferred

**Trigger:** 有第二个独立项目要复用 Grove，且需要稳定的外部 API/版本承诺。

**Rules:**

- 先设计不暴露 internal/config.Config、internal/provider.Provider 的公开 Kernel/Options；保留当前仓库作为应用示例。
- 通过独立 package、兼容性测试和版本策略验证后再移动 pkg/server；没有真实消费者时不做抽象性搬家。


---

## 7.1 本轮 Deferred 复核

本轮已完成所有当前代码和现有业务边界内可实施的 Task 1–13。Task 14–16 保持 Deferred，不用“补空壳组件”的方式制造完成假象：

- **Task 14（跨实例可靠性）**：当前 scheduler 的互斥语义明确是进程内；尚未发现需要跨事务 outbox、重复 webhook 去重或 exactly-once 的业务不变量。触发条件出现后，先定义唯一键、重试、恢复和观测，再选择 Redis/DB 实现。
- **Task 15（通知/实时）**：当前没有已确认的邮件、短信、站内信、WebSocket 或 Broadcast provider 合同。引入前必须先确认同意/退订、模板、配额、身份授权和 backpressure。
- **Task 16（公开 Kernel）**：只有当前 Grove 仓库一个真实消费者，`internal/config` 和 `internal/provider` 仍是应用装配边界；暂不抽象公开 API，也不为“像 Laravel”搬迁 `pkg/server`。

文件存储的 owner/租户授权同样不提前虚构：当前 Console 存储是受 RBAC 保护的共享管理员资产。若产品以后要求用户级私有文件，Task 3 需要以 owner/ACL 数据模型、查询过滤和越权测试重新开启。

## 8. 每个阶段的验收门槛

### 功能与契约

- 新增/修改路由同时更新权限清单、OpenAPI、前端类型和文档。
- 成功、校验失败、认证失败、越权、冲突、依赖不可用和 panic 都有稳定 envelope。
- 上传、日志、审计和错误响应没有 secret、原始 token 或不必要的个人信息。

### 可靠性

- 外部 I/O 有 timeout/cancel；goroutine 有 owner 和退出路径；Close 错误不被吞掉。
- 队列和 scheduler 的重试、panic、取消、关闭和幂等边界有测试。
- 多实例行为（Casbin 刷新、锁、session、cache namespace）要么有实现和集成测试，要么在文档明确“不支持”。

### 工程质量

~~~bash
go test ./...
go test -race ./...
go vet ./...
make build
make admin.typecheck
make admin.build
golangci-lint run
git diff --check
~~~

前端还应执行：

~~~bash
cd web/admin-vben
pnpm lint
pnpm check:circular
pnpm test:unit -- --runInBand
~~~

真实环境另行执行：

- govulncheck ./...（使用仓库要求的 Go 1.25.12）。
- Testcontainers PostgreSQL/MySQL、Redis contract。
- staging migration up/down、readiness、队列和对象存储。
- 认证、权限、上传、日志页面和错误响应的浏览器 E2E。

### 文档与变更

- docs/architecture.md、docs/guide/* 与源码/Makefile 一致；旧计划标为历史，不作为当前行为依据。
- 每个公共配置键、命令、错误码、任务类型和路由权限 key 有唯一说明。
- 交付说明区分：本地已验证、CI 已验证、staging 已验证、生产尚未验证。

---

## 9. 非目标与风险边界

- 本计划不承诺 Laravel API 兼容，也不承诺覆盖 Laravel 全部产品组件。
- 本计划不自动轮换线上 secret、不执行生产迁移、不修改外部 GitLab/云存储/微信等系统；这些需要独立授权和验收。
- 不因“框架完整”而引入微服务、DDD 全套层次、通用 Repository、插件市场或反射式 DI。
- 迁移旧接口时保留兼容窗口；任务类型、权限 key、路由 name 和数据库字段不能为整洁而随意更名。
- 当前发现的 Docker、GitLab CI、真实 DB/Redis、浏览器 E2E 问题，若没有对应环境只能标记未验证，不能用本地 build 代替上线结论。

---

## 10. 下一步执行顺序

默认从 **Phase 1 / Task 1 → Task 7** 顺序推进；Task 2 和 Task 3 属于安全阻断项，Task 4/5 负责把用户点名的“返回、验证、日志”变成可复用契约，Task 6/7 负责避免修复后再次回归。完成 Phase 1 后再开始 Provider、路由和 OpenAPI 收敛。

建议每个任务的交付信息固定为：

~~~text
任务：Task N
变更文件：...
失败测试：...
实现结果：...
实际验证：...
未验证外部条件：...
Commit/PR：...
~~~

**下一步开发入口：** 当前基线升级已完成，可从真实业务模块开始增量开发。新增模块优先使用 `make:module`/显式依赖/`Request → Handler → Service → Model` 分层，并同步更新 OpenAPI、前端合同、权限清单、迁移和测试；若触发 Deferred 条件，再按对应 Task 重新立项。

### 10.1 基线后的文档维护（2026-08-30）

- 已将 canonical 指南中的旧式 `Provider` 持有、`route.Wrap` 和直接从 Provider 取组件的业务示例，统一为显式依赖、实例级 `route.Catalog` 和装配边界取依赖。
- 新增 `make docs.check`，并纳入 `make quality`，防止新增文档再次引入已废弃的架构示例。
- 当前仓库仍只有系统管理与演示 API，没有已确认的业务领域；真实业务纵向切片需先确认用户、内容、订单等产品边界后再实施。
