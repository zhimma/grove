# Grove 框架升级计划

> 事实来源：当前 checkout 的源码、测试与命令输出。与本文冲突时以代码为准。
> 上次核对：2026-09-29。

**Goal:** 把 Grove 从「可运行的模块化单体脚手架」收敛为可持续开发的 Go Web 框架基线——分层边界干净、多实例可部署、常用组件齐备。

**Architecture:** 保留模块化单体与显式依赖注入。启动入口负责配置与生命周期；handler 只做 HTTP 适配；service 负责业务流程与事务；model 负责共享模型；`pkg/` 只放不依赖本仓库业务的通用能力，`internal/` 放本仓库装配代码。借鉴 Laravel 的能力分类与开发体验，不复制 Facade、运行时容器、通用 Repository。

---

## 1. 已完成（2026-08-30 `cb3cba8` / `3725645`）

上一轮计划已全部落地，逐条回查源码确认：

| 项 | 证据 |
| --- | --- |
| 配置模板移除固定凭据 | `config.example.yaml` 密码/secret 均为 `''` |
| JWT 加固 | `pkg/auth/token.go:224` HS256 pin + `WithIssuer` + audience + subject + iat 必填 |
| Console 权限 fail-closed | `app/console/internal/middleware/admin_auth.go:145` enforcer 缺失返回 503 |
| API 强制 RBAC | `app/api/internal/router/router.go` `permissionSet.RequireRoute()` |
| 存储默认私有 | `app/console/internal/server/server.go:58` `!disk.Public \|\| !disk.ServeStatic` |
| Scheduler panic 隔离 | `pkg/scheduler/scheduler.go:127` `cron.Recover` + `:215` recover |
| 装配层显式注入依赖 | `app/console/internal/router/router.go` 传 db/enforcer/pagePolicy/catalog，不再透传 Provider |
| 日志前后端契约对齐 | 前后端均为 3 个只读接口 |
| 分页配置生效 | `router.go:32` `NewPagePolicy(cfg.API.DefaultPerPage, cfg.API.MaxPerPage)` |
| 旧 log service 清理 | `operation_log.go` / `login_log.go` 已删除 |

## 2. 当前实测基线

```
go build ./...     exit 0
go test ./...      exit 0（45 包 ok / 7 包无测试）
go vet ./...       exit 0
make quality       exit 0（fmt、any、password、vet、docs、diff、前端 lint 与循环依赖）
make contracts     PASS（路由↔OpenAPI、前端↔OpenAPI 双向）
前端单测            45 文件 / 328 测试
```

规模：211 个 Go 文件 / 36,007 行 / 14 个迁移（postgres 与 mysql 各一份，均含 down）。

`pkg/` 22 个基础层组件：`auth` `cache` `database` `errx` `event` `httpclient` `job` `logger` `migrate` `password` `permission` `ratelimit` `rbac` `request` `response` `route` `scheduler` `secretbox` `storage` `transaction` `ulid` `validation`。

`server` 已于 T1 移入 `internal/`；`password` 由 D1 新增。

## 3. 剩余问题

| # | 问题 | 位置 | 影响 |
| --- | --- | --- | --- |
| 1 | 缺 Mail / Notification | 全仓无 smtp 相关代码 | 注册、找回密码、告警无法交付。按 YAGNI 等真实触发 |
| 2 | 业务纵深薄 | `app/api` 仅 starter+auth，`app/worker` 仅 echo + 会话清理 | 未验证「新增一个功能要改几处」 |
| 3 | 迁移未在真实库执行 | 本机无 Docker，`tests/integration/` 处于 skip | 需在 CI 或有 Docker 的机器确认 up/down |

## 4. 任务

### Phase 1 — 分层收口

| ID | 任务 | 验收 |
| --- | --- | --- |
| ~~T1~~ | ~~`pkg/server` → `internal/server`~~ | ✅ `891540e` |
| ~~T2~~ | ~~删 `pkg/route` 全局状态 + 合并 `*WithCatalog` 双轨~~ | ✅ `a0ac0ff` |
| ~~T3~~ | ~~清理 `RegisterXxxRoutesWithDeps` 后缀~~ | ✅ 本节第 3 行 |

### Phase 2 — 多实例能力

| ID | 任务 | 验收 |
| --- | --- | --- |
| ~~T4~~ | ~~Casbin 策略定时重载~~ | ✅ `85cac4c` |
| ~~T5~~ | ~~Scheduler 集群互斥~~ | ✅ `643d1a7` |

### Phase 3 — 补组件

| ID | 任务 | 验收 |
| --- | --- | --- |
| T6 | `pkg/mail`：接口 + SMTP 驱动 + log 驱动（dev），配置进 `config.yaml` | 单测覆盖发送失败与超时 |
| T7 | `pkg/notify`：站内信 + 邮件，复用 `pkg/event` | 单测覆盖多通道分发 |
| T8 | `internal/testkit`：模型工厂 + `httptest` 助手 | 至少替换 3 处重复 fixture |

### Phase 4 — 纵深验证

| ID | 任务 | 验收 |
| --- | --- | --- |
| T9 | 用一个真实模块走完 `grove make:module` 全流程，记录改动点数量 | 产出改动清单，决定是否需要再收敛 |
| ~~T10~~ | ~~补无测试的基础包~~ | ✅ Phase 6 D2/D3。剩余 7 个是 3 个 `cmd`（main 包）、api/worker 示例模块、`pkg/ulid`（13 行），均不值得为覆盖率硬测 |

### Phase 5 — 计划任务后台管理

参考 `xinliangnote/go-gin-api` 的 `internal/{api,services}/cron`：DB 存调度参数 + 后台 CRUD + 手动触发。

**但不照抄它的执行模型。** 它的 `AddJob` 只打一条日志，注释写着"生产环境应写入 Kafka 由执行器订阅"——把任务内容存进 DB 再运行时解释这条路它自己没走通，而且那等于开一个远程命令执行口子。

Grove 的切法：

| 归属 | 内容 | 理由 |
| --- | --- | --- |
| 代码 | 任务名 → handler 函数 | 编译期确定，可测试，无法注入 |
| DB | cron 表达式、启停、超时、互斥、上次执行结果 | 改调度不必重新部署 |
| Console | 改表达式、启停、手动触发、看上次结果 | 运维自助 |

关键约束：**Console 不能新建任务**。行由 Worker 按代码注册表补齐，Console 只能改已存在的行。任务名对不上代码注册表就没有 handler，也就不存在"从后台注入一个任务"的路径。

#### 任务清单

- [x] **S1 数据模型与迁移** — `internal/model/console_scheduled_task.go` + `202604150014` 双方言迁移
  - `console_scheduled_tasks`：`name`(唯一) / `schedule` / `enabled` / `mutex` / `timeout_seconds` / `run_requested_at` / `last_run_at` / `last_status` / `last_error` / `last_duration_ms`
  - postgres + mysql 双份迁移，含 down
  - 验收：`pkg/migrate` 配对/方言/「schedule-only」守卫测试通过；集成测试断言已补但**本机 Docker 不可用，未真实执行 up/down**

- [x] **S2 Worker 任务注册表** — `app/worker/internal/task`
  - `Definitions(dbs) map[string]Definition`，编译期确定；名称重复、无 Job、非法 cron 均在构建时报错
  - 真实任务：`console.purge-expired-sessions` 清理过期后台会话（分批删除，保留已吊销但未过期的行）
  - 顺带：`scheduler.ValidateSchedule` 抽为共用校验器，Console 保存前用同一个解析器，避免"后台存得进、Worker 跑不了"
  - 验收：5 个单测通过（注册表可运行性、缺数据库报错、只删过期、跨批清空、取消 context 中止）

- [x] **S3 Worker reconcile 循环** — `app/worker/internal/task/reconciler.go`
  - 每轮先补齐缺失的行，再让 Scheduler 与表对齐；已有行不覆盖，运维改过的调度在重新部署后仍保留
  - `schedule`/`mutex`/`timeout` 变了才 `Remove`+`Register`；`enabled=false` 移除；表达式解析失败只跳过该行
  - 执行结果经 `instrument` 包装写回 `last_*`；写回用独立 context，超时被取消的任务仍能记录失败
  - 默认间隔 30s，与 T4 casbin 重载同一节奏
  - 无数据库时降级：Scheduler 仍跑代码内注册的任务，只是无法在后台管理（Worker 会告警）
  - 验收：14 个单测通过；「保留运维改动」「停用即移除」两项做过变异验证，改坏实现后确实变红
  - 已知缺口：代码中已删除的任务留下的行，Console 看不出「无 handler」与「从未执行」的区别，仅 Worker 日志告警

- [x] **S4 手动触发** — `runOnRequest` / `claimRunRequest` / `declineRunRequest`
  - Console 写 `run_requested_at`；Worker 用条件 UPDATE 抢占，`RowsAffected == 1` 的那个才执行，多副本下只跑一次
  - 走 `scheduler.Run`，因此手动执行同样受 mutex、集群锁、超时和 panic 隔离保护
  - 跑不了的请求也要回答：任务已停用或代码中不存在时，清空标记并写 `skipped` + 原因，不会永远显示"待执行"
  - 新增 `scheduler.ErrTaskNotFound` sentinel，用于区分"未注册"和任务自身失败
  - 验收：17 个单测通过；抢占逻辑做过变异验证（去掉 `RowsAffected` 检查后 4 个 worker 全部抢到，测试变红）
  - 上限：触发延迟最多一个对账周期（30s）。要即时需改走 asynq，Console 得加 `WithJob()`

- [x] **S5 Console 接口** — `handler/scheduled_task.go` + `service/scheduled_task.go`
  - 4 个接口：列表 / 改调度 / 启停 / 手动触发。**没有新建和删除**，行由 Worker 按代码注册表补齐
  - 保存前用 `scheduler.ValidateSchedule` 校验，与 Worker 的 cron 同一个解析器
  - 停用的任务拒绝手动触发；已有待执行请求时拒绝重复提交
  - 权限走 route catalog `.Name("计划任务.xxx")`；OpenAPI 已登记
  - 验收：8 个 service 单测 + `make contracts` 通过
  - 修掉一个模型缺陷：`gorm:"default:true"` 会让 `false` 在 INSERT 时被省略，`Mutex: false` 的任务定义会被静默存成 `true`。已去掉标签并加回归测试（把标签加回去测试会红）

- [x] **S6 前端页面** — `views/system/scheduled-task/` + `api/scheduled-task.ts`
  - 列表 + 编辑弹窗（表达式/互斥/超时）+ 启停 + 手动触发 + 上次执行结果（状态标签、耗时、错误摘要）
  - 页面顶部说明"任务内容在代码中定义，此处只能调整执行时机"，并提示对账延迟
  - 已有待执行请求或任务停用时，"执行一次"按钮禁用
  - 路由登记在 `router/routes/modules/system.ts`（不放 `log.ts`，那是日志模块）
  - 验收：`admin.typecheck` / `admin.lint` / `admin.circular` / `admin.build` 通过；前端单测 326 个（新增 4 个 API 契约测试）
  - 契约门禁做过变异验证：改坏 `console-contract.json` 里的路径后 `make contracts` 变红

- [x] **S7 文档**
  - `docs/guide/scheduler.md` 新增「后台管理」：注册方式、代码/DB 职责划分、为什么不能后台建任务、对账生效时机、边界
  - `docs/guide/structure.md` 补 `app/worker/internal/task` 的位置与去向指引
  - 验收：`make docs.check` 通过

#### 本阶段不做

- 任务内容存 DB、运行时解释脚本或 shell 命令。
- Kafka / 独立执行器。
- 完整执行历史表——先用行上的 `last_*` 字段；真要排查多次失败再单开 `console_scheduled_task_runs`。

### Phase 6 — fork 质量治理

产品形态定为 **fork / clone**：新项目整仓 fork 后自持，不做库化、不承诺 API 稳定、不提供升级通道。

这个决定让一批问题直接失效，不再追：版本标签与 CHANGELOG、`pkg/` 的 doc.go 与 Example、`pkg/` 的 gin/gorm 解绑、升级机制。剩下的唯一产品是「你 fork 到手的这份代码」。

核心判断：**问题里有一半是文档在说谎，不是代码有病。** `pkg/` 声称"不承载业务语义"却塞满 `UserTypeConsole`、`CheckConsolePermission`；`pkg/logger` 的全局单例对应用是惯例、只对发布库才是反模式。这类问题改声明比改代码便宜，也更诚实。

| ID | 任务 | 验收 |
| --- | --- | --- |
| ~~A1~~ | ~~清掉文档里的个人机器路径~~ | ✅ `0392ce8` |
| ~~A2~~ | ~~codegen 从目标仓库 go.mod 读 module path~~ | ✅ `fdc811b` |
| ~~A3~~ | ~~dashboard 解除示例表依赖~~ | ✅ 前提有误，见实施记录，不改代码 |
| ~~A4~~ | ~~新增 fork 指南~~ | ✅ `3df8766` |
| ~~B1~~ | ~~删 `errx` 的坏 sentinel~~ | ✅ `12dfcba` |
| ~~B2~~ | ~~`response.Fail` 改为接受 `error`~~ | ✅ `12dfcba` |
| ~~B3~~ | ~~删重复的响应别名~~ | ✅ `12dfcba` |
| ~~C1~~ | ~~修正 `pkg/` 定位声明~~ | ✅ `afeefeb` |
| ~~C2~~ | ~~删单实现接口~~ | ✅ `53ddd8f` |
| ~~C3~~ | ~~构造函数统一 `New(Config)`~~ | ✅ `3abce2b` |
| ~~C4~~ | ~~`interface{}` → `any`~~ | ✅ `c002031` |
| ~~C5~~ | ~~统一 `/system` 前端归属~~ | ✅ `a8c76a5` |
| ~~D1~~ | ~~抽出 `pkg/password`~~ | ✅ `a0af363` |
| ~~D2~~ | ~~补 `pkg/request` 测试~~ | ✅ `88aae4b` |
| ~~D3~~ | ~~补 route / permission / errx 测试~~ | ✅ `34dda5a` |
| ~~D4~~ | ~~工具链可复现~~ | ✅ `5adf9fb` |

#### 本轮新增的 6 个守卫

每个都做过变异验证（改坏后确认变红），都挂在 `make quality` 或 `make docs.check` 下：

| 守卫 | 拦截什么 |
| --- | --- |
| 文档个人路径 | 文档里出现 `/Users/...` 或 `/home/...` |
| 文档教已删 API | `response.OK(`、`httpclient.NewWithConfig(`、`event.NewDispatcher(` 等 |
| `/system` 路由归属 | 出现第二个 `/system` 父路由或游离的 `/system/*` |
| `interface{}` 回流 | 非测试代码出现 `interface{}` |
| bcrypt 越界 | 非测试代码绕过 `pkg/password` 直接用 bcrypt |
| 计划任务 schedule-only | 迁移里出现 command/script/payload 之类的列 |

### Phase 7 — 对标评估路线图（2026-09-29）

**对照原则：Laravel 只借「要有哪些能力」，Go 原生框架决定「怎么实现」。** 不参考任何 Java 系脚手架的设计（注解/AOP、BaseService 继承、Repository/DAO 分层、部门岗位数据权限）。

| 对照组 | 借什么 | 不借什么 |
| --- | --- | --- |
| Laravel | 能力清单与开发闭环：`make:model -mcr`、`key:generate`、`paginate()`、`throttle`、daily 日志、RefreshDatabase | Facade、服务容器、Eloquent 魔法方法、模型观察者 |
| go-zero | goctl「一份描述 → 全栈代码」、内置限流、ServiceContext 显式依赖 | 微服务注册发现（Grove 是单体） |
| Huma | OpenAPI 与代码不能漂移 | 替换 gin handler 签名（Grove 已有契约测试兜底） |
| Goravel | 仅作能力覆盖参照 | 它的 Facade 风格，正是要避免的 |
| nunu | lumberjack 日志轮转、`run` 热重载、docker-compose 起本地依赖 | `google/wire`（2025-08-25 已归档）；`XxxService` 接口 + 唯一实现 + `*Service` 基类嵌入；Repository 层；gomock 与独立 `test/` 目录 |
| go-gin-api | 生成器意识、限流、pprof | 连真实 MySQL 读表生成（凭据进 shell 历史、仅 MySQL）；handler 以带 `i()` 标记的接口声明 |
| GoFrame | `gf run` 热重载、一条命令出代码 | 全局 `g.DB()` / `g.Log()` 单例 |
| Rails / Phoenix（思想） | `scaffold Post title:string`：命令行字段直接生成迁移、模型、控制器与测试 | — |

#### 实测结论

| 结论 | 证据 |
| --- | --- |
| **生成器与门禁互相矛盾** | 临时 worktree 执行 `grove make:module Invoice` 后 `make contracts` 立即失败：`missing OpenAPI operations: GET /console/v1/invoices`。且生成的 service 只返回「模块已就绪」，没有迁移、没有 CRUD |
| 日志永不轮转 | `pkg/logger` 以 `O_APPEND` 写 `./logs/<service>.log`，无切割、无保留期，部署文档未提 |
| 分页锁在 console 内部 | `PagePolicy`/`ListMeta` 在 `app/console/internal/service`，api 服务无法复用；`ListMeta` 在 handler 与 service 各定义一份；构造靠 `policies []PagePolicy` 可变参数冒充可选参数 |
| 三个 app 结构不一致 | `app/api` 的 handler/service/middleware 在 `internal/` 外，console 与 worker 在里面 |
| 测试夹具重复 | 23 个测试文件各自 `sqlite.Open`，6 个同构的建库函数 |
| Java 味残留 | `pkg/request` 12 个 `Get` 前缀访问器（Effective Go 不推荐）；`auth`/`cache`/`migrate`/`storage` 四个 `Manager`；`database.NewConnections` 返回单实现接口 |

#### 任务（按优先级）

| 优先级 | ID | 任务 | 验收方向 | 阻塞 |
| --- | --- | --- | --- | --- |
| ~~P0~~ | ~~G1~~ | ~~`make:module --fields` 生成可用纵向切片（后端）~~ | ✅ 双方言迁移 + 模型 + 分页 CRUD + 自带测试 + handler + OpenAPI；回归测试在仓库副本里生成后跑 `go vet`、契约、生成的 CRUD 测试、方言规则，三处变异均被抓到。顺带修复 `grove migrate create` 只建单方言的问题 | — |
| ~~P0~~ | ~~G1b~~ | ~~生成前端：`console-contract.json` 条目 + `api/*.ts` + 基于 `resource-page` 的页面 + 路由~~ | ✅ 在真实仓库生成 Invoice 后 `admin.typecheck` / `quality`（含 `admin.lint`）/ `admin.test` / `admin.build` / `contracts` / 全量 `go test` 通过；回归测试校验前端只调用已登记的 operation，契约 JSON 往返逐字节不变，两处变异均被抓到。time 字段改为与响应同格式的字符串入参，`""` 清空。顺带修 `resource-page` 编辑时沿用上一条记录 `omitempty` 字段的旧值、清空日期提交 `null` 被后端忽略两个问题 | — |
| ~~P1~~ | ~~G2~~ | ~~日志轮转：按大小切割 + 保留期，对应 Laravel daily channel，做法同 nunu~~ | ✅ lumberjack v2.2.1；`log.max_size_mb`（默认 100）/ `max_age_days`（默认 14，`0` 不清理），显式非法值启动即拒绝；启动时即打开文件，坏目录不会拖到第一条日志才暴露。轮转与急切打开各做一次变异验证。未被读取的 `log.service` 保留为兼容字段（严格解码下删掉会让旧配置启动失败） | — |
| ~~P1~~ | ~~G3~~ | ~~分页下沉到 `pkg/`，api 与 console 共用；去掉 `[]PagePolicy` 可变参数~~ | ✅ `pkg/pagination`：`Policy`（零值可用）/ `Request` / `Page.Apply` / `Meta`，三份分页结构与两份 `ListMeta` 合一；8 个列表 service 的构造函数改为显式 `pages pagination.Policy`，`NewRoleServiceWithPolicy` 删除；router 只建一份 policy 与一份 `SessionService`。重构前后 OpenAPI 逐字节一致。顺带修复：计划任务 `list_all` 生成 `LIMIT 0` 返回空列表（加回归测试并变异验证）、用户列表从未接收配置的分页上限 | — |
| ~~P1~~ | ~~G4~~ | ~~`internal/testkit`：`OpenDB(t, models...)` 等，替换重复夹具~~ | ✅ `OpenDB`（每测独立、`t.Cleanup` 关闭，原先无一处关闭连接）+ `CreateCasbinTable`（按迁移后的 `NOT NULL DEFAULT ''` 与唯一索引建表，原先 6 份手写 DDL 有两种形状）；app/cmd/internal 下 26 处建库与 6 份 DDL 全部收敛，生成器模板同步。`pkg/*` 测试因依赖方向保留自建 | — |
| ~~P2~~ | ~~G5~~ | ~~`app/api` 结构与 console/worker 对齐~~ | ✅ handler/service/middleware 移入 `app/api/internal/`，去掉 `auth_handler.go`、`starter_service.go` 这类与包名重复的后缀；`structure.md` 原先描述的目录结构与 console 实际不符，一并改正 | — |
| P2 | G6 | 去 Java 味：`request.GetAdminID` → `request.AdminID` 等；`database.Connections` 返回具体类型；`Manager` 改为表意名 | 行为不变，调用点机械替换 | 改名面较大，可分批 |
| P2 | G7 | `grove key:generate`：生成 `jwt.secret`、`config_encryption_key` 等强密钥 | fork 后无需手工造密钥 | 无 |
| P3 | G8 | 通用限流中间件（go-zero 内置、Laravel `throttle`），复用现有 `x/time/rate` 与 Redis | 按 IP/用户限流，429 走统一错误信封 | 等 api 服务有公开接口 |
| P3 | G9 | `pkg/mail` | — | 等真实触发 |
| P3 | G10 | `make dev` 热重载（对应 nunu `run`、`gf run`），用 `go run` 固定版本的 air，不进 go.mod | 改代码后服务自动重启 | 无 |
| P3 | G11 | docker-compose 起本地 PostgreSQL + Redis | `make deps.up` 后 quickstart 直接可跑 | 本机无 Docker，只能静态校验 |

#### 本阶段不做

- Facade、服务容器、注解/AOP 式横切（幂等、脱敏、字段翻译装饰器）。
- Repository/DAO 层、BaseService 继承、DTO/VO/BO 分层。
- 部门/岗位/数据权限/多租户、字典表——需要下拉选项时用代码枚举或 `system_configs`。
- 迁移到 Huma 或 `log/slog`——现有契约测试与 zerolog 已够用，迁移成本不抵收益。
- `google/wire` 编译期注入——已归档；Grove 的手写 Options 装配在当前规模下更直观。
- 读库生成代码（go-gin-api gormgen、GoFrame `gf gen dao`）——需要活数据库与方言自省，不适合双方言与 CI。
- swag 注释生成文档——Grove 用代码声明 OpenAPI 并双向契约测试，更不易漂移。

## 5. 明确不做

- 不拆微服务、不引入插件系统。
- 不做 ServiceContainer / Facade / 全局 helper。
- 不引入通用 Repository 层或继承式领域模型。
- 不做一次性全仓重命名或大重写。
- i18n 暂缓：当前只有中文一种语言，等真出现第二语言再做。

### 对照 go-gin-api / nunu 后确认不引入

2026-09-04 逐项比对 `xinliangnote/go-gin-api` 与 `huluxiaobao-nunu/console-api`，除 Phase 5 外均判定不抄：

| 能力 | 对方实现 | Grove 现状 | 结论 |
| --- | --- | --- | --- |
| ID 生成 | nunu：sonyflake + base62 | `pkg/ulid` | 不换。sonyflake 需协调 machine ID，容器里易冲突；ULID 128 位、字典序即时间序、无需协调 |
| ID 混淆 | nunu：`SafeID` Blowfish + base58 | ULID | 不做。ULID 本就不泄露自增序号；nunu 那个是为兼容老系统 `bind_key` 的历史包袱 |
| trace | go-gin-api：自研 `pkg/trace` | OpenTelemetry | 不换 |
| 错误码 | go-gin-api：`pkg/errors` | `pkg/errx` | 已有 |
| 优雅关闭 | go-gin-api：`pkg/shutdown` | `internal/server` | 已有 |
| 路由白名单 | go-gin-api：`pkg/urltable` | route Catalog | 已有 |
| 加解密 | go-gin-api：`aes`/`rsa`/`hash`；nunu：blowfish | `pkg/secretbox` | 已有 |
| 文件 | go-gin-api：`pkg/file` | `pkg/storage` | 已有且更完整 |
| 时间工具 | go-gin-api：`pkg/timeutil` | stdlib | 不需要 |
| API 签名验签 | go-gin-api：`pkg/signature` | 无 | 当前无需求，不做 |
| 多租户过滤 | nunu：`scope.Apply` | 无 | 无多租户需求，不做组件；但其 fail-closed 原则（空范围 → `WHERE 1=0` 而非不过滤）已记入权限文档待办 |

## 6. 实施记录

| 日期 | 任务 | 结果 |
| --- | --- | --- |
| 2026-08-30 | 上一轮全部任务 | 已完成，见第 1 节 |
| 2026-09-04 | T1 `pkg/server` → `internal/server` | `891540e`。`pkg/` 非测试代码已无 `internal/` 依赖；`pkg/job/job_test.go` 仍引 `internal/observability`，属测试专用，不影响外部消费，暂留。 |
| 2026-09-04 | T2 route catalog 单一化 | `a0ac0ff`。删全局 `sync.Map` / `ResetForTest` / 4 组 `*WithCatalog` 双份实现，净删 139 行。 |
| 2026-09-04 | T3 去 `WithDeps` 后缀 | 纯重命名，11 个注册函数。 |
| 2026-09-04 | T4 Casbin 定时重载 | `85cac4c`。复用 `SyncedEnforcer.StartAutoLoadPolicy`，无新依赖；`auto_load_seconds` 默认 30，Provider 关闭时停 goroutine。代价：变更最多延迟一个间隔。 |
| 2026-09-04 | T5 Scheduler 集群互斥 | `643d1a7`。复用 `pkg/cache.Store` 的 SETNX，无新依赖；Redis 启用时 Mutex 任务全局互斥，未启用时行为不变（仍限单 Worker）。释放为 Get+Delete 比对，非原子 CAS。 |
| 2026-09-04 | Phase 5 计划任务后台管理（S1–S7） | `b7e687f` `87fd273` `2419ecc` `901ab35` `1677cd0` `66d38ed`。改调度不再需要重新部署。过程中由测试抓出两个真实缺陷：孤儿行的 `run_requested_at` 永远清不掉；GORM `default:true` 标签使 `Mutex: false` 被静默存成 `true`。四处关键逻辑做过变异验证。**迁移未在真实 PostgreSQL/MySQL 执行**（本机无 Docker），集成断言处于 skip。 |
| 2026-09-29 | Phase 6 fork 质量治理（A/B/C/D） | 13 个提交 `0392ce8`…`5adf9fb`。详见下方「被代码推翻的判断」。全门禁绿：`build`/`test`/`vet`/`quality`/`contracts`/`docs.check` 与前端 typecheck/lint/circular/test/build。 |

### 被代码推翻的判断

Phase 6 的计划里有 6 条经不起查证，均以代码为准修正：

| 计划原本写的 | 查证结果 |
| --- | --- |
| `dashboard.go` 依赖示例表 `users`，要解耦 | `users` 是框架能力（355 行 CRUD 的 C 端客户管理），`console_admins` 才是运营者。真正的示例只有 `articles` 和 `starter`。**不改代码** |
| 删 `Success`/`Error` 别名，保留 `OK`/`Fail` | `OK` 与 `Error` 各 **0 调用**，`Success` 59 处、`Fail` 151 处。方向反了，改为删 `OK`/`Error` |
| unexport `logger.InitForTest` | 被 `internal/bootstrap` 的测试跨包使用，不能 unexport |
| 去掉 `zerolog.SetGlobalLevel` | 它是**唯一**应用 `log.level` 的地方，删掉配置直接失效。**此项会造成 regression** |
| `ratelimit.NewLoginGuard` 应返回具体类型 | 它按有无 Redis 在两个实现间选，返回接口是正确的工厂 |
| 在 `.mise.toml` 里钉 pnpm | 本机 `~/Library/pnpm` 在 PATH 中优先，钉了也不生效；且与 `packageManager` 构成两个真相源。改为 Makefile 走 corepack |

### 删除死代码时的连锁发现

删掉 `transaction.Manager` 后变异验证**没有变红**，查下去发现 `txKey` 的唯一写入方就是被删的 `Manager.Execute`，因此 `FromContext` 永远返回 nil、`GetDB` 的第一个分支不可达——测试覆盖的是一条走不到的路径。清理后该包从 86 行降到 33 行，两个测试才真正可被变异打红。

同类情况：`SetIdentity` 把身份镜像进 std context，但 `GetIdentityFromContext` 零消费方，而正是这个镜像让 setter 解引用 `c.Request`，在 `gin.CreateTestContext` 给的 context 上 panic。

### 覆盖率变化

| 包 | 改前 | 改后 |
| --- | --- | --- |
| `pkg/request` | 0% | 97.0% |
| `pkg/permission` | 33.9% | 96.6% |
| `pkg/route` | 54.8% | 94.5% |
| `pkg/errx` | 43.9% | 80.6% |
| `pkg/password` | 新增 | 90.0% |
