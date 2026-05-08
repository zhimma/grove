# 配置说明

本文档说明 Grove 的配置来源、核心配置项和使用方式。

## 配置来源

Grove 默认按以下顺序读取配置：

1. `config.yaml`
2. `.env`
3. 环境变量覆盖

配置文件支持 `${VAR:default}` 语法。

## 最短路径

### 示例配置

```yaml
app:
  name: grove
  env: development
  debug: true

port: 8080
console_port: 8081

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

jwt:
  secret: change-me
  issuer: grove
  access_expiry_hours: 24
  refresh_expiry_hours: 168
```

### 代码中读取配置

```go
cfg, err := config.Load()
if err != nil {
	return err
}
```

## 核心配置项

### `app`

- `name`：应用名称
- `env`：运行环境
- `debug`：调试开关；未显式配置时，`production` 默认为 `false`，其他环境默认为 `true`。可用 `APP_DEBUG=true/false/1/0/yes/no` 覆盖。

### `port` / `console_port`

- `port`：`api` 服务端口
- `console_port`：`console` 服务端口

### `databases.default`

默认数据库连接。当前主驱动为 Postgres。

### `databases.resources`

命名数据源集合，用于多数据库资源场景。

### `redis`

Redis 连接配置。启用缓存、队列或 worker 时需要。

### `jwt`

- `secret`：签名密钥
- `issuer`：签发者
- `access_expiry_hours`
- `refresh_expiry_hours`

### `casbin.enforcers`

定义权限执行器。`console` 使用独立的 Casbin 表。

### `storage`

定义默认存储磁盘与各磁盘配置。

## 使用约定

- 生产环境必须替换 `jwt.secret`。
- 生产环境建议保持 `app.debug=false`，避免响应体暴露底层错误信息。
- 推荐把敏感信息放到环境变量，不直接写入版本库。
- 多数据库资源命名应体现业务语义，例如 `orders`、`crm`。
- 未启用的组件应显式保持 `enabled: false`。

## 边界

- 配置系统不提供远程配置中心。
- Grove 不在配置层实现多环境继承语法。
- 复杂部署环境的配置分发由外部系统负责。

## 相关文档

- [快速上手](./quickstart.md)
- [部署指南](../deployment/deploy.md)
