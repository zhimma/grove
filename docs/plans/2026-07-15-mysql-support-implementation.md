# PostgreSQL 与 MySQL 8 支持实施记录

> 详细任务拆分、目录契约、验收标准和后续开发规则见
> [数据库方言分层与 MySQL 支持实施计划](2026-07-15-database-dialect-layering-plan.md)。

## 状态

- 状态：Completed locally（远程 CI 待执行）
- 默认数据库：PostgreSQL
- 新增数据库：MySQL 8.0.16+
- 不包含：PostgreSQL 存量数据迁移、MariaDB 兼容

## 已完成

- database 配置支持 `postgres`、`mysql`。
- MySQL DSN 默认启用 `utf8mb4`、`parseTime`、`multiStatements` 和 `loc`。
- Provider、CLI、migration manager 根据 database driver 选择方言。
- PostgreSQL migration/seed 移入 `postgres` 子目录。
- 新增 MySQL migration/seed 子目录。
- `StringArray` 支持 PostgreSQL JSONB 和 MySQL JSON。
- 日志清理逻辑改为参数化时间条件，移除 MySQL 专属 `DATE_SUB`。
- MySQL 默认端口仅在未配置端口时使用 3306，显式端口和 `parse_time: false` 均会保留。
- 增加 MySQL 配置、DSN、迁移目录和 integration test 入口。
- MySQL 集成测试覆盖 dirty 状态、bootstrap 幂等、demo、外键、CHECK、Casbin 唯一规则和软删除唯一性。

## 已完成验证

- PostgreSQL 17.7 临时数据库：11 个 migration up、bootstrap、重复 bootstrap、约束检查、完整 down 通过。
- MySQL 8.0.46 隔离临时实例：11 个 migration up、bootstrap、重复 bootstrap、demo、生成列唯一索引、外键、CHECK、Casbin 唯一规则、软删除唯一性、完整 down 通过。
- 两种数据库均确认 `grove_migrations.dirty = false`，root 已有密码不会被重复 bootstrap 覆盖。
- `go test ./...`、`go test -race ./...`、`go vet ./...`、`govulncheck ./...`、`make build`、`git diff --check` 通过。
- Testcontainers 集成测试在本机因 Docker/OrbStack 不可用安全 Skip；同样的真实数据库流程已使用本机 PostgreSQL 和隔离 MySQL 实例完成。

## 外部门禁

- `.github/workflows/ci.yml` 已加入 PostgreSQL/MySQL 集成矩阵。
- 当前会话未 push、未创建 PR，因此没有远程 CI 运行结果；不能将 CI 标记为已通过。

## 本次验证记录

### PostgreSQL

- 版本：PostgreSQL 17.7（Homebrew）。
- 数据库：临时库 `grove_codex_pg_20260715`，验证后已删除。
- 执行：`migrate up` -> `seed bootstrap` -> 重复 bootstrap -> `seed demo` -> `migrate status` -> 11 次 `migrate down`。
- 结果：11 个 migration 全部执行，root 密码在重复 bootstrap 后保持不变，`grove_migrations.dirty=false`，所有业务表回到空库。

### MySQL

- 版本：MySQL 8.0.46（仓库现有 Homebrew 服务不可用，因此使用临时隔离实例，端口 13306）。
- 数据库：临时库 `grove_codex_mysql_20260715`，临时实例和数据目录验证后已关闭并清理。
- 执行：`migrate up` -> `seed bootstrap` -> 重复 bootstrap -> `seed demo` -> `migrate status` -> 11 次 `migrate down`。
- 结果：11 个 migration 全部执行；root 密码保持不变；生成列唯一索引、外键、CHECK、Casbin 唯一约束、软删除 email 复用均通过；`grove_migrations.dirty=0`；所有业务表回到空库。

### 自动化门禁

- 通过：`go test ./...`、`go test -race ./...`、`go vet ./...`、`govulncheck ./...`、`make build`、`git diff --check`。
- 通过：带 integration build tag 的测试包编译。
- 本机 Testcontainers：因 Docker/OrbStack 未运行而安全 Skip；CI 中仍会按 workflow 启动容器并在失败时失败。
