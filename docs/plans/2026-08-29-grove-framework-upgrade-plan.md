# Grove 框架升级计划

> 事实来源：当前 checkout 的源码、测试与命令输出。与本文冲突时以代码为准。
> 上次核对：2026-09-04。

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
go build ./...   exit 0
go vet ./...     exit 0
gofmt -l         空
go test ./...    exit 0（42 包 ok / 8 包无测试）
make contracts   PASS（路由↔OpenAPI、前端↔OpenAPI 双向）
```

规模：197 个 Go 文件 / 33,295 行 / 26 个 migration（postgres + mysql 各一份）。

`pkg/` 已有 22 个组件：`auth` `rbac` `permission` `validation` `request` `response` `errx` `storage` `logger` `cache` `event` `job` `scheduler` `database` `migrate` `transaction` `ratelimit` `secretbox` `httpclient` `route` `ulid` `server`。

## 3. 剩余问题

| # | 问题 | 位置 | 影响 |
| --- | --- | --- | --- |
| 1 | 缺 Mail / Notification | 全仓无 smtp 相关代码 | 注册、找回密码、告警无法交付 |
| 2 | 业务纵深薄 | `app/api` 仅 starter+auth，`app/worker` 仅 default_job | 未验证「新增一个功能要改几处」 |
| 3 | 定时任务调度写死在代码 | `pkg/scheduler` 仅支持代码内注册 | 改 cron 表达式要重新部署，见 Phase 5 |

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
| T10 | 补 `pkg/request`、`app/api/service` 等 8 个无测试包 | `go test ./...` 无 `no test files` |

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

- [ ] **S7 文档**
  - `docs/guide/scheduler.md` 增加「后台管理」一节：代码/DB 各管什么、为什么不能后台建任务
  - 验收：`make docs.check` 通过

#### 本阶段不做

- 任务内容存 DB、运行时解释脚本或 shell 命令。
- Kafka / 独立执行器。
- 完整执行历史表——先用行上的 `last_*` 字段；真要排查多次失败再单开 `console_scheduled_task_runs`。

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
