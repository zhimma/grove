# Grove Go 基础项目审查报告

审查日期：2026-05-07

## 结论摘要

当前项目是一个比较完整的 `console-first` Go 单仓基础框架：后端按 `api / console / worker` 三个服务入口拆分，公共启动、配置、Provider、数据库、认证、权限、响应、存储、任务、迁移等能力基本齐全；`console` 后台的 RBAC、菜单权限、运行时 API 权限清单、操作日志、登录日志和前端 `admin-vben` 协同方向也比较明确。

整体方向合理，适合作为后台项目模板继续迭代。但从“基础框架”的角度看，还存在一些需要优先处理的问题：

1. 若干基础组件存在真实行为 bug，尤其是 Redis `Add`、HTTP client 重试与 multipart 字段、异步事件关闭、CORS 头处理、本地存储路径边界判断。
2. Provider 暴露了 `Cache / HTTPClient / Event / Scheduler` 字段和初始化函数，但默认 `APIOptions` / `ConsoleOptions` 未装配，文档和实际可用能力不一致。
3. `console` 业务中数据库变更与 Casbin 策略变更没有事务边界，创建管理员、更新角色、删除角色、配置权限时可能出现“数据库成功但权限同步失败”的半成功状态。
4. 认证注销和刷新令牌撤销只在进程内内存 blacklist，重启或多实例部署会失效，不适合作为正式生产会话撤销机制。
5. 目录和命名大体清晰，但 `pkg/permission` 已包含 console 菜单目录，和 `pkg` “无业务语义”的定位冲突；部分 service 直接收 `*gorm.DB`，部分收 `database.Repo`，依赖风格不统一。

建议先修复行为 bug 和安全边界，再统一 Provider 装配和 service 依赖规范，最后再补充框架级文档、代码生成和测试覆盖。

## 项目运行流程

### 启动链路

三个后端服务入口基本一致：

- `app/api/cmd/main.go`
- `app/console/cmd/main.go`
- `app/worker/cmd/main.go`

HTTP 服务链路：

```text
cmd/main.go
  -> internal/config.LoadWithOptions
  -> app/*/internal/server.NewServer
  -> pkg/server.NewCoreServer
  -> internal/provider.New
  -> gin.New + global middleware
  -> app/*/internal/docs.RegisterDocs
  -> app/*/internal/router.InstallToEngine
  -> handler -> service -> model/pkg
```

`worker` 不使用 `CoreServer`，链路为：

```text
app/worker/cmd/main.go
  -> config.LoadWithOptions
  -> provider.New(... WorkerOptions)
  -> handler.RegisterDefaultJobs
  -> job.Server.Run
```

这个分层是清楚的：入口只负责配置、启动和信号处理；server 负责组装；router 负责挂载；handler 负责 HTTP；service 负责业务。

### 全局中间件

`pkg/server/core.go` 创建 Gin engine 后统一加载：

- `RequestID`
- `RequestMeta`
- `AccessLog`
- `Recovery`
- `CORS`

优点是所有服务都有一致的 request id、日志和恢复能力。问题是 `RequestMeta` 在 `c.Next()` 之前读取 `c.FullPath()`，此时路由模板通常还未解析完成，`Route` 字段可能为空。位置：`internal/middleware/request_meta.go:11-20`。后续审计日志又依赖 `meta.Route`，虽然审计里做了 `c.FullPath()` fallback，但从上下文角度看，`RequestMeta.Route` 本身不可靠。

建议把 `RequestMeta` 分成两段：进入请求时写 request id、method、path、client ip；`c.Next()` 后再补 route/status/duration，或者让需要 route 的中间件直接使用 `c.FullPath()`。

## 目录结构审查

### 顶层目录

当前顶层职责清晰：

- `app/`：服务应用层，包含 `api`、`console`、`worker`
- `cmd/`：独立 CLI，目前是 `artisan`
- `internal/`：仓库内部共享基础设施
- `pkg/`：可复用基础组件
- `database/`：SQL migrations 与 seeds
- `docs/`：项目文档
- `web/admin-vben/`：管理后台前端

这个结构符合 Go 单仓后台框架的常见实践，`app/*` 和 `pkg/*` 的边界也有文档说明。

### 主要问题

1. `pkg/permission/menu_catalog.go` 固化了 `ConsoleDashboard`、`ConsoleRoles` 等 console 菜单。`pkg` 文档说“不承载具体业务语义”，但这里已经包含 console 后台业务/前端路由语义。建议迁移到 `app/console/service` 或 `app/console/internal/permission`，`pkg/permission` 只保留通用的 permission key、route collection、scope 工具。

2. `internal/model` 同时承担共享模型和 console 模型，例如 `ConsoleAdmin`、`ConsoleRole`、`SystemConfig`。如果未来只有 console-first，这可以接受；如果 `api` 或其他后台域增长，建议改成：

```text
internal/model/common
app/console/model
```

或者保留 `internal/model`，但在文档中明确“当前所有业务模型也是仓库内部共享模型”。

3. `app/console/service` 里的依赖风格不统一：`AuthService/AdminService/RoleService` 使用 `database.Repo`，`SystemConfigService/LogService` 使用 `*gorm.DB`。建议统一为 `database.Repo` 或统一用 `*gorm.DB`，否则后续事务、多数据源和测试替身会变得混乱。

4. `cmd/artisan/main.go` 已经比较大，包含命令定义、生成模板、字符串转换、文件写入。作为 CLI 入口可以先接受，但长期建议拆成 `cmd/artisan/internal/command` 与 `cmd/artisan/internal/generator`，避免继续膨胀。

## 组件逐项审查

### config

位置：`internal/config`

优点：

- 支持默认配置、yaml、环境变量占位符 `${KEY:default}`、`.env`、显式环境变量覆盖。
- `normalize` 做了较多兜底，降低缺省配置启动失败概率。

问题：

- `loadDotEnv(configDir, cfg.App.Env)` 在读取 `config.yaml` 之前执行，使用的是默认 `development`。如果配置文件里写 `app.env: production`，但没有通过环境变量提前设置 `APP_ENV`，仍会加载 `.env`。位置：`internal/config/load.go:133-165`。
- `JWT_SECRET` 默认是 `change-me`，没有生产环境强校验。生产启动时应拒绝弱 secret。
- `applyEnvironmentOverrides` 只覆盖少部分配置，和 `config.example.yaml` 的可配置项不完全一致。

建议：

- 先读取基础 yaml 得到 `app.env`，再决定是否加载 `.env`；或者仅由真实环境变量 `APP_ENV=production` 控制。
- 增加 `Validate(service string) error`，至少校验生产环境 JWT secret、数据库启用状态、Redis/job 依赖、CORS credentials 与 wildcard 冲突。
- 明确哪些配置支持 env override，避免“yaml 支持但 env 不支持”的隐性差异。

### provider

位置：`internal/provider/provider.go`

优点：

- Option 模式清楚，启动期组装显式。
- `APIOptions`、`ConsoleOptions`、`WorkerOptions` 能表达不同服务依赖。
- `Close` 集中释放数据库、Redis、job、scheduler。

问题：

- Provider 结构体有 `Cache / HTTPClient / Event / Scheduler`，也有 `WithCache / WithHTTPClient / WithEvent / WithScheduler`，但默认 `APIOptions` 和 `ConsoleOptions` 未启用。位置：`internal/provider/provider.go:42-68`。
- `WithCache` 依赖 Redis 是否存在，但 `ConsoleOptions` 没有 `WithRedis`，因此 console 即便启用 Redis，也不会有 Redis cache。
- `WithRedis` 创建 client 后没有 `Ping`，Redis 配置错误会延迟到首次使用才暴露。
- `WithStorage` 对未知 driver 是 `continue`，如果默认 disk 写错，最后可能得到空 manager 或自动切到其他 disk，启动期错误不够明确。

建议：

- 明确组件级别：核心默认装配、可选装配、实验装配。文档和 `Provider` 字段保持一致。
- 如果 `p.Cache` 是推荐能力，应把 `WithRedis` + `WithCache` 纳入 API/Console，或者从文档移除默认可用的暗示。
- Redis、Storage、Casbin 这类基础设施建议启动期快速失败，尤其生产环境。

### server

位置：`pkg/server/core.go`

优点：

- HTTP server 封装简洁，超时和 shutdown timeout 都配置化。
- `/health` 自动注册，适合部署探活。

问题：

- `Start` 在 goroutine 中 `ListenAndServe`，立即返回 `nil`，端口占用等启动失败只能通过 `logger.Fatal` 退出，调用方无法感知启动失败。位置：`pkg/server/core.go:66-73`。
- `logger.Fatal` 会直接退出进程，不利于测试和上层控制。

建议：

- 如果要保留异步启动，可以用 channel 返回启动阶段错误。
- 或提供 `Run(ctx)` 统一阻塞启动和优雅关闭，减少每个 `cmd/main.go` 的重复信号处理代码。

### middleware

位置：`internal/middleware`

优点：

- 中间件职责短小，request id、access log、recovery、cors 都是必要基础能力。

问题：

- CORS 多 origin 直接 `strings.Join` 后写入 `Access-Control-Allow-Origin`，不符合浏览器要求；该 header 只能是单个 origin 或 `*`。位置：`internal/middleware/cors.go:12-19`。
- `Access-Control-Max-Age` 写死 `"600"`，没有使用配置中的 `cfg.MaxAge`。位置：`internal/middleware/cors.go:29-30`。
- 如果 `AllowCredentials=true` 且 origin 为 `*`，浏览器会拒绝，应启动期校验或运行时按请求 Origin 回显。
- `Recovery` 没有记录 stack trace，排查 panic 成本较高。

建议：

- 改成按 `Origin` 请求头匹配 allowlist，命中后回写该 Origin，并设置 `Vary: Origin`。
- `MaxAge` 使用 `strconv.Itoa(cfg.MaxAge)`。
- Recovery 增加 `debug.Stack()`，但响应仍保持统一错误格式。

### auth

位置：`pkg/auth/token.go`

优点：

- access / refresh token 类型明确。
- Claims 同时支持 API 用户和 console admin。
- Revoke 使用 token hash，不直接保存原 token。

问题：

- blacklist 是进程内 map。位置：`pkg/auth/token.go:76-82`、`pkg/auth/token.go:160-179`。服务重启、多实例部署、滚动发布都会让已注销 token 重新有效，直到 JWT 自然过期。
- `ValidateToken` 没有限制签名算法，虽然 `jwt.ParseWithClaims` 通常会按 token header 选择算法，但最佳实践应显式检查 HMAC 方法。
- token 没有 `audience`，不同服务共享 secret 时边界较弱。

建议：

- 将 refresh token 存储为服务端会话表或 Redis key，支持轮换、吊销和多端管理。
- access token blacklist 如果需要生产有效，应接 Redis；否则把 access token 设计为短期且不可撤销，注销只撤销 refresh token。
- `ValidateToken` 中检查 `token.Method.(*jwt.SigningMethodHMAC)`。

### response / errors

位置：`pkg/response`、`pkg/errors`

优点：

- 响应格式统一，`request_id` 始终返回。
- `HTTPError` 支持 message、code、data、cause，足够覆盖后台业务。

问题：

- 顶层 `Code` 成功为 `0`，失败固定 `-1`，真实错误码放在 `data.error_code`。这对前端判断不够直观。
- `response.Fail(c, err)` 对普通 error 全部归一化为 500，handler 如果直接传 `gin` binding error，容易把参数错误变成 500；部分代码已经使用 `validation`，但 `app/api/handler/auth_handler.go` 仍直接 `ShouldBindJSON`。

建议：

- 可以保持当前格式，但建议把 `error_code` 提升为顶层字段，或者约定前端只读 `data.error_code`。
- 所有 handler 统一使用 `pkg/validation`，不要直接 `ShouldBindJSON`。

### validation

位置：`pkg/validation/request.go`

优点：

- 封装 `BindJSON / BindQuery / BindURI`，错误消息中文化。
- 支持 `label` 标签和 `Validate()` hook，handler 代码比较干净。

问题：

- 复杂度偏高，反射处理类型错误、标签解析、错误格式都在一个文件里。
- 为了兼容 typo，支持了 `lebel`，这会让错误拼写长期存在。

建议：

- 拆成 `bind.go / format.go / field_meta.go`。
- 新代码只允许 `label`，对 `lebel` 保留兼容但标注 deprecated。

### database / transaction

位置：`pkg/database`、`pkg/transaction`

优点：

- `database.Repo` 支持默认库和命名资源库，接口简洁。
- transaction manager 支持 context 中已有事务时复用，方向正确。

问题：

- `pkg/database` 当前只支持 Postgres，但 `go.mod` 间接保留 mysql/sqlserver/sqlite 等 driver，容易让使用者误以为支持多驱动。
- `transaction.GetDB(ctx, defaultDB)` 若 `defaultDB` 为 nil 会 panic。
- 当前 console 核心变更没有使用 `TxManager`，导致数据库和 Casbin 策略同步没有一致性保障。

建议：

- 要么文档明确“仅 Postgres”，要么真正补齐 driver switch。
- 在 service 中约定所有跨表或 DB+Casbin 操作必须经过 transaction manager。
- 提供 `Repo.Tx(ctx, fn)` 或统一注入 `TxManager`，减少业务层自行拼接。

### casbin / permission / route

位置：`pkg/casbin`、`pkg/permission`、`pkg/route`

优点：

- `route.Wrap(...).Name(...).Ignore()` 的体验不错，权限展示名与路由注册绑定，降低重复配置。
- `RuntimePermissionCatalog` 从 `engine.Routes()` 收集受保护路由，适合 console API 权限配置。
- Super admin 通过 role 的 `IsSuper` 短路，简单明确。

问题：

- `pkg/route` 用全局 `sync.Map` 存路由 name/scope/ignore。多 engine、多测试并发或未来多后台域时容易互相污染。
- `RuntimePermissionCatalog.LoadRoutes(engine.Routes())` 依赖路由注册顺序，目前放在最后是对的，但新增动态路由或插件式路由后要重新加载。位置：`app/console/internal/router/router.go:43-53`。
- `AdminPermission` 在 enforcer 为 nil 时直接放行，这对开发友好，但生产如果 Casbin 配置漏开，会变成无鉴权。建议生产环境快速失败，而不是请求期放行。
- `pkg/permission/menu_catalog.go` 业务语义过重，不应在 `pkg`。

建议：

- 把 route metadata 挂到 router/catalog 实例上，或者至少增加 app/service 前缀隔离。
- `AdminPermission` 增加配置策略：development 可放行，production 必须 enforcer 可用。
- console menu catalog 下沉到 console 应用层。

### storage

位置：`pkg/storage`

优点：

- Manager / Driver 抽象清楚，local 与 S3 可替换。
- 上传路径通过 UUID 重命名，避免原始文件名直接落盘。
- `buildObjectDir` 对目录做了 `path.Clean`，基本能防止 `../`。

问题：

- `LocalDriver.fullPath` 用 `strings.HasPrefix(absPath, absRoot)` 判断路径是否逃逸。位置：`pkg/storage/local.go:133-149`。例如 root 是 `/tmp/storage`，`/tmp/storage2/file` 也满足 prefix，边界判断不严谨。
- `PutFile` 对本地上传使用 `io.ReadAll`，大文件会全部进内存。位置：`pkg/storage/local.go:70-86`。
- `TemporaryURL` 只生成签名 URL，但没有看到对应的校验中间件或下载 handler，当前能力不闭环。
- `registerLocalStorageRoutes` 直接 `engine.Static(baseURL, root)` 暴露本地存储，所有文件只要知道 URL 就可访问，和 `TemporaryURL` 的存在形成语义冲突。

建议：

- 路径逃逸判断改为 `filepath.Rel(absRoot, absPath)`，检查 rel 不以 `..` 开头且不是绝对路径。
- 本地上传用 streaming copy 到临时文件，再 rename。
- 如果需要私有文件，改为受控下载接口校验签名；如果全部公开，移除或暂不暴露 `TemporaryURL`。

### cache

位置：`pkg/cache`

优点：

- Store 接口覆盖常见缓存操作。
- MemoryStore 有 TTL 和 GC，适合测试或单进程缓存。
- RedisStore 做了 prefix，避免多应用 key 冲突。

问题：

- Redis `Add` 行为是错的。`SetNX(...).Err()` 丢掉了 bool 结果，随后 `Has` 只要 key 存在就返回 true，所以 key 原本存在时也会返回 true。位置：`pkg/cache/redis.go:281-308`。
- Redis `Remember` 写入失败使用 `fmt.Printf`，绕过统一 logger。位置：`pkg/cache/redis.go:267-270`。
- MemoryStore `Close` 直接 close channel，如果被调用两次会 panic；Provider 也没有关闭 MemoryStore。
- `Get` 返回 `any` 但实际是 `[]byte`，对使用方不够直观。

建议：

- Redis `Add` 应读取 `SetNX(...).Result()` 的 bool。
- 统一使用 `pkg/logger`。
- Manager 支持关闭实现了 `Close() error` 的 store。

### httpclient

位置：`pkg/httpclient/client.go`

优点：

- 链式配置、hook、JSON/form/multipart/download 基本都有。
- `Clone` 避免 builder 污染原 client，方向不错。

问题：

- 重试复用同一个 `*http.Request`，body 在第一次请求后可能已被读取，后续重试会发送空 body 或失败。位置：`pkg/httpclient/client.go:325-351`。
- 只有 5xx 返回错误；4xx 会当作成功响应返回，是否符合预期需要明确。
- `PostMultipart` 把 `fields` 写成 query param，而不是 multipart form field。位置：`pkg/httpclient/client.go:578-592`。
- `Form` 设置了 `application/x-www-form-urlencoded`，但 `Request` 里 string body 不会自动设置 content type；因为 header 已设置所以没问题，但这个行为依赖 builder。

建议：

- 构建 request 时保存 `GetBody` 或在每次 retry 重新构造 request。
- `PostMultipart` 应设置 `builder.body = fields`。
- 明确 4xx 是否返回 error，可增加 `ThrowOnError` 配置。

### event

位置：`pkg/event/dispatcher.go`

优点：

- 同步事件简洁，panic recovery 能保护主流程。
- 异步事件有队列和 worker。

问题：

- `Close` 关闭 `stopCh` 后 worker 可能直接退出，队列中已 `wg.Add(1)` 的 job 可能无人 `Done`，`wg.Wait()` 有挂死风险。位置：`pkg/event/dispatcher.go:98-121`。
- `Config.WorkerNum` 如果为 0 且 Async=true，会创建队列但没有 worker，DispatchAsync 后 Close 一定风险很高。
- `Dispatch` 吞掉 listener 错误，仅日志记录，调用方永远拿到 nil。这适合“通知类事件”，但不适合需要阻断主流程的事件。

建议：

- Close 时关闭 queue，让 worker drain queue 后退出；或者使用 context cancellation + drain 策略。
- 校验 QueueSize / WorkerNum 默认值。
- 区分 `DispatchBestEffort` 和 `DispatchStrict`。

### scheduler

位置：`pkg/scheduler`

优点：

- 基于 robfig/cron，封装了常见周期方法。
- 支持任务级 Mutex 防重叠。

问题：

- `Register` 未校验 task、name、job 是否为空，传错会 panic 或注册无效任务。
- `Stop` 会等待 cron 当前任务结束，长任务可能拖住关闭；这本身合理，但需要文档说明。
- Provider 有 `WithScheduler`，默认服务没有启用。

建议：

- 增加参数校验。
- 明确 scheduler 是单进程定时器，多实例需要外部锁。

### job / worker

位置：`pkg/job`、`app/worker`

优点：

- asynq 封装轻量，任务类型和 payload 放在 `pkg/job/tasks.go`，示例清楚。
- worker disabled 时会记录“任务服务未启用”，不会崩溃。

问题：

- 当前只注册 echo 示例任务；任务注册未来多了以后，`app/worker/handler/default_job.go` 会膨胀。
- Job client/server 都依赖 Redis，但 `WithRedis` 和 `WithJobServer` 各自创建连接或配置，没有统一 ping/健康检查。

建议：

- 按业务模块拆 task registrar，例如 `handler.RegisterConsoleJobs`。
- 增加 `/health` 等价的 worker doctor 或 `artisan doctor` 深度检查 Redis/asynq。

### migrate / artisan

位置：`pkg/migrate`、`cmd/artisan`

优点：

- SQL migration 简单可控，适合基础模板。
- `make:module` 自动生成 model/service/handler 并注册路由，提升一致性。

问题：

- migration 只按目录文件名顺序执行，没有 checksum。已执行迁移文件被改动时无法发现。
- seed 每次跑全量 SQL，没有 seed 记录表；重复执行是否安全完全依赖 SQL 自身。
- 代码生成没有同步生成 migration、前端页面、菜单 key，文档已说明，但实际使用者仍容易遗漏。

建议：

- schema_migrations 加 `checksum` 和 `execution_time_ms`。
- seeds 分为 idempotent seed 和 one-shot seed。
- `make:module` 输出后续 checklist，或者生成占位 migration。

## Console 业务组织审查

### 优点

- handler/service 分层清晰，handler 基本只做绑定、调用 service、响应。
- DTO 命名直观，例如 `CreateRoleRequest`、`ListRolesInput`、`RoleDetail`。
- 操作日志通过中间件统一记录，业务 handler 只补充 audit meta。
- API 权限和菜单权限分离，符合 README 中的设计原则。
- `RuntimePermissionCatalog` 用运行时路由作为 API 权限真相源，减少手工维护。

### 主要问题

1. DB 与 Casbin 策略不同步风险。

`AdminService.CreateAdmin` 先创建管理员，再调用 `syncAdminRoleBinding`。位置：`app/console/service/admin.go:220-226`。如果 DB 创建成功但 Casbin 绑定失败，会返回错误，但管理员已经存在。

`AdminService.UpdateAdmin` 先更新 DB，再同步 Casbin。位置：`app/console/service/admin.go:301-312`。失败时角色字段和 Casbin grouping policy 可能不一致。

`RoleService.DeleteRole` 先删除角色，再删除策略。位置：`app/console/service/role.go:307-317`。策略删除失败时角色已消失但策略残留。

`RoleService.SetRolePermissions` 先删除旧策略，再添加新策略。位置：`app/console/service/role.go:353-370`。添加失败时角色会失去全部 API 权限。

建议：这些操作必须放进同一个数据库事务中。Casbin gorm adapter 也基于同一 DB 时，可以考虑直接操作 adapter 表或使用事务内 enforcer/adapter；如果做不到强事务，至少要设计补偿和重试。

2. 生产环境权限降级过于宽松。

`AdminPermission` 在 enforcer nil 时放行。开发期可以接受，但生产应启动失败。否则 Casbin 配置漏开就是后台无接口权限控制。

3. 超级管理员保护过于硬编码。

目前多处判断 `HasSuperAccess()` 后禁止修改/删除。这是对的，但依赖 `Role.IsSuper` 预加载成功。应保证所有相关路径都 Preload Role，并在 DB 层对 root 角色和 root admin 做不可删除约束或 seed 约束。

4. 列表分页逻辑重复。

`AdminService`、`RoleService`、`SystemConfigService`、`LogService` 都有类似查询、排序、时间范围、分页代码。当前重复还能接受，但框架模板继续扩展 CRUD 后会快速复制。建议抽一个小的 `ListQuery` helper，不要引入重型 Repository。

5. 审计日志记录结果偏粗。

`AuditOperation` 对失败统一使用 `http.StatusText(status)`，业务错误消息没有写入。可以在 `response.Fail` 中把错误摘要写入 `reqctx.AuditMeta` 或 gin context，让审计日志能看到具体业务失败原因。

## API 业务组织审查

`app/api` 当前更像示例链路：

- `/auth/access-token` 签发测试 token
- `/ping`
- `/profile`
- `/jobs/echo`

优点是能展示认证、数据库、任务队列、request meta 的完整用法。

问题是 `/auth/access-token` 默认 `api-user`，没有任何凭证校验，只适合作为 scaffold/demo。作为基础项目模板，应明确标记为开发示例，生产默认关闭或移除。

## 测试覆盖审查

当前 Go 文件约 114 个，测试文件约 27 个。测试覆盖了不少基础组件：

- config
- provider
- server
- middleware recovery
- auth
- response
- validation
- database
- transaction
- cache
- event
- scheduler
- storage manager
- migrate
- job
- api/console router
- console role service

这是一个不错的起点。缺口主要在：

- CORS 行为测试不足。
- RedisStore 真实语义没有覆盖，`Add` bug 没被测出来。
- HTTP client retry body 复用、multipart fields 没有被测出来。
- LocalDriver 路径边界没有覆盖相似前缀目录。
- AdminService 的 DB+Casbin 一致性没有测试。
- Auth blacklist 多实例/重启问题无法通过单元测试发现，需要设计层面处理。

## 优先级建议

### P0：先修真实 bug 和生产安全边界

1. 修复 Redis `Add` 返回值。
2. 修复 HTTP client retry 重建请求体。
3. 修复 `PostMultipart` form fields。
4. 修复 CORS origin / max age / credentials 行为。
5. 修复 LocalDriver 路径边界判断。
6. 生产环境禁止默认 JWT secret。
7. 生产环境 Casbin enforcer nil 不允许放行。

### P1：统一框架能力和依赖风格

1. 决定 `Cache / Event / Scheduler / HTTPClient` 是否默认装配，并同步文档。
2. 统一 console service 依赖为 `database.Repo + TxManager` 或 `*gorm.DB`。
3. 将 console 菜单 catalog 从 `pkg/permission` 下沉到 console 应用层。
4. 给 DB+Casbin 写事务或补偿机制。

### P2：提升可维护性

1. 拆分 `cmd/artisan/main.go`。
2. 拆分 `pkg/validation/request.go`。
3. 抽轻量列表查询 helper。
4. route metadata 从全局 map 迁移为实例级 catalog。
5. migration 增加 checksum。

### P3：补文档和测试

1. 增加“生产启动检查”文档。
2. 增加“新增组件如何接入 Provider”的文档。
3. 增加 Redis、HTTP client、CORS、storage path、AdminService 权限同步测试。
4. 明确 `api` 示例接口哪些可用于生产，哪些必须删除或关闭。

## 建议的演进路线

第一阶段：框架可靠性修复

- 聚焦 P0 bug。
- 不做目录大调整，避免影响面过大。
- 每个 bug 都补对应单元测试。

第二阶段：Provider 与权限一致性

- 明确默认装配组件。
- 引入生产配置校验。
- 改造 Admin/Role 权限同步事务。

第三阶段：目录边界清理

- 下沉 console menu catalog。
- 统一 service 依赖风格。
- 拆分 artisan 和 validation。

第四阶段：模板体验完善

- `make:module` 生成更完整的 checklist 或 migration 占位。
- 完善 docs/guide 中“新增模块后的权限、菜单、前端页面、迁移”闭环。

## 总体评价

这个项目的骨架是合理的：`app` 承载服务应用，`internal` 承载仓库内部基础设施，`pkg` 承载可复用组件，`console` 作为主线后台具备真实可用的登录、RBAC、配置、上传和日志能力。

当前最大问题不是“大方向错”，而是基础框架进入生产前常见的细节债：默认配置校验、组件装配一致性、跨组件事务、可选能力与文档一致性、以及少数实现 bug。把 P0/P1 处理掉后，这个项目可以作为比较稳的 Go 后台起步模板继续扩展。
