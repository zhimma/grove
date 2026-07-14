# Grove Foundation Hardening and Framework Roadmap Implementation Plan

> **For Claude:** Use an executing-plans workflow to implement this plan task-by-task.

**Goal:** 将 Grove 建设为具备可靠初始化、安全后台、稳定基础组件、清晰 Go 代码规则和持续验证能力的中大型模块化单体脚手架。

**Architecture:** 保留 `api / console / worker` 三入口和 `handler / service / model` 主分层，依赖只在启动装配层集中创建，业务构造函数接收精确依赖。借鉴 Laravel 的开发体验和常用能力，但使用 Go 的显式组合、小接口、直接命名和按需抽象，不引入运行时服务容器、通用 Repository、BaseService 或注解式魔法。

**Tech Stack:** Go 1.25、Gin、GORM、PostgreSQL、Redis、Casbin、Asynq、Vue 3、Vben Admin、pnpm、GitHub Actions、Testcontainers、OpenTelemetry。

---

## 1. 执行规则

### 状态定义

- `[ ] Planned`：已记录，尚未开始。
- `[-] In Progress`：正在开发，同一时间每个执行者只保留一个进行中任务。
- `[x] Completed`：实现、测试、文档和复核全部完成。
- `[!] Blocked`：存在明确阻塞，必须记录阻塞原因和解除条件。
- `[~] Deferred`：低优先级或等待真实业务需求，不进入当前迭代。

每个任务开始时填写：

```text
Status:
Owner:
Branch/PR:
Started at:
Completed at:
Blocker:
```

### 实施原则

- 每个任务先写失败测试，再做最小实现。
- 每个任务独立提交；不要把多个不相关问题放进同一提交。
- 不顺手重构任务范围外的代码。
- 共享组件必须先定义稳定语义，再补便捷 API。
- 简单 CRUD 不增加额外层；复杂状态、渠道或策略变化才使用接口和设计模式。
- `internal/provider.Provider` 只允许出现在启动、server、router 装配层。
- service、handler、job 不得接收完整 Provider，只接收实际依赖。
- 新依赖必须说明替代的自研代码或解决的明确问题。
- Phase 5 的扩展组件默认不实现，满足触发条件后再转为 Planned。

### 每个任务的完成标准

- 失败测试已添加并确认失败原因正确。
- 最小实现完成。
- 目标包测试通过。
- 相关 race 测试通过。
- 变更文件重新阅读，导入、错误路径、资源关闭正确。
- 用户可见行为、配置或命令变化已更新文档。
- `git diff --check` 通过。
- 对应提交已创建。

### 里程碑统一验证

```bash
go test ./...
go test -race ./...
go vet ./...
make build
pnpm --dir web/admin-vben --filter @grove/console typecheck
pnpm --dir web/admin-vben build:console
git diff --check
```

预期：全部命令退出码为 `0`。如果前端依赖未安装，先执行：

```bash
pnpm --dir web/admin-vben install --frozen-lockfile
```

---

## 2. 里程碑总览

### Milestone 1：初始化与安全阻断项

- [x] Task 1：修复 CLI 名称和文档漂移。
- [x] Task 2：补齐数据库 schema 和可回滚迁移。
- [x] Task 3：拆分安全 bootstrap seed 与 demo seed。
- [x] Task 4：替换迁移引擎并使用真实 PostgreSQL 验证生命周期。
- [x] Task 5：修复 `make:module` 生成代码和原子性。
- [x] Task 6：明确软删除语义并增加数据库约束。
- [x] Task 7：隔离 API 演示接口和虚拟数据。

完成条件：全新环境可以按文档初始化；重复 seed 不改变管理员密码；生成模块可直接编译；数据库删除语义明确。

**Checkpoint 2026-07-14:** Task 1–7 已完成；`go test ./...`、`go vet ./...`、`make build` 通过。`go test -race ./...` 暴露 `pkg/scheduler.TestScheduler_Mutex` 共享计数器竞争，归入 Task 15。前端依赖未安装，按用户要求未执行下载，typecheck/build 待本地依赖可用后补跑。

### Milestone 2：认证、授权与输入安全

- [x] Task 8：实现可持久化 Console Session 和 refresh token 轮换。
- [x] Task 9：增加登录限流、失败锁定与可信代理配置。
- [x] Task 10：增加请求体和文件上传限制。
- [x] Task 11：修复系统配置敏感值和审计泄漏。
- [x] Task 12：修复 GORM 与 Casbin 的一致性边界。

完成条件：多实例下退出和 refresh 语义一致；登录入口可防暴力尝试；上传不能绕过服务端限制；敏感配置不出现在 API 和审计日志中。

### Milestone 3：基础组件契约稳定化

- [ ] Task 13：重构 Cache 契约和生命周期。
- [ ] Task 14：重构 HTTP Client 为请求级不可变状态。
- [ ] Task 15：修复 Scheduler 并发、配置和取消语义。
- [ ] Task 16：修复 Event 异步投递语义。
- [ ] Task 17：统一 Provider 生命周期和按服务配置校验。

完成条件：所有共享组件有一致返回语义、明确错误、并发安全和关闭路径；`go test -race ./...` 通过。

### Milestone 4：代码组织与开发体验

- [ ] Task 18：消除后端静态菜单真相源。
- [ ] Task 19：清理未使用的全局单例 API 和命名。
- [ ] Task 20：收敛重复分页和响应映射。
- [ ] Task 21：完善 OpenAPI 合同和漂移检查。
- [ ] Task 22：增加 readiness、指标、trace 和安全 CI。
- [ ] Task 23：建立前端自定义代码测试基线。

完成条件：新增模块路径清晰；权限、文档和路由不会静默漂移；运行状态可观测；前后端关键自定义逻辑有自动化测试。

### Milestone 5：按需扩展组件

- [~] Task 24：分布式锁和幂等组件。
- [~] Task 25：Transactional Outbox。
- [~] Task 26：通知渠道组件。
- [~] Task 27：Webhook 管理组件。
- [~] Task 28：CSV/Excel 导入导出。
- [~] Task 29：数据权限、工作流、Feature Flag 等领域增强。

完成条件：仅当触发条件成立后，将对应任务改为 Planned 并补充独立设计文档。

---

## 3. Milestone 1：初始化与安全阻断项

### Task 1：修复 CLI 名称和文档漂移

**Status:** `[x] Completed`

**Owner:** Codex

**Branch/PR:** `codex/grove-foundation-roadmap`

**Started at:** 2026-07-13

**Completed at:** 2026-07-13

**Verification:**

- `go test ./cmd/grove -run TestRepositoryCommandsUseGroveCLI -v`：PASS。
- `go test ./cmd/grove -v`：PASS。
- `go run ./cmd/grove about`：PASS。
- `make migrate.status`：已进入数据库配置检查，因默认数据库未启用退出；未再访问旧 `cmd/artisan` 路径。

**Files:**

- Modify: `Makefile`
- Modify: `README.md`
- Modify: `docs/guide/quickstart.md`
- Modify: `docs/guide/database.md`
- Modify: `docs/deployment/deploy.md`
- Test: `cmd/grove/main_test.go`

**Step 1：增加仓库命令契约测试**

在 `cmd/grove/main_test.go` 增加测试，读取根目录 `Makefile` 和 `README.md`，断言：

```go
assertContains(t, makefile, "GROVE := $(GO) run ./cmd/grove")
assertNotContains(t, makefile, "cmd/artisan")
assertContains(t, readme, "go run ./cmd/grove")
assertNotContains(t, readme, "cmd/artisan")
```

**Step 2：运行测试并确认失败**

```bash
go test ./cmd/grove -run 'TestRepositoryCommandsUseGroveCLI' -v
```

预期：FAIL，输出仍包含 `cmd/artisan`。

**Step 3：统一 CLI 命名**

- 将 Makefile 变量 `ARTISAN` 改为 `GROVE`。
- 所有迁移和 seed target 使用 `$(GROVE)`。
- README 目录、命令和能力描述统一为 `grove`。
- 删除全部 `artisan` 历史名称。

**Step 4：验证真实命令**

```bash
go test ./cmd/grove -v
go run ./cmd/grove about
make migrate.status
```

预期：前两项 PASS；`make migrate.status` 可以进入配置/数据库检查，不再报文件不存在。

**Step 5：提交**

```bash
git add Makefile README.md docs cmd/grove/main_test.go
git commit -m "fix: unify grove cli commands"
```

### Task 2：补齐数据库 schema 和可回滚迁移

**Status:** `[x] Completed`

**Owner:** Codex

**Branch/PR:** `codex/grove-foundation-roadmap`

**Started at:** 2026-07-13

**Completed at:** 2026-07-13

**Architecture note:** 003 是已发布且已包含完整管理字段的基线迁移，不改写历史文件。004 down 只回滚 004 实际改变的 email 可空性；不删除 003 已定义的列和索引。

**Verification:**

- `go test ./pkg/migrate -v`：PASS。
- `go test -race ./pkg/migrate -v`：PASS。
- 真实 PostgreSQL 的 up/down 生命周期验证归入 Task 4。

**Depends on:** Task 1

**Files:**

- Create: `database/migrations/<timestamp>_create_system_configs.up.sql`
- Create: `database/migrations/<timestamp>_create_system_configs.down.sql`
- Create: `database/migrations/202604150004_expand_console_management.down.sql`
- Review: `database/migrations/202604150003_create_console_tables.up.sql`
- Review: `database/migrations/202604150003_create_console_tables.down.sql`
- Test: `pkg/migrate/migrate_test.go`

**Step 1：增加迁移文件完整性测试**

扫描所有 `.up.sql`，要求存在同名 `.down.sql`：

```go
func TestEveryUpMigrationHasDownMigration(t *testing.T) {
    // list .up.sql and assert matching .down.sql exists
}
```

**Step 2：运行并确认失败**

```bash
go test ./pkg/migrate -run TestEveryUpMigrationHasDownMigration -v
```

预期：FAIL，指出 `202604150004_expand_console_management.down.sql` 缺失。

**Step 3：创建 system_configs 迁移**

字段必须与 `internal/model/system_config.go` 一致，并包含：

- ULID 字符串主键。
- `(config_group, config_key)` 唯一索引。
- `value_type`、`value`、`default_value`。
- `is_editable`、`is_system`、`sort_order`。
- 时间字段和明确的删除字段语义。

**Step 4：补写 004 down**

只回滚 004 引入的 schema 变化，不删除 003 已创建的基础列。若 003 当前已包含 004 的最终列，先整理迁移演进关系，保证空库顺序执行和历史库升级都正确。

**Step 5：验证 SQL 文件对称性**

```bash
go test ./pkg/migrate -v
```

预期：PASS。

**Step 6：提交**

```bash
git add database/migrations pkg/migrate/migrate_test.go
git commit -m "fix: complete database migration chain"
```

### Task 3：拆分安全 bootstrap seed 与 demo seed

**Status:** `[x] Completed`

**Owner:** Codex

**Branch/PR:** `codex/grove-foundation-roadmap`

**Started at:** 2026-07-13

**Completed at:** 2026-07-13

**Architecture note:** bootstrap 与 demo 使用显式目录和命令分界。bootstrap 在单个数据库事务中执行，root 密码由 CLI 注入 bcrypt 占位符；只有随机密码实际创建了 root 时才输出一次。管理员自行修改密码后清除 `must_change_password`，后台创建或重置密码时重新置为 `true`。

**Verification:**

- `go test ./cmd/grove -v`：PASS。
- `go test ./pkg/migrate -v`：PASS。
- `go test ./app/console/internal/service -run TestChangePasswordClearsMustChangePassword -v`：PASS。
- `go test ./...`：PASS。
- `go test -race ./cmd/grove ./pkg/migrate ./app/console/internal/service`：PASS。
- `go vet ./cmd/grove ./pkg/migrate ./app/console/internal/service`：PASS。
- production 环境执行 `go run ./cmd/grove seed demo`：按预期拒绝。
- 真实 PostgreSQL 下重复 bootstrap 不改密码的集成验证归入 Task 4。

**Depends on:** Task 2

**Files:**

- Create: `database/seeds/bootstrap/`
- Create: `database/seeds/demo/`
- Delete: `database/seeds/*.sql` 旧扁平 seed
- Create: `database/migrations/202604150008_add_console_admin_password_state.*.sql`
- Modify: `cmd/grove/main.go`
- Modify: `pkg/migrate/migrate.go`
- Modify: `internal/model/console_admin.go`
- Modify: `app/console/internal/service/auth.go`
- Modify: `app/console/internal/service/admin.go`
- Modify: `app/console/internal/handler/auth.go`
- Modify: `app/console/internal/handler/admin.go`
- Modify: `Makefile`
- Modify: `README.md`
- Modify: `docs/guide/quickstart.md`
- Modify: `docs/deployment/deploy.md`
- Modify: `config.example.yaml`
- Modify: `.env.example`
- Test: `cmd/grove/main_test.go`
- Test: `pkg/migrate/migrate_test.go`
- Test: `app/console/internal/service/auth_password_test.go`

**Step 1：定义命令行为测试**

要求 CLI 支持：

```text
grove seed bootstrap
grove seed demo
```

并要求 production 环境拒绝执行 `seed demo`。

**Step 2：增加密码不覆盖测试**

使用真实 PostgreSQL 集成测试或 seed SQL 内容测试，验证重复执行 bootstrap 后管理员 password 字段保持原值。

**Step 3：修改 bootstrap seed**

- Root 账号使用 `ON CONFLICT DO NOTHING`，不得更新 password。
- 初始密码从 `GROVE_ROOT_PASSWORD` 读取并在 CLI 中生成 bcrypt hash。
- 未提供初始密码时生成一次性随机密码，仅打印一次，不写日志文件。
- 增加 `must_change_password` 字段或等价状态。

**Step 4：移动 demo 数据**

- `api-user`、demo admin、示例权限进入 `database/seeds/demo/`。
- production 环境拒绝 demo seed。

**Step 5：验证**

```bash
go test ./cmd/grove -v
```

预期：PASS。

**Step 6：提交**

```bash
git add database/seeds cmd/grove config.example.yaml .env.example
git commit -m "fix: separate bootstrap and demo seeds"
```

### Task 4：替换迁移引擎并使用真实 PostgreSQL 验证 migrate/seed/down

**Status:** `[x] Completed`

**Owner:** Codex

**Branch/PR:** `codex/grove-foundation-roadmap`

**Started at:** 2026-07-13

**Completed at:** 2026-07-14

**Architecture note:** 使用 `golang-migrate` 负责版本状态、dirty 状态和 PostgreSQL advisory lock；Grove 只保留 CLI 薄封装。迁移元数据表使用 `grove_migrations`，避免与旧版自研 `schema_migrations(name, applied_at)` 结构冲突。迁移源目录通过 `migrate --path` 配置。

**Verification:**

- `go mod verify`：PASS。
- `go test ./...`：PASS。
- `go test -tags=integration ./tests/integration -run TestFreshDatabaseLifecycle -v`：测试编译通过；本机 Docker 镜像拉取受网络限制时明确 Skip/失败，不伪造通过。
- 本机 PostgreSQL `127.0.0.1` 临时数据库真实验证：up 8 个迁移、dirty 拒绝、重复 bootstrap 不覆盖密码、down 至业务表为空，全部 PASS。
- CI 已接入 `go test -tags=integration ./tests/integration -v`，在有 Docker 的 runner 上强制执行 Testcontainers 生命周期测试。

**Depends on:** Task 2, Task 3

**Files:**

- Modify: `go.mod`
- Modify: `go.sum`
- Modify: `pkg/migrate/migrate.go`
- Modify: `cmd/grove/main.go`
- Create: `tests/integration/migration_test.go`
- Modify: `.github/workflows/ci.yml`
- Modify: `docs/development/testing.md`

**Step 1：引入成熟迁移引擎和 Testcontainers PostgreSQL 模块**

```bash
go get github.com/golang-migrate/migrate/v4
go get github.com/testcontainers/testcontainers-go/modules/postgres
```

使用 `golang-migrate` 替换自研的版本执行和 down 状态管理，保留 Grove CLI 作为薄封装。必须保留：

- `grove migrate up/down/status/create` 命令体验。
- migration source path 配置。
- dirty 状态错误输出。
- 并发执行保护由数据库 driver 负责。
- 不在 Grove 内重复实现迁移锁和版本状态机。

**Step 2：编写空库迁移测试**

测试流程：

```text
start PostgreSQL
→ migrate up
→ assert expected tables and indexes
→ seed bootstrap
→ change root password
→ seed bootstrap again
→ assert password unchanged
→ migrate down to empty database
```

**Step 3：运行并确认初始失败**

```bash
go test -tags=integration ./tests/integration -run TestFreshDatabaseLifecycle -v
```

**Step 4：修复迁移和 seed 直到测试通过**

不得在测试中使用 GORM `AutoMigrate` 替代生产迁移。

**Step 5：接入 CI**

CI 增加：

```bash
go test -tags=integration ./tests/integration -v
```

**Step 6：提交**

```bash
git add go.mod go.sum tests .github docs/development/testing.md
git commit -m "test: verify postgres bootstrap lifecycle"
```

### Task 5：修复 `make:module` 生成代码和原子性

**Status:** `[x] Completed`

**Owner:** Codex

**Branch/PR:** `codex/grove-foundation-roadmap`

**Started at:** 2026-07-14

**Completed at:** 2026-07-14

**Architecture note:** 模板和命名逻辑从 CLI 命令文件拆出；`make:module` 在写入前完成名称、全部目标文件和 router marker 预检。生成源码先经 `go/format`，新文件使用同目录临时文件和原子 hard link 创建，router 原子替换失败时回滚本次生成文件。

**Verification:**

- `go test ./cmd/grove -v`：PASS。
- 临时同模块工作区执行生成后 `go test` 编译 model/service/handler/router：PASS。
- `go test ./...`：PASS。
- `go test -race ./cmd/grove`：PASS。
- `go vet ./cmd/grove`：PASS。
- 搜索旧 `app/console/service`、`app/console/handler`、`app/console/router` 模板路径：无残留。

**Files:**

- Modify: `cmd/grove/main.go`
- Split: `cmd/grove/templates.go`
- Split: `cmd/grove/naming.go`
- Test: `cmd/grove/main_test.go`

**Step 1：增加生成结果编译测试**

在临时 Go module 中运行 `make:module ProductCategory`，随后执行：

```bash
go test ./...
```

预期当前失败，错误指向不存在的 `app/console/service`。

**Step 2：修复模板路径**

生成代码必须导入：

```go
consoleservice "github.com/zhimma/grove/app/console/internal/service"
```

**Step 3：增加预检阶段**

写文件前先检查：

- model、service、handler 目标文件都不存在。
- router marker 存在。
- 名称转换结果非空且是合法 Go identifier。

任一失败时不得创建任何文件。

**Step 4：使用临时文件原子写入**

所有内容准备成功后再 rename 到目标位置；router 修改失败时清理本次新文件。

**Step 5：格式化和编译生成结果**

执行 `go/format`，测试中运行生成 module 的 `go test ./...`。

**Step 6：提交**

```bash
git add cmd/grove
git commit -m "fix: make module generation atomic and compilable"
```

### Task 6：明确软删除语义并增加数据库约束

**Status:** `[x] Completed`

**Owner:** Codex

**Branch/PR:** `codex/grove-foundation-roadmap`

**Started at:** 2026-07-14

**Completed at:** 2026-07-14

**Architecture note:** 业务实体统一使用 GORM 原生软删除；登录与操作审计日志使用不含删除字段的 `AuditBase`，删除即物理删除。业务唯一键只约束未删除记录，数据库同时负责稳定枚举、角色引用和 Casbin 规则完整性。

**Verification:**

- `go test ./internal/model ./pkg/migrate ./app/console/internal/service -v`：PASS。
- `go test -race ./internal/model ./pkg/migrate ./app/console/internal/service`：PASS。
- `go test -tags=integration ./tests/integration -run '^$'`：PASS，集成测试可编译。
- 本机 PostgreSQL 17.7 临时库：9 个迁移 up、约束实际拒绝、软删除唯一值复用、bootstrap 幂等、009 down/up、全量 down 全部 PASS，临时库已删除。
- `go test ./...`、`go vet ./...`、`git diff --check`：PASS。

**Depends on:** Task 4

**Recommended decision:** 业务实体使用软删除；审计日志使用物理删除或归档策略。不要让所有模型共享同一种删除语义。

**Files:**

- Modify: `internal/model/base.go`
- Create: `internal/model/audit_base.go`
- Modify: `internal/model/console_admin.go`
- Modify: `internal/model/console_role.go`
- Modify: `internal/model/system_config.go`
- Modify: `internal/model/console_login_log.go`
- Modify: `internal/model/console_operation_log.go`
- Create: new schema constraint migration pair
- Test: service tests and integration migration tests

**Step 1：增加删除语义测试**

- 删除管理员后普通查询不可见。
- `Unscoped` 可以查询管理员。
- 删除审计日志后数据库中记录不存在。

**Step 2：拆分模型基础结构**

```go
type Base struct {
    ID        string
    CreatedAt time.Time
    UpdatedAt time.Time
    DeletedAt gorm.DeletedAt
}

type AuditBase struct {
    ID        string
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

**Step 3：增加数据库约束**

- `console_admins.role_id` 引用 `console_roles.id`，删除策略为 RESTRICT。
- status、value_type 等稳定枚举增加 CHECK 约束。
- Casbin rule 增加与 adapter 兼容的组合唯一索引。

**Step 4：验证**

```bash
go test ./app/console/internal/service ./internal/model -v
go test -tags=integration ./tests/integration -v
```

**Step 5：提交**

```bash
git add internal/model database/migrations app/console/internal/service tests
git commit -m "fix: define deletion and database integrity semantics"
```

### Task 7：隔离 API 演示接口和虚拟数据

**Status:** `[x] Completed`

**Owner:** Codex

**Branch/PR:** `codex/grove-foundation-roadmap`

**Started at:** 2026-07-14

**Completed at:** 2026-07-14

**Architecture note:** 使用显式 `demo.enabled` 和独立 demo 路由注册函数。演示端点默认关闭，production 即使误设为 true 也不注册；正式模块生成路由标记保持在 demo 条件之外。

**Verification:**

- `go test ./app/api/... ./internal/config ./internal/model -v`：PASS。
- `go test -race ./app/api/... ./internal/config ./internal/model`：PASS。
- production 且 `demo.enabled=true`：`/ping`、`/auth/access-token`、`/profile`、`/jobs/echo` 全部返回 404。
- `APP_ENV=development DEMO_ENABLED=false go run ./cmd/grove --config config.example.yaml about`：PASS。
- `go test ./...`、`go vet ./...`、`git diff --check`：PASS。

**Files:**

- Create: `app/api/internal/router/demo.go`
- Modify: `app/api/internal/router/router.go`
- Modify: `app/api/handler/auth_handler.go`
- Modify: `app/api/service/auth_service.go`
- Modify: `app/api/handler/starter_handler.go`
- Modify: `app/api/service/starter_service.go`
- Modify: `app/api/internal/docs/docs.go`
- Modify: `internal/model/user.go`
- Modify: `internal/config/types.go`
- Modify: `internal/config/load.go`
- Modify: `config.example.yaml`
- Test: router、docs、config、model tests

**Step 1：增加 production 禁用测试**

生产配置下请求 `/api/v1/auth/access-token` 必须返回 404。

**Step 2：增加配置**

```yaml
demo:
  enabled: false
```

默认必须为 false。

**Step 3：移动演示实现**

- ping、任意 token、虚拟用户、echo job 归入 examples 或 demo router。
- 正式 API 不允许数据库缺失时伪造用户。

**Step 4：验证**

```bash
go test ./app/api/... -v
```

**Step 5：提交**

```bash
git add app/api internal examples config.example.yaml
git commit -m "fix: isolate demo api behavior"
```

---

## 4. Milestone 2：认证、授权与输入安全

### Task 8：实现持久化 Console Session 和 refresh token 轮换

**Status:** `[x] Completed`

**Owner:** Codex

**Branch/PR:** `codex/grove-foundation-roadmap`

**Started at:** 2026-07-14

**Completed at:** 2026-07-14

**Design:** `docs/plans/2026-07-14-console-session-design.md`

**Verification:**

- `go test ./...`：PASS。
- `go test -race ./pkg/auth ./app/console/internal/service ./app/console/internal/middleware ./app/console/internal/router`：PASS。
- `go vet ./...`、`make build`、`git diff --check`：PASS。
- 本机 PostgreSQL 17.7：010 up/down、约束与索引、bootstrap、全量 down 全部 PASS。
- 本机 PostgreSQL 17.7：8 个并发请求复用同一个 refresh token，恰好 1 个成功，其余返回 `invalid_refresh_token`。
- 前端 `vue-tsc --noEmit --skipLibCheck`：PASS。
- 前端 production build 已进入 Rollup，因离线安装被终止后其它 Vben workspace 包未生成 stub 而阻塞；新增页面没有类型错误，未继续联网恢复整个工作区。

**Depends on:** Task 4

**Architecture:** access token 保持短期 JWT；refresh token 对应服务端 session 记录。数据库保存 session 元数据和 refresh token hash，Redis 可作为加速但不是唯一真相源。

**Files:**

- Create: `internal/model/console_session.go`
- Create: migration pair for `console_sessions`
- Create: `app/console/internal/service/session.go`
- Modify: `pkg/auth/token.go`
- Modify: `app/console/internal/service/auth.go`
- Modify: `app/console/internal/handler/auth.go`
- Create: Console session management handlers and frontend page
- Test: auth service and router tests

**Required behavior:**

- refresh token 只保存 SHA-256 hash。
- refresh 成功后旧 session token 原子失效。
- 并发使用同一 refresh token 只能成功一次。
- logout、强制下线、修改密码可以撤销 session。
- 服务重启和多实例不恢复已撤销 token。
- session 记录设备、IP、最后活跃时间和过期时间。

**Verification:**

```bash
go test ./pkg/auth ./app/console/internal/service ./app/console/internal/router -race -v
```

**Commit:**

```bash
git commit -m "feat: add persistent console sessions"
```

### Task 9：增加登录限流、失败锁定与可信代理配置

**Status:** `[x] Completed`

**Owner:** Codex

**Branch/PR:** `codex/grove-foundation-roadmap`

**Started at:** 2026-07-14

**Completed at:** 2026-07-14

**Design:** `docs/plans/2026-07-14-login-protection-design.md`

**Verification:**

- `go test -race ./internal/middleware ./internal/bootstrap ./pkg/server ./pkg/ratelimit ./app/console/internal/service`：PASS。
- `go test ./...`、`go vet ./...`、`make build`、`git diff --check`：PASS。
- 本机临时 Redis：两个独立 `RedisLoginGuard` 实例共享请求限流、失败计数和锁定状态，测试 PASS；临时进程和数据目录已删除。
- 本机 PostgreSQL 一次性测试库：8 路并发 refresh 恰好 1 次成功，集成回归 PASS；测试库已删除。
- production CORS、可信代理、非法 Request ID、安全响应头、登录错误语义、失败锁定、成功重置和账号/IP 隔离均有自动化测试。

**Files:**

- Create: `internal/middleware/security_headers.go`
- Create: `pkg/ratelimit/login.go`
- Create: `pkg/ratelimit/login_redis.go`
- Modify: `internal/config/types.go`
- Modify: `internal/config/load.go`
- Modify: `pkg/server/core.go`
- Modify: `internal/bootstrap/middleware.go`
- Modify: `app/console/internal/handler/auth.go`
- Modify: `app/console/internal/service/auth.go`
- Test: config、middleware、server、ratelimit 和 auth tests

**Required behavior:**

- 单机默认实现使用 `golang.org/x/time/rate`。
- Redis 启用时支持跨实例登录限流。
- 限流 key 使用规范化账号 + 可信客户端 IP。
- 连续失败达到阈值后短期锁定，不直接永久修改管理员 status。
- 成功登录清理失败计数。
- `trusted_proxies` 必须显式配置；production 不允许默认信任所有代理。
- `X-Request-Id` 限制长度和字符集，非法值重新生成。
- production CORS 不允许默认 `*`。
- 增加 `X-Content-Type-Options`、`X-Frame-Options`、`Referrer-Policy` 等基础安全响应头；HSTS 仅在明确 HTTPS 部署时启用。

**Required verification command:**

```bash
go test ./internal/middleware ./pkg/ratelimit ./app/console/internal/service -race -v
```

### Task 10：增加请求体和文件上传限制

**Status:** `[x] Completed`

**Owner:** Codex

**Branch/PR:** `codex/grove-foundation-roadmap`

**Started at:** 2026-07-14

**Completed at:** 2026-07-14

**Design:** `docs/plans/2026-07-14-upload-security-design.md`

**Verification:**

- `go test -race ./internal/middleware ./internal/bootstrap ./pkg/storage ./pkg/validation ./app/console/internal/service ./app/console/internal/router ./app/console/internal/server`：PASS。
- `go test ./...`、`go vet ./...`、`make build`、`git diff --check`：PASS。
- Console `vue-tsc --noEmit --skipLibCheck`：PASS，直接使用现有离线 module cache，未安装或下载依赖。
- 前端变更文件与 package/lockfile 使用本地 Prettier 校验：PASS。
- 路由测试覆盖 document 成功上传、策略目录、主动内容伪装拒绝、策略大小 413 和未知长度请求体 413。
- Local 驱动覆盖流式写入、内容一致性、读失败时临时文件与目标文件清理；静态文件响应 `nosniff`。

**Files:**

- Create: `internal/middleware/body_limit.go`
- Create: `pkg/storage/upload_policy.go`
- Modify: `app/console/internal/handler/storage.go`
- Modify: `pkg/storage/local.go`
- Modify: `pkg/storage/s3.go`
- Modify: `web/admin-vben/apps/console/src/components/upload/FileUpload.vue`
- Delete: `web/admin-vben/apps/console/src/utils/storage/` 中 COS、OSS、未实现 S3 适配器和工厂残留
- Modify: `web/admin-vben/apps/console/src/api/core/file.ts`
- Modify: `internal/config/types.go`
- Modify: `internal/bootstrap/middleware.go`
- Modify: `internal/provider/provider.go`
- Modify: `config.example.yaml`
- Test: config、middleware、validation、storage、server、router 和 Vue typecheck

**Required behavior:**

- 使用 `http.MaxBytesReader` 限制请求体。
- 服务端验证文件大小、扩展名、MIME 和 magic bytes。
- 本地存储使用流式 `io.Copy`，不得 `io.ReadAll` 整个文件。
- 上传策略按用途命名，例如 `avatar`、`document`，不使用一个无限制全局策略。
- SVG、HTML、脚本类文件默认拒绝公开托管。
- 本地公开文件增加 `X-Content-Type-Options: nosniff`。
- 前端只消费后端返回的 `local / s3` ClientConfig；删除当前默认 COS、未实现 OSS/S3 等与后端协议冲突的残留适配器。
- local 使用服务端 multipart 上传；S3 STS 实现后再启用前端直传。

**Required verification command:**

```bash
go test ./pkg/storage ./app/console/internal/router -v
```

### Task 11：修复系统配置敏感值和审计泄漏

**Status:** `[x] Completed`

**Owner:** Codex

**Branch/PR:** `codex/grove-foundation-roadmap`

**Started at:** 2026-07-14

**Completed at:** 2026-07-14

**Design:** `docs/plans/2026-07-14-system-config-secrets-design.md`

**Verification:**

- `go test ./...`、`go vet ./...`、`make build`、`git diff --check`：PASS。
- `go test -race ./pkg/secretbox ./pkg/migrate ./app/console/internal/service ./app/console/internal/router ./app/console/internal/middleware`：PASS。
- Console `vue-tsc --noEmit --skipLibCheck` 与前端变更文件 Prettier：PASS，使用现有离线 module cache，未下载依赖。
- 本机 PostgreSQL 唯一临时库：011 up/down、`is_secret` 列生命周期、存在敏感配置时拒绝 down 且迁移保持 `dirty=false`、清理敏感记录后可正常 down，全部 PASS；临时库已删除。
- service/router 测试覆盖 AES-256-GCM 随机 nonce、密文防篡改、API 掩码、`keep_secret`、默认值解密、基础设施 secret 拒绝和审计脱敏。

**Files:**

- Modify: `internal/model/system_config.go`
- Create: migration pair adding `is_secret`
- Modify: `app/console/internal/service/system_config.go`
- Modify: `app/console/internal/handler/system_config.go`
- Modify: frontend system config pages
- Test: service, router and audit tests

**Required behavior:**

- 增加 `is_secret`。
- secret 值使用应用密钥加密后存储。
- API 列表和详情只返回掩码值。
- 更新 secret 时允许“保持原值”。
- 操作日志只记录配置 key 和 `changed=true`，禁止记录明文值。
- 明确禁止将数据库密码、JWT 主密钥等基础设施 secret 放入业务配置表。

### Task 12：修复 GORM 与 Casbin 的一致性边界

**Status:** `[x] Completed`

**Owner:** Codex

**Branch/PR:** `codex/grove-foundation-roadmap`

**Started at:** 2026-07-14

**Completed at:** 2026-07-14

**Design:** `docs/plans/2026-07-14-rbac-consistency-design.md`

**Verification:**

- `go test ./...`、`go vet ./...`、`make build`、`git diff --check`：PASS。
- `go test -race ./pkg/rbac ./cmd/grove ./app/console/internal/service ./app/console/internal/router`：PASS。
- SQLite trigger 故障注入：新 policy/grouping 写入失败时 adapter 事务回滚，旧集合和内存模型保持不变。
- service 测试覆盖管理员创建补偿、换角色 fail-closed 与恢复、删除前清理、角色删除失败恢复旧权限。
- 本机 PostgreSQL 唯一临时库：`rbac check` 识别 grouping 错位、孤儿 grouping 和孤儿 policy；默认 repair 未修改数据；`--dry-run=false` 修复后 check PASS；临时库已删除。

**Depends on:** Task 6

**Recommended decision:** 不伪装成单事务。角色和管理员数据由数据库事务提交，Casbin 关系更新使用明确的同步步骤、失败补偿和一致性检查命令。

**Files:**

- Modify: `app/console/internal/service/admin.go`
- Modify: `app/console/internal/service/role.go`
- Modify: `pkg/rbac/casbin.go`
- Create: `cmd/grove/rbac.go`
- Test: admin and role failure-path tests

**Required behavior:**

- 删除当前没有实际覆盖 Casbin 写入的伪事务包装。
- 更新管理员角色时，失败不得留下半完成 grouping policy。
- 更新角色权限时，先准备新集合，再使用 adapter 支持的批量更新能力；失败时保留旧策略。
- 增加 `grove rbac check`，检查管理员 role_id、Casbin grouping 和角色 policy 一致性。
- 增加 `grove rbac repair --dry-run`，默认只输出差异。

---

## 5. Milestone 3：基础组件契约稳定化

### Task 13：重构 Cache 契约和生命周期

**Status:** `[-] In Progress`

**Owner:** Codex

**Branch/PR:** `codex/grove-foundation-roadmap`

**Started at:** 2026-07-14

**Design:** `docs/plans/2026-07-14-cache-contract-design.md`

**Files:**

- Modify: `pkg/cache/store.go`
- Modify: `pkg/cache/memory.go`
- Modify: `pkg/cache/redis.go`
- Modify: `internal/provider/provider.go`
- Modify: cache documentation
- Test: `pkg/cache/store_test.go`

**Required contract:**

```go
type Store interface {
    Get(ctx context.Context, key string) ([]byte, bool, error)
    Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
    Delete(ctx context.Context, key string) error
    Add(ctx context.Context, key string, value []byte, ttl time.Duration) (bool, error)
    TTL(ctx context.Context, key string) (time.Duration, bool, error)
}
```

- 不存在和永久 key 的语义在 Memory/Redis 完全一致。
- 删除 `Remember() (any, error)` 这种命中前后类型不同的 API。
- 泛型序列化通过包级函数提供，不塞入 Store 接口。
- Manager 增加 `Close()`，关闭 MemoryStore GC。
- 为 `RememberJSON[T]` 使用 singleflight 抑制进程内击穿。
- `Add` 不再宣传为完整分布式锁。

### Task 14：重构 HTTP Client 为请求级不可变状态

**Status:** `[ ] Planned`

**Files:**

- Modify: `pkg/httpclient/client.go`
- Split: `pkg/httpclient/request.go`
- Split: `pkg/httpclient/retry.go`
- Split: `pkg/httpclient/stream.go`
- Test: `pkg/httpclient/client_test.go`

**Required behavior:**

- Client 只保存 transport、base URL、默认 timeout。
- header、query、body、retry 全部属于单次 Request。
- Request Builder 不修改共享 Client。
- 默认仅重试 GET、HEAD、OPTIONS。
- POST/PUT 重试必须显式声明幂等 key 或 retry policy。
- backoff 使用 timer + context select，不使用裸 `time.Sleep`。
- 普通响应体有最大字节限制。
- 下载和 multipart 上传使用流式 IO。
- 默认 Transport 配置连接池、TLS handshake、response header timeout。

### Task 15：修复 Scheduler 并发、配置和取消语义

**Status:** `[ ] Planned`

**Files:**

- Modify: `pkg/scheduler/scheduler.go`
- Modify: `pkg/scheduler/scheduler_test.go`
- Modify: `internal/config/types.go`
- Modify: `internal/config/load.go`
- Modify: `internal/provider/provider.go`
- Modify: scheduler docs

**Required behavior:**

- Register 校验 nil task、空 name、空 schedule、nil job。
- 访问 tasks、entries、running 时统一加锁。
- Task 注册后复制配置，外部修改原指针不影响调度器。
- Scheduler 持有 root context；Stop 时 cancel。
- Task 支持可选 timeout。
- Stop 有最大等待时间并返回 error。
- 增加 `scheduler.enabled` 和 `scheduler.timezone` 配置。
- 明确哪些入口启用 Scheduler；不得存在文档有配置但运行时永远为 nil。
- 修复 race 测试，使用 atomic 或 channel 同步。

**Observed failure 2026-07-14:** `go test -race ./...` 在 `pkg/scheduler/scheduler_test.go:77` 读取计数器时，与测试任务函数第 58 行写入发生竞争；生产调度器并发语义仍需按本任务完整复核，不能只压掉测试告警。

**Verification:**

```bash
go test ./pkg/scheduler -race -count=20
```

### Task 16：修复 Event 异步投递语义

**Status:** `[ ] Planned`

**Files:**

- Modify: `pkg/event/dispatcher.go`
- Modify: `pkg/event/dispatcher_test.go`
- Modify: event docs

**Required behavior:**

- `Dispatch`：同步，返回监听器错误集合。
- `DispatchAsync`：等待入队或 context 取消，不静默丢弃。
- `TryDispatchAsync`：非阻塞，队列满返回 `ErrQueueFull`。
- 异步事件使用 `context.WithoutCancel` 或显式后台 context，但保留 request ID 等值。
- panic 转换成可观测错误，不只写日志后返回 nil。
- Close 拒绝新事件并排空已入队事件。

### Task 17：统一 Provider 生命周期和按服务配置校验

**Status:** `[ ] Planned`

**Files:**

- Modify: `internal/provider/provider.go`
- Modify: `internal/config/load.go`
- Modify: `internal/config/types.go`
- Modify: `pkg/server/core.go`
- Test: provider/config/server tests

**Required behavior:**

- Config `Validate(service)` 根据 api、console、worker 检查真实依赖。
- YAML 使用 `yaml.Decoder.KnownFields(true)`；未知字段和拼写错误必须启动失败。
- `config.example.yaml` 在不设置环境变量时必须可直接解析；通配符等 YAML 特殊值必须正确引用，并增加模板加载回归测试。
- production console 缺少数据库或权限配置时启动失败，不运行成 503 服务。
- worker 未启用时明确退出，不启动空进程等待信号。
- Provider 维护按创建逆序执行的 closers。
- logger、cache、event、scheduler、job、redis、database 都有明确关闭路径。
- HTTP server Start 返回监听失败，不在后台 goroutine 中 `Fatal`。
- Gin production 判断使用大小写无关比较。

---

## 6. Milestone 4：代码组织与开发体验

### Task 18：消除后端静态菜单真相源

**Status:** `[ ] Planned`

**Files:**

- Delete or reduce: `app/console/internal/service/menu_catalog.go`
- Modify: `app/console/internal/service/role.go`
- Modify: frontend menu permission helpers
- Modify: permission documentation
- Test: menu permission tests

**Recommended decision:** 前端路由 name 是菜单权限唯一真相源。后端只保存去重后的 key，并校验长度、数量、字符格式；不维护 title、path、icon、parent 或 sort。

**Required behavior:**

- 新增前端菜单无需修改 Go 文件。
- 历史无效 key 在返回时保留或通过显式清理命令处理，不能静默丢失数据。
- 前端角色页从本地 route 构建菜单树。

### Task 19：清理未使用的全局单例 API 和命名

**Status:** `[ ] Planned`

**Files:**

- Modify: `pkg/cache/store.go`
- Modify: `pkg/event/dispatcher.go`
- Modify: `pkg/scheduler/scheduler.go`
- Rename: `pkg/database.Repo` to `pkg/database.Manager` or `Connections`
- Modify all imports and docs

**Required behavior:**

- 删除未使用的 package-level `Init/Default/Dispatch/Register` 单例入口。
- 业务代码只通过构造函数获得依赖。
- `database.Repo` 不再使用 Repository 语义命名。
- 不新增 ServiceContainer 或字符串依赖解析。

### Task 20：收敛重复分页和响应映射

**Status:** `[ ] Planned`

**Files:**

- Modify: `app/console/internal/handler/common.go`
- Modify: admin/role/log/system_config handlers
- Create small response conversion files inside corresponding handler packages
- Test: handler contract tests

**Required behavior:**

- 使用嵌入结构组合分页、时间范围和排序 query。
- `AdminResponse` 使用一个 `newAdminResponse` 转换函数。
- `SystemConfigItem` 使用一个转换函数。
- 不创建通用 Mapper 包。
- 不让 GORM model 直接成为 HTTP response。

### Task 21：完善 OpenAPI 合同和漂移检查

**Status:** `[ ] Planned`

**Files:**

- Replace or simplify: `app/api/internal/docs/docs.go`
- Replace or simplify: `app/console/internal/docs/docs.go`
- Modify: `internal/docsui/`
- Create: `api/openapi/` or equivalent explicit contract source
- Add generated frontend API types if adopted
- Test: route/spec contract tests

**Required behavior:**

- OpenAPI 覆盖全部注册路由、请求、响应和错误结构。
- CI 比较 Gin route 列表和 OpenAPI operation，禁止静默漏文档。
- 文档不依赖大量手写 `map[string]any`。
- 保留 Scalar UI，但生产环境支持关闭或自托管静态资源。
- 不为生成文档引入新的 Web 框架。

### Task 22：增加 readiness、指标、trace 和安全 CI

**Status:** `[ ] Planned`

**Files:**

- Create: `internal/observability/`
- Create: readiness middleware/handler
- Modify: `pkg/server/core.go`
- Modify: provider constructors
- Modify: `.github/workflows/ci.yml`
- Modify: deployment docs

**Required behavior:**

- `/health/live` 只表示进程存活。
- `/health/ready` 检查当前服务必需的 DB、Redis、queue 依赖。
- OpenTelemetry 接入 Gin、GORM、Redis、HTTP Client、Asynq。
- 指标至少包含请求量、延迟、错误、DB pool、job 执行结果。
- CI 增加：

```bash
go test -race ./...
go vet ./...
govulncheck ./...
pnpm --dir web/admin-vben test:unit
pnpm --dir web/admin-vben build:console
```

- 增加 Dependabot 或 Renovate，只创建依赖更新 PR，不自动合并。

### Task 23：建立前端自定义代码测试基线

**Status:** `[ ] Planned`

**Files:**

- Create tests beside `apps/console/src/store/permission.ts`
- Create tests beside `apps/console/src/router/menu-access.ts`
- Create tests for request refresh/logout flow
- Create tests for role permission page helpers
- Modify CI

**Required behavior:**

- 菜单 key 过滤、首页选择、通配符行为有测试。
- 权限加载失败后必须 fail closed。
- refresh token 并发请求只触发一次 refresh。
- logout 清理 access、refresh、permission、route 状态。
- 自定义 Console 逻辑不依赖 Vben 上游测试间接覆盖。

---

## 7. Milestone 5：按需扩展组件

### Task 24：分布式锁和幂等组件

**Status:** `[~] Deferred`

**Activation condition:** 出现支付、Webhook、定时任务多实例、重复 Job 或同一业务操作并发执行。

**Design constraints:**

- `Locker` 使用唯一 token。
- 释放锁使用 Lua compare-and-delete。
- 明确 TTL、续期和 fencing token 需求。
- HTTP 幂等记录 request hash、response、状态和过期时间。
- 不把 `cache.Add` 伪装成完整分布式锁。

### Task 25：Transactional Outbox

**Status:** `[~] Deferred`

**Activation condition:** 出现“数据库成功但 Job/Event 投递失败”不可接受的业务流程。

**Design constraints:**

- 业务数据和 outbox 记录使用同一数据库事务。
- 独立 worker 投递并记录 attempt、last_error、next_retry_at。
- 消费端必须幂等。
- 不为仅进程内通知引入 Outbox。

### Task 26：通知渠道组件

**Status:** `[~] Deferred`

**Activation condition:** 至少存在邮件、短信、站内信中的两个真实渠道。

**Recommended pattern:** Strategy + Adapter。

```go
type Channel interface {
    Send(ctx context.Context, message Message) error
}
```

要求模板渲染、渠道发送、重试记录彼此分离，不创建 NotificationManager 层级树。

### Task 27：Webhook 管理组件

**Status:** `[~] Deferred`

**Activation condition:** 项目需要对第三方可靠推送业务事件。

范围：签名、secret 轮换、重试、退避、投递日志、手工重放、SSRF 防护。

### Task 28：CSV/Excel 导入导出

**Status:** `[~] Deferred`

**Activation condition:** 至少两个业务模块需要批量导入或导出。

范围：流式解析、行级错误、异步 Job、进度查询、结果文件、最大行数和公式注入防护。

### Task 29：数据权限、工作流、Feature Flag 等领域增强

**Status:** `[~] Deferred`

分别创建独立设计文档，禁止合并为“企业能力包”。

- 数据权限：出现部门、自有数据、区域或项目成员范围后再设计小型 `DataScope`。
- 状态机：出现至少一个包含非法状态迁移约束的复杂流程后使用 State 模式。
- Feature Flag：出现灰度发布或按用户启用功能的真实需求后接入 OpenFeature 等标准接口。
- 多租户：仅在租户隔离成为明确产品需求时启动。
- 插件系统：仅在外部团队需要独立发布模块时启动。

---

## 8. 设计模式使用约束

### 推荐使用

- Strategy：存储、通知、支付、验证码等存在多个可替换实现。
- Factory：仅负责根据显式配置创建 driver。
- State：复杂业务状态迁移和非法转换保护。
- Adapter：隔离第三方 SDK。
- Decorator/Middleware：日志、指标、重试、鉴权等横切行为。
- Outbox：数据库和可靠消息之间的一致性。

### 禁止默认引入

- ServiceContainer、运行时字符串依赖解析。
- `BaseService`、`AbstractRepository`、通用 CRUD Repository。
- 每个 struct 都提前定义 interface。
- 单一实现的 Factory/Strategy。
- 用事件替代一眼可见的直接函数调用。
- Handler → ApplicationService → DomainService → Repository 的固定层级模板。

### 抽象评审问题

新增抽象前必须回答：

1. 当前真实变化点是什么？
2. 至少有几个实现或调用方？
3. 直接函数或组合为什么不够？
4. 删除抽象后会破坏什么稳定契约？
5. 是否增加了调试调用链或运行时隐式行为？

---

## 9. 开发状态记录模板

每完成一个任务，在本节追加记录：

```markdown
### YYYY-MM-DD Task N 完成记录

- Status: Completed
- Owner:
- Branch/PR:
- Commits:
- Verification:
  - `go test ...`: PASS
  - `go test -race ...`: PASS
- Deviations from plan:
- Follow-up:
```

不得只勾选任务而不记录验证结果。若实现偏离本计划，必须先更新对应任务的 Architecture、Files 和 Required behavior，再继续开发。

---

## 10. 最终完成标准

本计划完成需同时满足：

- Milestone 1–4 全部为 Completed。
- Milestone 5 每项保持 Deferred，或已有独立设计文档和明确业务触发依据。
- 空库初始化、重复 bootstrap seed、完整回滚在 PostgreSQL 上通过。
- Console 登录、刷新、退出、权限变更和强制下线在多实例语义下有测试。
- 所有基础组件 race 测试通过并具有关闭路径。
- API 文档、路由、权限目录有自动漂移检查。
- 前后端生产构建通过。
- CI 包含 unit、integration、race、vet、vulnerability、frontend build。
- README、快速开始、部署和组件文档与实际命令一致。
- 仓库不存在默认固定密码、公开任意 token、未限制上传等生产阻断项。
