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

## 5. 明确不做

- 不拆微服务、不引入插件系统。
- 不做 ServiceContainer / Facade / 全局 helper。
- 不引入通用 Repository 层或继承式领域模型。
- 不做一次性全仓重命名或大重写。
- i18n 暂缓：当前只有中文一种语言，等真出现第二语言再做。

## 6. 实施记录

| 日期 | 任务 | 结果 |
| --- | --- | --- |
| 2026-08-30 | 上一轮全部任务 | 已完成，见第 1 节 |
| 2026-09-04 | T1 `pkg/server` → `internal/server` | `891540e`。`pkg/` 非测试代码已无 `internal/` 依赖；`pkg/job/job_test.go` 仍引 `internal/observability`，属测试专用，不影响外部消费，暂留。 |
| 2026-09-04 | T2 route catalog 单一化 | `a0ac0ff`。删全局 `sync.Map` / `ResetForTest` / 4 组 `*WithCatalog` 双份实现，净删 139 行。 |
| 2026-09-04 | T3 去 `WithDeps` 后缀 | 纯重命名，11 个注册函数。 |
| 2026-09-04 | T4 Casbin 定时重载 | `85cac4c`。复用 `SyncedEnforcer.StartAutoLoadPolicy`，无新依赖；`auto_load_seconds` 默认 30，Provider 关闭时停 goroutine。代价：变更最多延迟一个间隔。 |
| 2026-09-04 | T5 Scheduler 集群互斥 | `643d1a7`。复用 `pkg/cache.Store` 的 SETNX，无新依赖；Redis 启用时 Mutex 任务全局互斥，未启用时行为不变（仍限单 Worker）。释放为 Get+Delete 比对，非原子 CAS。 |
