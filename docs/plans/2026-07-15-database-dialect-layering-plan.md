# 数据库方言分层与 MySQL 支持实施计划

> **For Claude:** Use an executing-plans workflow to implement this plan task-by-task.

**Goal:** 在保留 PostgreSQL 默认行为的前提下，为 Grove 增加 MySQL 8.0.16+ 支持，并将 migration、bootstrap seed、demo seed 按数据库方言和用途分层隔离，保证同一版本号在不同数据库上保持可追踪、可回滚、可验证。

**Architecture:** 配置中的 `databases.default.driver` 是数据库方言唯一选择入口。CLI 不直接拼接 PostgreSQL/MySQL SQL，而是根据 driver 解析 `database/migrations/<driver>` 和 `database/seeds/<driver>/<kind>`；两种数据库共享迁移版本号和业务语义，但每个方言维护自己的 SQL 文件。`bootstrap` 是生产安全初始化，`demo` 只允许非生产环境执行。

**Tech Stack:** Go 1.25.12, GORM, `golang-migrate/migrate`, PostgreSQL, MySQL 8.0.16+, Cobra CLI, Testcontainers, GitHub Actions。

---

## 1. 目录契约

最终目录固定为：

```text
database/
├── migrations/
│   ├── postgres/
│   │   ├── 202604150001_create_users.up.sql
│   │   ├── 202604150001_create_users.down.sql
│   │   └── ...
│   └── mysql/
│       ├── 202604150001_create_users.up.sql
│       ├── 202604150001_create_users.down.sql
│       └── ...
└── seeds/
    ├── postgres/
    │   ├── bootstrap/
    │   │   ├── 202604150001_system_configs.sql
    │   │   └── 202604150002_console_root_super_admin.sql
    │   └── demo/
    │       ├── 202604150001_api_user.sql
    │       └── 202604150002_console_admin.sql
    └── mysql/
        ├── bootstrap/
        │   ├── 202604150001_system_configs.sql
        │   └── 202604150002_console_root_super_admin.sql
        └── demo/
            ├── 202604150001_api_user.sql
            └── 202604150002_console_admin.sql
```

目录规则：

- `migrations/<driver>` 只放该数据库的 `.up.sql` 与 `.down.sql`，不得混放其他方言。
- `seeds/<driver>/bootstrap` 只放生产可执行的基础数据；`seeds/<driver>/demo` 只放开发/测试演示数据。
- PostgreSQL 和 MySQL 的 migration 版本号必须一一对应；文件名中的版本号相同，SQL 内容允许不同。
- 新增数据库时只增加新的 `<driver>` 子目录和对应连接/迁移驱动，不修改已有 PostgreSQL/MySQL 文件。
- 业务代码不读取方言目录，也不在 handler/service 中执行数据库初始化 SQL。
- 兼容自定义 `--path`，但新文件必须写入该路径下的方言子目录；根目录兼容读取只用于过渡，不作为新项目结构。

## 2. 执行状态

- `[x]` 已完成代码、测试和本地真实数据库验证。
- `[~]` 已配置但尚未在当前会话执行的外部门禁（例如 GitHub Actions）。
- `[ ]` 尚未完成。

当前状态：`[x]` Implementation complete（本地）。PostgreSQL 17.7 和隔离 MySQL 8.0.46 已完成 migration、bootstrap、重复 bootstrap、demo、约束、软删除唯一性、status 和完整 down 闭环；CI workflow 已配置，尚未从远程 CI 实际运行。

## 3. 分阶段任务

### Task 1：盘点现有数据库入口与迁移资产 `[x]`

**Files:**

- Review: `internal/config/types.go`
- Review: `internal/config/load.go`
- Review: `pkg/database/database.go`
- Review: `pkg/migrate/migrate.go`
- Review: `cmd/grove/main.go`
- Review: `database/migrations/`
- Review: `database/seeds/`
- Review: `Makefile`

**Steps:**

1. 确认默认数据库仍为 PostgreSQL，环境变量只覆盖配置字段，不引入第二套配置源。
2. 列出全部 migration 版本号、up/down 对，记录 seed 的 bootstrap/demo 边界。
3. 搜索 PostgreSQL 专属 SQL：`::jsonb`、`TIMESTAMPTZ`、`BIGSERIAL`、`ON CONFLICT`、`USING`、partial index。
4. 搜索 MySQL 专属 SQL：`DATE_SUB`、`ON DUPLICATE KEY`，确认它们没有泄漏到共享业务查询。

**Verification:**

```bash
rg -n '::jsonb|TIMESTAMPTZ|BIGSERIAL|ON CONFLICT|USING|DATE_SUB|ON DUPLICATE KEY' \
  database app internal pkg
```

完成条件：所有数据库相关入口和方言差异都有明确归属，后续任务不依赖猜测。

### Task 2：迁移文件按方言拆分 `[x]`

**Files:**

- Move: `database/migrations/*.up.sql` -> `database/migrations/postgres/`
- Move: `database/migrations/*.down.sql` -> `database/migrations/postgres/`
- Create: `database/migrations/mysql/*.up.sql`
- Create: `database/migrations/mysql/*.down.sql`
- Modify: `pkg/migrate/migrate.go`
- Test: `pkg/migrate/migrate_test.go`

**Steps:**

1. 将现有 PostgreSQL 文件完整移动到 `database/migrations/postgres/`，不改版本号。
2. 为 MySQL 创建同版本的 001-011 migration 文件。
3. 替换数据库差异：
   - `TIMESTAMPTZ` -> `DATETIME(6)`。
   - `BIGSERIAL` -> `BIGINT AUTO_INCREMENT`。
   - `JSONB` -> `JSON`。
   - PostgreSQL partial unique index -> MySQL 生成列 + unique index。
   - `ON CONFLICT` -> `ON DUPLICATE KEY UPDATE`，仅用于 seed。
   - PostgreSQL `DELETE ... USING` -> MySQL JOIN DELETE。
4. 所有 migration 必须有匹配 down 文件。
5. 迁移元数据表统一为 `grove_migrations`，不得复用旧表结构。

**Verification:**

```bash
go test ./pkg/migrate -run 'TestMigration|TestSeed' -v
```

完成条件：两套目录的 migration 版本集合一致，MySQL 文件不包含 PostgreSQL 专属语法。

### Task 3：实现按 driver 解析 migration 目录 `[x]`

**Files:**

- Modify: `pkg/migrate/migrate.go`
- Modify: `cmd/grove/main.go`
- Test: `pkg/migrate/migrate_test.go`
- Test: `cmd/grove/main_test.go`

**Steps:**

1. 实现 `ResolveDialectDir(baseDir, driver)`，支持 `postgres` 和 `mysql`。
2. 实现 `ResolveDialectDirWithSuffix(baseDir, driver, suffix)`，供 migration 和 SQL seed 共用。
3. `migrate up/down/status` 使用 GORM Dialector 名称选择 golang-migrate 的 PostgreSQL/MySQL database driver。
4. `migrate create <name>` 根据当前配置的 driver 写入对应子目录，并同时创建 up/down 文件。
5. 对 `postgresql` 做归一化为 `postgres`；其他 driver 直接报错。
6. 保留显式具体目录和旧根目录读取兼容，但不在根目录创建新文件。

**Verification:**

```bash
go test ./pkg/migrate ./cmd/grove -run 'TestResolve|TestMigrationCreate|TestMigrateHelp' -v
```

完成条件：在 `driver=mysql` 时，`migrate create` 只能写入 `database/migrations/mysql/`；不能误写 PostgreSQL 目录。

### Task 4：seed 按方言和用途分层 `[x]`

**Files:**

- Move: `database/seeds/bootstrap/` -> `database/seeds/postgres/bootstrap/`
- Move: `database/seeds/demo/` -> `database/seeds/postgres/demo/`
- Create: `database/seeds/mysql/bootstrap/`
- Create: `database/seeds/mysql/demo/`
- Modify: `cmd/grove/main.go`
- Modify: `pkg/migrate/migrate.go`
- Test: `cmd/grove/main_test.go`
- Test: `pkg/migrate/migrate_test.go`

**Steps:**

1. `seed bootstrap` 解析 `database/seeds/<driver>/bootstrap`。
2. `seed demo` 解析 `database/seeds/<driver>/demo`。
3. `seed demo` 在 `production` 环境直接拒绝，不能依赖调用者自觉。
4. bootstrap seed 在事务中执行，并通过 `GROVE_ROOT_PASSWORD` 或随机一次性密码生成 root 密码哈希。
5. bootstrap 重复执行时不覆盖已有 root 密码；demo seed 可幂等刷新演示数据。
6. 支持 `seed --path <dir>`，自定义目录仍遵循 `<driver>/<kind>` 结构。
7. SQL 文件按文件名排序执行，单个文件失败时返回已执行数量并回滚 bootstrap 事务。

**Verification:**

```bash
go test ./cmd/grove ./pkg/migrate -run 'TestSeed|TestBootstrap|TestDemo' -v
```

完成条件：PostgreSQL/MySQL seed 文件不交叉执行，bootstrap/demo 的安全边界和幂等性都有测试保护。

### Task 5：连接配置和 DSN 支持 MySQL `[x]`

**Files:**

- Modify: `internal/config/types.go`
- Modify: `internal/config/load.go`
- Modify: `pkg/database/database.go`
- Modify: `internal/provider/provider.go`
- Modify: `cmd/grove/main.go`
- Modify: `config.example.yaml`
- Test: `internal/config/load_test.go`
- Test: `pkg/database/database_test.go`

**Steps:**

1. 增加 `driver: mysql`，默认端口 3306；PostgreSQL 默认端口保持 5432。
2. 增加 MySQL 字段：`charset`、`parse_time`、`loc`、`tls`。
3. MySQL DSN 默认包含 `charset=utf8mb4`、`parseTime=true`、`multiStatements=true`、`loc=Local`。
4. 保持 GORM 的 `db.Dialector.Name()` 为 `postgres` 或 `mysql`，供 migration/seed 目录选择。
5. `doctor` 输出当前 driver，便于排查执行了错误方言。
6. 不引入 Java/Spring 风格命名，不新增 Repository/DAO 层作为数据库适配层。

**Verification:**

```bash
go test ./internal/config ./pkg/database ./internal/provider -v
```

完成条件：配置文件、环境变量和 DSN 三者对 MySQL 的行为一致，默认 PostgreSQL 回归不变。

### Task 6：修复共享模型和业务 SQL 的方言耦合 `[x]`

**Files:**

- Modify: `internal/datatype/string_array.go`
- Modify: `internal/model/console_role.go`
- Modify: `internal/model/user.go`
- Modify: `internal/model/console_admin.go`
- Modify: `internal/model/system_config.go`
- Modify: `app/console/internal/service/login_log.go`
- Modify: `app/console/internal/service/operation_log.go`
- Test: `internal/datatype/string_array_test.go`

**Steps:**

1. `StringArray.DataType()` 根据 GORM Dialector 返回 PostgreSQL `jsonb`，MySQL/SQLite 返回 `json`。
2. `Scan` 同时兼容 `[]byte`、`string` 和 `nil`，避免 MySQL 驱动返回类型差异导致解析失败。
3. 删除模型中硬编码的 PostgreSQL partial-index GORM tag，把生产唯一约束交给 migration。
4. 将 `DATE_SUB(NOW(), INTERVAL ? DAY)` 改为 Go 计算 cutoff 后的参数化时间比较。
5. 保持软删除后可复用 email、account、phone、配置 key 等唯一业务语义。

**Verification:**

```bash
go test ./internal/datatype ./app/console/internal/service ./internal/model -v
```

完成条件：共享 Go 代码不依赖某个数据库的 SQL 方言，唯一约束和 JSON 字段在两种数据库中语义一致。

### Task 7：补齐自动化测试和 CI 矩阵 `[x] / [~]`

**Files:**

- Create: `tests/integration/mysql_migration_test.go`
- Modify: `tests/integration/migration_test.go`
- Modify: `.github/workflows/ci.yml`
- Modify: `pkg/migrate/migrate_test.go`
- Modify: `cmd/grove/main_test.go`

**Steps:**

1. 单元测试验证配置、DSN、目录解析、版本一致性、seed 文件完整性和 PostgreSQL 语法扫描。
2. Testcontainers 集成测试读取 `GROVE_INTEGRATION_DB`，至少覆盖 `postgres` 和 `mysql` 两个矩阵值。
3. 集成测试顺序固定为：空库 -> `migrate up` -> `seed bootstrap` -> 重复 bootstrap -> `seed demo` -> `migrate down`。
4. 验证重复 bootstrap 不改变已有 root 密码。
5. 验证 foreign key、check、唯一索引、Casbin 规则去重和软删除唯一约束。
6. Docker/OrbStack 不可用时本地测试安全 Skip；CI 中容器启动失败必须失败。

**Verification:**

```bash
go test ./internal/config ./internal/datatype ./pkg/database ./pkg/migrate ./internal/provider ./cmd/grove
GROVE_INTEGRATION_DB=postgres go test -tags=integration ./tests/integration -v
GROVE_INTEGRATION_DB=mysql go test -tags=integration ./tests/integration -v
```

完成条件：本地单元测试、race、vet 和真实 PostgreSQL/MySQL 手工闭环通过；CI PostgreSQL/MySQL 矩阵已写入 workflow。当前会话没有远程 CI 执行权限，因此 CI 结果需在 PR/push 后继续记录。

### Task 8：更新开发、部署和 AI 上下文文档 `[x]`

**Files:**

- Modify: `README.md`
- Modify: `docs/architecture.md`
- Modify: `docs/commands.md`
- Modify: `docs/guide/database.md`
- Modify: `docs/guide/configuration.md`
- Modify: `docs/guide/quickstart.md`
- Modify: `docs/deployment/deploy.md`
- Modify: `docs/development/testing.md`
- Modify: `docs/ai/project-context.md`
- Modify: `docs/plans/README.md`

**Steps:**

1. 在 quickstart 中分别给出 PostgreSQL 和 MySQL 连接配置。
2. 在 database guide 中说明目录契约、版本号一致性和 SQL 不混用规则。
3. 在 commands guide 中记录 `migrate up/down/status/create`、`seed bootstrap/demo` 和 `--path`。
4. 在部署文档中强调生产只执行 migration + bootstrap，不执行 demo seed。
5. 在 AI 上下文中记录：数据库 driver 是 migration/seed 方言选择真相源。
6. 在 plans 索引中链接本计划和实施记录。

**Verification:**

```bash
rg -n 'database/migrations/(postgres|mysql)|database/seeds/(postgres|mysql)|seed bootstrap|seed demo|migrate create' \
  README.md docs
```

完成条件：新开发者只阅读 README、database guide、commands guide 和本计划即可完成数据库初始化。

### Task 9：真实数据库闭环验证 `[x]`

**Files:**

- Verify: `database/migrations/postgres/`
- Verify: `database/migrations/mysql/`
- Verify: `database/seeds/postgres/`
- Verify: `database/seeds/mysql/`
- Update: `docs/plans/2026-07-15-mysql-support-implementation.md`
- Update: this plan's status section

**Steps:**

1. 确认本机或 CI 有可用 PostgreSQL 和 MySQL 8.0.16+，不要擅自重启或修改用户数据库服务。
2. PostgreSQL 执行：

   ```bash
   DB_DRIVER=postgres make migrate.up
   DB_DRIVER=postgres make seed.bootstrap
   DB_DRIVER=postgres make seed.bootstrap
   DB_DRIVER=postgres make seed.demo
   DB_DRIVER=postgres make migrate.status
   ```

3. MySQL 执行同样流程：

   ```bash
   DB_DRIVER=mysql make migrate.up
   DB_DRIVER=mysql make seed.bootstrap
   DB_DRIVER=mysql make seed.bootstrap
   DB_DRIVER=mysql make seed.demo
   DB_DRIVER=mysql make migrate.status
   ```

4. 在临时数据库上执行多次 `migrate down`，确认能完整回到空库；migration dirty 时 CLI 必须拒绝继续。
5. 记录数据库版本、容器镜像、命令、结果和失败 SQL；不能只记录“测试通过”。

**完成条件：**

- `[x]` PostgreSQL 17.7 空库初始化、重复 bootstrap、约束检查、demo 规则和完整 down 通过。
- `[x]` MySQL 8.0.46 空库初始化、重复 bootstrap、demo、CHECK、外键、Casbin 唯一约束、软删除唯一性和完整 down 通过。
- `[x]` 两种数据库的 migration version 一致且 `grove_migrations.dirty = false`。
- `[x]` 用户现有 MySQL 服务未被重启或修改；验证使用隔离临时实例，验证后已关闭并清理。

### Task 10：最终门禁与交付 `[x] / [~]`

**Files:**

- Verify: all changed files
- Update: `docs/plans/2026-07-15-mysql-support-implementation.md`
- Update: this plan's status section

**Steps:**

1. 执行格式、测试、静态检查：

   ```bash
   git diff --check
   GOCACHE=/private/tmp/grove-go-build go test ./...
   GOCACHE=/private/tmp/grove-go-build go vet ./...
   ```

2. 执行最终方言泄漏扫描：

   ```bash
   rg -n 'DATE_SUB|::jsonb|ON CONFLICT|EXCLUDED|TIMESTAMPTZ|BIGSERIAL' \
     database/migrations/mysql database/seeds/mysql app internal pkg \
     --glob '!**/*_test.go'
   ```

   预期：MySQL 目录不出现 PostgreSQL 专属关键字；共享业务代码不出现 MySQL 专属日期函数。

3. 重新阅读所有修改文件，确认 imports、事务边界、错误处理和文档命令一致。
4. 更新实施记录状态：本地代码和真实数据库验证全部通过后改为 `Completed locally`；远程 CI 尚未执行时，不得伪造 CI 通过记录。
5. 当前任务不执行 PostgreSQL 存量数据迁移、不承诺 MariaDB、不引入通用 Repository/DAO 抽象。

## 4. 版本与新增模块开发规则

以后新增业务模块时按以下顺序执行：

1. 先在 `database/migrations/postgres/` 创建 up/down 文件。
2. 复制相同版本号到 `database/migrations/mysql/`，将 SQL 翻译为 MySQL 语法。
3. 在两种数据库的 `bootstrap` 或 `demo` 目录增加对应 seed；不把初始化数据塞进 migration。
4. 修改共享 model 时先确认 GORM DataType、索引和软删除语义在两种数据库中的等价实现。
5. 补单元测试，再补 PostgreSQL/MySQL 集成测试。
6. 文档和 AI 上下文同步更新，最后运行 `make verify`。

禁止事项：

- 禁止把 PostgreSQL SQL 直接复制到 MySQL 目录后只改文件名。
- 禁止在 handler/service 中拼接初始化 SQL。
- 禁止用 AutoMigrate 代替生产 migration。
- 禁止用 demo seed 创建生产管理员或覆盖真实密码。
- 禁止只验证 PostgreSQL 就宣称 MySQL 支持完成。

## 5. 风险与处理策略

- **MySQL 版本差异：** 目标版本固定为 MySQL 8.0.16+，因为生成列、CHECK 和 JSON 行为依赖版本；不承诺 MariaDB。
- **已有 PostgreSQL 数据：** 本计划只新增 MySQL 支持，不做 PostgreSQL 存量数据迁移；迁移任务只针对新环境和可控测试库。
- **软删除唯一索引：** PostgreSQL 使用 partial unique index，MySQL 使用生成列 + unique index；两套 SQL 必须用相同业务语义测试验证。
- **seed 重复执行：** bootstrap 必须保护 root 密码，Casbin 规则先删除再插入；demo 只针对非生产环境。
- **自定义路径：** `--path` 允许扩展，但必须保持 `<driver>/<kind>` 子目录契约，避免路径指向错误方言。

## 6. 完成定义

本计划只有在以下条件全部满足时才算完成：

- PostgreSQL 默认路径和现有命令回归通过。
- MySQL 8.0.16+ 配置、连接、migration、bootstrap seed、demo seed、status、down 均通过。
- 两种数据库 migration 版本号一致，up/down 成对，目录隔离测试通过。
- `go test ./...`、`go test -race ./...`、`go vet ./...`、`govulncheck ./...`、`make build`、`git diff --check` 通过。
- README、数据库指南、命令指南、部署文档和 AI 上下文已同步。
- 实施记录中明确列出真实环境、命令、结果和未执行的远程 CI/前端门禁。
