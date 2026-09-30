# 数据库与模型

本文档说明 Grove 中数据库连接、共享模型和迁移的基本用法。

## 适用范围

当前数据库能力基于 GORM，支持 PostgreSQL 和 MySQL 8.0.16+：

- 默认数据库连接
- 命名数据库资源
- SQL 迁移与种子数据
- 共享模型定义

`*database.Connections` 表示默认连接和命名连接的集合，是具体类型而非接口（只有一种实现，测试直接用 `database.NewConnectionsFromDBs`）。它只管理连接生命周期，不提供通用 Repository 抽象；方法对 nil 接收者安全。

## 最短路径

### 默认数据库配置

```yaml
databases:
  default:
    enabled: true
    driver: postgres
    host: 127.0.0.1
    port: 5432
    user: postgres
    password: ""
    dbname: grove
    ssl_mode: disable
```

MySQL 配置：

```yaml
databases:
  default:
    enabled: true
    driver: mysql
    host: 127.0.0.1
    port: 3306
    user: root
    password: ""
    dbname: grove
    charset: utf8mb4
    parse_time: true
    loc: Local
    tls: false
```

### 在装配层获取连接

以下示例中的 `p` 是启动或路由装配层的 Provider。获取连接后，将业务所需的数据库依赖注入 service。

```go
db := p.DB.Default()
```

### 获取命名资源

```go
ordersDB, err := p.DB.Get("orders")
if err != nil {
	return err
}
_ = ordersDB
```

### 定义共享模型

```go
type Article struct {
	ID        string    `gorm:"primaryKey"`
	Title     string    `gorm:"size:255;not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
}
```

### 执行迁移

```bash
go run ./cmd/grove migrate up
```

迁移和种子按数据库方言分层：

```text
database/migrations/postgres/
database/migrations/mysql/
database/seeds/postgres/
database/seeds/mysql/
```

两种数据库使用相同迁移版本号，但 SQL 文件不混用。现有 PostgreSQL 部署继续使用 PostgreSQL 目录；新 MySQL 环境会自动选择 MySQL 目录。

## 使用约定

- 共享模型放在 `internal/model`。
- 装配层从 `p.DB` 获取数据库资源，将所需的 `*database.Connections` 或 `*gorm.DB` 注入 service；handler 不直接操作数据库。
- 需要多数据源时使用 `databases.resources`，不要在业务代码里手工创建连接。
- 迁移文件使用正反向 SQL，按时间戳命名。
- 生产环境只通过迁移和种子初始化数据库，不使用 AutoMigrate 替代迁移。

## 模型边界

- 模型负责字段定义与通用查询辅助。
- 模型不负责权限判断、HTTP 响应拼装或业务流程编排。
- 复杂业务逻辑应放在 `service`，而不是 GORM hook。

## 相关文档

- [开发规范](../01-%E5%BC%80%E5%8F%91%E8%A7%84%E8%8C%83.md)
- [服务层](../03-console-新增模块指南.md)

## 回滚与数据前置条件

回滚应在备份完成、应用写入停止的维护窗口中执行。版本 `202604150009` 会恢复包含软删除行的旧唯一约束。当前 CLI 会先检查 `users.email`、`console_roles.code`、`console_admins.account`、配置分组与键，以及 PostgreSQL 旧管理员邮箱和手机号约束。

存在冲突时，CLI 会在执行数据定义语句（DDL）、标记迁移未完成状态（dirty）前拒绝回滚。错误只说明表和字段，不回显数据值；应用不会自动删除或改名数据。

维护人员须依据业务规则显式处理冲突，或继续运行兼容当前数据库结构的应用；不要跳过检查、直接执行回滚 SQL。该检查不阻止并发业务写入，所以不能替代停写。

加密配置的 `is_secret` 回滚同样有数据保护检查。实现与测试分别见 `pkg/migrate/preflight.go` 和 `tests/integration/rollback_test.go`。
