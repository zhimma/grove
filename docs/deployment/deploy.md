# 部署指南

本文档说明 Grove 的基础部署方式。示例以 `console` 服务为主，`api` 与 `worker` 的部署方式相同。

## 部署前提

### 运行环境

- Go 1.25+
- PostgreSQL 14+
- Redis 6+（启用缓存、队列或 worker 时需要）
- Linux systemd 环境，或容器运行环境

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
    password: change-me
    dbname: grove
    ssl_mode: disable

redis:
  enabled: true
  addr: 127.0.0.1:6379

jwt:
  secret: change-me
  issuer: grove

casbin:
  enforcers:
    console:
      enabled: true
      database: default
      mode: rbac
      table_name: console_casbin_rules
```

### 3. 初始化数据库

```bash
go run ./cmd/artisan/main.go migrate up
go run ./cmd/artisan/main.go seed run
```

生产环境上线前应替换默认账号、默认密码和 JWT secret。

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

容器部署时建议通过环境变量覆盖数据库、Redis 和 JWT 配置。

## 运行约束

- `console`、`api`、`worker` 可以独立部署
- `scheduler` 适合单实例运行；多实例场景应明确是否允许重复执行
- `pkg/job` 依赖 Redis；未启用 Redis 时不应启动 worker
- 日志统一由 `pkg/logger` 输出，生产环境建议落盘并接入集中日志系统

## 相关文档

- [快速上手](../guide/quickstart.md)
- [配置说明](../guide/configuration.md)
- [测试策略](../development/testing.md)
