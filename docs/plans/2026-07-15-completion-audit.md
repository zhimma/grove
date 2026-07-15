# Grove Foundation 路线图完成审计

## 状态

- 审计日期：2026-07-15
- Task 1–23：Completed
- Task 24–29：Deferred，等待真实业务触发
- 当前分支：`codex/grove-foundation-roadmap`

## 验收证据

- PostgreSQL 17.7（`127.0.0.1:5432`）临时数据库：11 个迁移 up、重复 bootstrap 不覆盖已修改 root 密码、全部迁移 down 后业务表归零。
- PostgreSQL 并发 refresh：8 路并发复用同一 refresh token，恰好 1 次成功，其余返回 `invalid_refresh_token`。
- PostgreSQL readiness：真实连接检查通过。
- 后端：`go test ./...`、`go test -race ./...`、`go vet ./...`、`make build` 全部通过。
- 安全扫描：Go 1.25.12 实际 GOROOT 下 `govulncheck ./...` 通过，可达漏洞为 0。使用旧 Go 1.25.0 wrapper 会产生错误的标准库漏洞结果，CI 已固定 Go 1.25.12。
- 路由与 OpenAPI：API、Console route contract 测试通过。
- 前端：使用 pnpm 10.28.2 执行 `test:unit`（41 个文件、316 个测试）、Console `typecheck` 和 `@grove/console build` 均通过。
- CI：包含 unit、route/OpenAPI contract、race、vet、govulncheck、Testcontainers PostgreSQL integration、backend build、frontend typecheck/unit/build。
- 配置安全：示例和部署文档不再提供固定 PostgreSQL 密码或 JWT secret；`internal/config` 增加静态凭据回归测试。
- 配置来源已收敛为单一 `config.yaml`；`.env` 不再由后端自动读取，环境变量只保留为部署覆盖机制。
- Makefile 已删除 `run/dev`、`test.go`、`fmt.go`、`verify.go`、独立后端 build 和重复前端 verify 入口，只保留面向开发者的公开命令。

## 环境边界

- 本机没有 Docker/OrbStack，Testcontainers 测试按测试设计安全 Skip；CI runner 无容器运行时时会失败而不是伪造通过。
- 本机 Redis 需要认证，本轮未猜测密码执行 Redis contract；Memory/cache 生命周期和 race 测试已通过，Redis contract 仍可在提供 Redis 凭据后单独执行。
- 本机前端 `node_modules` 已有依赖但缺少 `.bin`/public-hoist 链接；根 `build:console` 的 `cross-env` 入口因此无法直接启动，使用临时 PATH 链接执行了同一 `@grove/console build`，并通过 `NODE_PATH` 找到已有 `cssnano`。没有执行 `pnpm install`。

## 本轮修复

- `config.example.yaml` 将数据库密码和 JWT secret 默认值改为空环境变量。
- 快速开始、配置、数据库和部署文档改为通过 `DB_PASSWORD`、`JWT_SECRET` 注入敏感值。
- 增加 `TestConfigExampleDoesNotContainStaticCredentials`，防止固定凭据回归。
- 删除 `.env.example`，增加 `.env` 不会被读取的配置回归测试。

## 文档重构补充

- 建立 `AGENTS.md`、`docs/architecture.md`、`docs/commands.md`、`docs/operations.md` 和 `docs/ai/` 作为开发者与 AI 的统一入口。
- 重写快速上手、项目结构、配置、Console 新增模块和响应错误文档，按当前代码路径、Makefile、配置和 OpenAPI 入口校正。
- `docs/plans/` 增加索引与文档重构计划；历史设计文档保留为决策背景，不再作为日常开发入口。
- 文档链接、旧命令、旧路径和配置来源扫描通过；文档修改未改变运行时代码行为。
