# 测试策略

本文档说明 Grove 当前采用的测试方式与编写约定。

## 测试范围

当前仓库主要包含：

- Go 单元测试与集成测试
- Testcontainers 驱动的真实 PostgreSQL/MySQL 生命周期测试
- 路由与服务层测试
- 前端 unit、类型检查和 production build

## 最短路径

### 运行 Go 测试

```bash
go test ./...
```

### 运行统一验证

```bash
make verify
```

`make verify` 包含：

- Go 测试
- 后端构建
- 管理后台类型检查

前端专项验证：

```bash
make admin.typecheck
make admin.build
```

前端 unit 测试使用仓库现有 Vitest 配置：

```bash
make admin.test
```

Makefile 通过 corepack 使用仓库固定的 pnpm 版本，避免全局 pnpm 版本不同导致检查失败。

### 运行数据库集成测试

集成测试通过 Testcontainers 启动真实数据库，验证迁移、bootstrap seed、重复执行密码不覆盖、dirty 状态、约束和完整 down 生命周期。需要可用的 Docker、OrbStack 或其他兼容容器运行时，两种方言须分别执行：

```bash
GROVE_INTEGRATION_DB=postgres go test -tags=integration ./tests/integration -v
GROVE_INTEGRATION_DB=mysql go test -tags=integration ./tests/integration -v
```

当前入口的行为并不完全相同：

- [PostgreSQL](../../tests/integration/migration_test.go)：未设置 `GROVE_INTEGRATION_DB` 时默认执行；仅在 `CI` 为空时允许因 Docker 不可用而跳过。
- [MySQL](../../tests/integration/mysql_migration_test.go)：必须设置 `GROVE_INTEGRATION_DB=mysql`；与 PostgreSQL 相同，仅本地允许因 Docker 不可用而跳过。通过真实 SQL 连接检查就绪，不以初始化期间的日志作为可用证明。
- 因此退出码为 0 或显示 `PASS` 不足以证明两种数据库均已执行，必须检查对应测试的 `-v` 输出中没有 `SKIP`。

Redis contract 使用独立测试实例或专用测试库，入口是 [TestRedisStoreContract](../../pkg/cache/redis_integration_test.go)：

```bash
CACHE_REDIS_ADDR=127.0.0.1:6379 CACHE_REDIS_DB=15 \
  go test -tags=integration ./pkg/cache -run '^TestRedisStoreContract$' -v
```

未设置 `CACHE_REDIS_ADDR` 时会跳过，不能作为 Redis 已验收的证据。

两种方言都验证旧唯一约束的回滚前置条件：存在软删除后的重复值时，CLI 必须在任何 DDL 之前拒绝，保持数据、索引和迁移版本不变；测试显式修正夹具数据后再验证完整 down。回滚在停止应用写入的维护窗口进行，详见[数据库指南](../guide/database.md)。

## 共用测试夹具

`app/`、`cmd/`、`internal/` 的测试可使用 [internal/testkit](../../internal/testkit/testkit.go)：

- `OpenDB(t, models...)`：每个测试独立的 SQLite 文件，可按需迁移模型，通过 `t.Cleanup` 关闭连接。
- `CreateCasbinTable(t, db, table)`：创建与迁移约束一致的 Casbin 测试表。

生成的 service 测试同样使用 `OpenDB`。`pkg/` 测试保持自己的夹具，避免反向依赖 `internal/`。目前没有通用模型工厂或 HTTP 测试助手；新增抽象须先有重复使用场景。

SQLite 的模型迁移测试不执行 PostgreSQL/MySQL SQL，不能替代上面的真实数据库测试。

## 生成器回归的范围

[cmd/grove/main_test.go](../../cmd/grove/main_test.go)会复制仓库、生成模块，再运行后端 `go vet`、生成的 CRUD 测试、Console 路由/OpenAPI/前端契约比对和双方言迁移文件规则；单字段分支也有回归。

这些回归在 `go test -short` 下跳过；它们不运行生成页面的 Vue typecheck/build、真实数据库迁移或浏览器操作。新增模块后仍需执行相关前端门禁与[Staging 验收](../deployment/staging-checklist.md)，真实业务接入成本见[升级清单 T9](../plans/2026-08-29-grove-framework-upgrade-plan.md#t9真实模块接入验证)。

## 编写约定

- 测试文件使用 `*_test.go`
- 优先使用表驱动测试
- service 测试关注输入输出与错误分支
- router 测试关注认证、权限和响应状态
- 共享组件测试放在对应 `pkg/*` 或 `internal/*` 目录

## 重点场景

- 参数校验失败
- 认证失败
- 权限拒绝
- 正常成功响应
- 关键业务错误分支

## 边界

- `tests/integration/` 仅放依赖真实基础设施的跨包生命周期测试。
- 前端 unit 覆盖请求错误解析、认证状态、权限菜单过滤、ResourcePage 和计划任务页等；尚无自动浏览器 E2E 基线，组件挂载测试不替代浏览器与真实服务验收。

## 相关文档

- [快速上手](../guide/quickstart.md)
- [错误处理](./error-handling.md)
