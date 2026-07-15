# 部署指南

本文档说明 Grove 的基础部署方式。示例以 `console` 服务为主，`api` 与 `worker` 的部署方式相同。

## 部署前提

### 运行环境

- Go 1.25.12+
- PostgreSQL 14+ 或 MySQL 8.0.16+
- Redis 6+（启用缓存、队列或 worker 时需要）
- Linux systemd 环境，或容器运行环境

生产环境根据 `databases.default.driver` 自动选择对应的 migration/seed 方言目录；不要让 MySQL 执行 PostgreSQL SQL。

MySQL 配置至少包含：

```yaml
databases:
  default:
    driver: mysql
    host: 127.0.0.1
    port: 3306
    user: grove
    password: ""
    dbname: grove
    charset: utf8mb4
    parse_time: true
    loc: Local
```

### 发布前检查

发布前应至少执行：

```bash
make verify
```

该命令会运行：

- Go 测试
- 三个后端二进制构建
- 管理后台前端类型检查

## 二进制部署

### 1. 获取代码并构建

```bash
git clone https://github.com/zhimma/grove.git
cd grove
go mod download
make build
```

构建产物位于：

```text
bin/api
bin/console
bin/worker
```

### 2. 准备配置

```bash
cp config.example.yaml config.yaml
```

至少需要配置：

- `app.env`
- `port`、`console_port`、`worker_port`
- `databases.default`
- `jwt.secret`
- `casbin.enforcers.console`（启用后台权限时）

生产环境示例：

```yaml
app:
  name: grove
  env: production

log:
  level: info
  path: /var/log/grove
  console: false

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

redis:
  enabled: true
  addr: 127.0.0.1:6379

jwt:
  secret: ""
  issuer: grove

casbin:
  enforcers:
    console:
      enabled: true
      database: default
      mode: rbac
      table_name: console_casbin_rules

observability:
  enabled: true
  metrics_enabled: true
  metrics_path: /metrics
  readiness_timeout: 3
  trace_sample_ratio: 0.1
  # 使用 OTLP HTTP collector 时填写完整 traces endpoint。
  otlp_trace_endpoint: https://otel-collector.example.com/v1/traces
  otlp_insecure: false
```

### 3. 初始化数据库

```bash
go run ./cmd/grove migrate up
# 在受保护的 config.yaml 中填写 security.initial_root_password
go run ./cmd/grove seed bootstrap
```

生产环境上线前必须在受保护的 `config.yaml` 中填写强 JWT secret 和强 root 初始密码。CLI 只把该密码的 bcrypt 哈希写入数据库；首次登录后立即修改。已有 root 账号不会被重复 bootstrap 覆盖。

### 4. 启动服务

后台服务：

```bash
./bin/console
```

对外 API：

```bash
./bin/api
```

异步任务：

```bash
./bin/worker
```

Worker 默认在 `worker_port`（`8082`）启动仅用于 health 与 metrics 的内部 HTTP 监听，不提供业务接口。

## systemd 示例

### `console`

示例服务文件：

```ini
[Unit]
Description=Grove Console
After=network.target

[Service]
Type=simple
WorkingDirectory=/opt/grove
ExecStart=/opt/grove/bin/console
Restart=always
RestartSec=5
Environment="APP_ENV=production"

[Install]
WantedBy=multi-user.target
```

部署步骤：

```bash
mkdir -p /opt/grove/bin
cp bin/console /opt/grove/bin/
cp config.yaml /opt/grove/
cp grove-console.service /etc/systemd/system/
systemctl daemon-reload
systemctl enable grove-console
systemctl start grove-console
```

常用检查命令：

```bash
systemctl status grove-console
journalctl -u grove-console -f
```

## 容器部署

### 构建镜像

```dockerfile
FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/console ./app/console/cmd/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /out/console ./console
COPY config.example.yaml ./config.yaml
CMD ["./console"]
```

### 运行容器

```bash
docker build -t grove-console .
docker run --rm -p 8081:8081 grove-console
```

容器部署时挂载已经填写完成的 `config.yaml`，不要依赖环境变量拼装后端配置。

## 健康检查与可观测性

HTTP 服务提供三个健康入口：

- `/health/live`：只表示进程存活，不访问数据库或 Redis。
- `/health/ready`：检查当前服务启用的 PostgreSQL/MySQL、Redis 和 queue 后端；任一依赖失败时返回 `503`。
- `/health`：兼容旧部署，当前等价于 live；新部署不要继续使用它作为 readiness probe。

检查示例：

```bash
curl -fsS http://127.0.0.1:8081/health/live
curl -fsS http://127.0.0.1:8081/health/ready
curl -fsS http://127.0.0.1:8081/metrics
```

Kubernetes 探针示例：

```yaml
livenessProbe:
  httpGet:
    path: /health/live
    port: 8081
readinessProbe:
  httpGet:
    path: /health/ready
    port: 8081
```

Prometheus 默认从 `/metrics` 采集 HTTP 请求量、延迟、错误、数据库连接池和任务执行结果。该端点不包含密码、SQL 或 token，但仍应只对监控网络开放；不要通过公网 ingress 暴露。

配置 `observability.otlp_trace_endpoint` 后，Gin、GORM、Redis、HTTP Client 和 Asynq span 会通过 OTLP HTTP 上报。生产环境使用 HTTPS；只有本地无 TLS collector 才设置 `otlp_insecure: true`。

## 运行约束

- `console`、`api`、`worker` 可以独立部署
- `scheduler` 只由 worker 进程承载，适合单实例运行；多 worker 场景应只启用一个实例，或明确允许重复执行
- `pkg/job` 依赖 Redis；未启用 Redis 时不应启动 worker
- 日志统一由 `pkg/logger` 输出，生产环境建议落盘并接入集中日志系统
- 访问日志包含 `trace_id` 和 `span_id`，可与 OTLP trace 关联
- Prometheus `/metrics` 应通过网络策略、反向代理 allowlist 或独立内部入口限制访问

## 相关文档

- [快速上手](../guide/quickstart.md)
- [配置说明](../guide/configuration.md)
- [测试策略](../development/testing.md)
