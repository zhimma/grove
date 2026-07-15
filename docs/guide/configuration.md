# 配置说明

本文档说明 Grove 的配置来源、核心配置项和使用方式。

## 配置来源

Grove 只维护一个本地配置文件：`config.yaml`。

`config.example.yaml` 是模板，不参与运行。复制后直接编辑 `config.yaml`；Grove 不会自动读取 `.env` 文件，配置模板也不使用环境变量占位符。

YAML 使用严格字段校验：未知字段、字段拼写错误和多个 YAML document 都会导致启动失败。

## 最短路径

### 示例配置

```yaml
app:
  name: grove
  env: development
  debug: true

port: 8080
console_port: 8081
worker_port: 8082

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

jwt:
  secret: ""
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

## 配置分组

### `app`

- `name`：应用名称
- `env`：运行环境
- `debug`：调试开关；直接在 `config.yaml` 设置。未显式配置时，`production` 默认为 `false`，其他环境默认为 `true`。

### `port` / `console_port`

- `port`：`api` 服务端口
- `console_port`：`console` 服务端口
- `worker_port`：`worker` 的 health/metrics 端口；Worker 不提供业务 API

### `server`

HTTP 服务级限制：

- `shutdown_timeout`
- `read_timeout`
- `write_timeout`
- `max_header_bytes`
- `max_body_bytes`

### `log`

- `level`：日志级别
- `path`：日志目录
- `console`：是否输出到标准输出
- `service`：日志中的服务名

### `databases.default`

默认数据库连接。PostgreSQL 是默认驱动，也支持 MySQL 8.0.16+。

MySQL 使用 `driver: mysql`，并建议配置 `charset: utf8mb4`、`parse_time: true`、`loc: Local`。

### `databases.resources`

命名数据源集合，用于多数据库资源场景。

### `redis`

Redis 连接配置。启用缓存、队列或 worker 时需要。

### `job`

- `enabled`：是否启用 Asynq 队列
- `concurrency`：Worker 并发数
- `queues`：队列权重；启用 Job 时必须同时启用 Redis

### `scheduler`

- `enabled`：是否在 Worker 进程启用计划任务调度器，默认 `false`
- `timezone`：IANA 时区名，例如 `Asia/Shanghai`，默认 `Local`

直接在 `config.yaml` 设置 `enabled` 和 `timezone`。API 和 Console 不会自动承载 Scheduler。

### `jwt`

- `secret`：签名密钥
- `issuer`：签发者
- `access_expiry_hours`
- `refresh_expiry_hours`

### `casbin.enforcers`

定义权限执行器。`console` 使用独立的 Casbin 表。

### `storage`

定义默认存储磁盘与各磁盘配置。

### `observability`

- `enabled`：是否启用观测运行时
- `metrics_enabled`：是否暴露 Prometheus metrics
- `metrics_path`：metrics 路径，默认 `/metrics`
- `readiness_timeout`：readiness 依赖检查超时
- `trace_sample_ratio`：Trace 采样比例
- `otlp_trace_endpoint`、`otlp_insecure`：OTLP HTTP trace 导出配置

### `docs`

控制 Scalar/OpenAPI 文档：

- API 文档页面：`/docs`
- API OpenAPI JSON：`/docs/openapi.json`
- Console 文档页面：`/console/docs`
- Console OpenAPI JSON：`/console/docs/openapi.json`
- `base_path` 只作为 API 文档默认业务前缀；Console 固定使用 `/console/v1`

### `cors`、`api`、`demo`、`security`

- `cors`：跨域开关、来源、方法、请求头和凭据策略
- `api`：API 前缀、默认分页和最大分页大小
- `demo`：开发/测试演示接口；production 始终忽略
- `security.trusted_proxies`：显式信任的代理地址或 CIDR
- `security.hsts_enabled`：是否启用 HSTS
- `security.config_encryption_key`：系统配置敏感值加密密钥
- `security.initial_root_password`：首次 bootstrap 使用的 root 初始密码；仅用于生成 bcrypt 哈希
- `security.login`：登录限流与锁定参数

### root 初始密码

在 `config.yaml` 中配置：

```yaml
security:
  initial_root_password: 'replace-with-your-password'
```

该字段只用于首次 `make seed.bootstrap`。CLI 会将它转换为 bcrypt 哈希后写入 `console_admins.password`，不会把明文写入数据库；已有 root 管理员也不会被重复 bootstrap 覆盖。

留空时可临时兼容 `GROVE_ROOT_PASSWORD`，两者都未提供时生成一次性随机密码并只输出一次。生产环境应使用强密码，并在首次登录后修改。

## 使用约定

- 生产环境必须替换 `jwt.secret`。
- `api`、`console`、`worker` 使用各自的服务级配置校验，未知 service 名不会被静默接受。
- production Console 必须启用默认数据库和 `casbin.enforcers.console`。
- Worker 必须至少启用 Job 或 Scheduler；启用 Job 时必须同时启用 Redis。
- 已启用的数据库必须配置 `driver`、`host`、`port`、`user` 和 `dbname`。
- `driver` 只能是 `postgres` 或 `mysql`；MySQL 必须配置 `charset` 和 `loc`。
- 已启用的 Casbin enforcer 必须引用已启用的数据库。
- 生产环境必须保持 `app.debug=false`，避免响应体暴露底层错误信息。
- 本地开发直接编辑未提交的 `config.yaml`；生产环境挂载受保护的、已填写敏感值的配置文件。
- 多数据库资源命名应体现业务语义，例如 `orders`、`crm`。
- 未启用的组件应显式保持 `enabled: false`。

## 边界

- 配置系统不提供远程配置中心。
- Grove 不在配置层实现多环境继承语法。
- 复杂部署环境的配置分发由外部系统负责。

## 相关文档

- [快速上手](./quickstart.md)
- [部署指南](../deployment/deploy.md)
- [命令参考](../commands.md)
