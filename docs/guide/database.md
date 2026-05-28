# 数据库与模型

本文档说明 Grove 中数据库连接、共享模型和迁移的基本用法。

## 适用范围

当前数据库能力基于 GORM，支持：

- 默认数据库连接
- 命名数据库资源
- SQL 迁移与 seeds
- 共享模型定义

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
    password: postgres
    dbname: grove
    ssl_mode: disable
```

### 在服务层获取连接

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

## 使用约定

- 共享模型放在 `internal/model`。
- 服务层通过 `p.DB` 获取数据库连接，不在 handler 中直接操作数据库。
- 需要多数据源时使用 `databases.resources`，不要在业务代码里手工创建连接。
- 迁移文件使用正反向 SQL，按时间戳命名。

## 模型边界

- 模型负责字段定义与通用查询辅助。
- 模型不负责权限判断、HTTP 响应拼装或业务流程编排。
- 复杂业务逻辑应放在 `service`，而不是 GORM hook。

## 相关文档

- [开发规范](../01-%E5%BC%80%E5%8F%91%E8%A7%84%E8%8C%83.md)
- [服务层](./service.md)
