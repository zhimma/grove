# 部署指南

本文档说明 Grove 的基础部署方式。示例以 `console` 服务为主，`api` 与 `worker` 的部署方式相同。

部署验证分为三个层次：

- 本地 `make build` 证明代码可以构建；Dockerfile 静态检查只检查构建脚本内容，不能证明镜像可以构建或运行。
- GitLab CI 证明代码通过了受管 Runner 上实际执行的质量检查。
- PostgreSQL、MySQL、Redis、迁移、浏览器和对象存储，需要在预发布环境（staging）逐项验收。

以上任一层验证通过，都不代表已经上线。

## 部署前提

### 运行环境

- Go 1.27.1+（构建工具链与 CI 固定 1.27.1）
- PostgreSQL 14+ 或 MySQL 8.0.16+
- Redis 6+（启用缓存、队列或 worker 时需要）
- Linux systemd 环境，或容器运行环境

生产环境根据 `databases.default.driver` 自动选择对应的迁移与种子方言目录；不要让 MySQL 执行 PostgreSQL SQL。

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
make admin.install
make verify
make quality
```

该命令会运行：

- Go 测试
- API、Console、Worker 与 `grove` CLI 二进制构建
- 管理后台前端类型检查
- Go 格式与 `go vet`、前端 lint 与循环依赖、当前工作树空白检查

`golangci-lint` 与 `govulncheck` 不必安装：两个目标经 `go run` 固定到 CI 使用的版本（v2.14.0、v1.6.0），首次运行会下载，本机装了别的版本也不受影响。lint 配置使用 v2 格式，原 `gosimple` 检查已由 `staticcheck` 承接，`gofmt` 移入 `formatters`：

```bash
make quality.go.lint
make quality.govuln
```

GitLab CI 会安装并强制执行这两个检查；具备相同工具和前端依赖时，可用 `make ci` 在本地复现完整门禁。

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
bin/grove
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
bin/grove --config /opt/grove/config.yaml migrate status
bin/grove --config /opt/grove/config.yaml migrate up
# 仅在全新环境、并在受保护的 config.yaml 中填写 security.initial_root_password 时执行
bin/grove --config /opt/grove/config.yaml seed bootstrap
```

生产环境上线前必须在受保护的 `config.yaml` 中填写强 JWT 密钥和强 root 初始密码。CLI 只把该密码的 bcrypt 哈希写入数据库；首次登录后立即修改。重复执行基础种子不会覆盖已有 root 账号。

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

Worker 默认在 `worker_port`（`8082`）启动仅用于健康检查与指标采集的内部 HTTP 服务，不提供业务接口。

### 5. 发布管理后台前端

```bash
make admin.install
make admin.build
```

产物在 `web/admin-vben/apps/console/dist`，按静态站点部署。默认同域：`.env.production` 的 `VITE_GLOB_API_URL` 留空，前端直接请求 `/console/v1/...`，由网关反向代理到 Console（`:8081`）。前后端分域部署时，把 `dist/_app.config.js` 里的 `VITE_GLOB_API_URL` 改成 Console 地址（不用重新构建），并把前端域名加进 `cors.allowed_origins`。

## systemd 示例

以 Console 为例，服务文件如下：

```ini
[Unit]
Description=Grove Console
After=network.target

[Service]
Type=simple
WorkingDirectory=/opt/grove
ExecStart=/opt/grove/bin/console -c /opt/grove/config.yaml
Restart=always
RestartSec=5
# 必须大于 config.yaml 中的 server.shutdown_timeout，给 SIGTERM 优雅退出留出时间。
TimeoutStopSec=45
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

```bash
docker build --build-arg SERVICE=console -t grove-console:local .
```

根目录 `Dockerfile` 支持 `api`、`console`、`worker` 三个服务，使用固定版本的 Go 和 Debian 基础镜像，并以非 root 用户运行。不要在镜像中复制 `config.example.yaml` 作为生产配置，也不要通过构建参数传递密钥。更多说明见[后端镜像指南](../../docker/README.md)。

### 受限方式运行容器

```bash
install -o 10001 -g 10001 -m 0400 config.yaml /opt/grove/config.yaml

docker run --rm --read-only \
  --tmpfs /tmp:rw,noexec,nosuid,size=64m \
  --security-opt no-new-privileges:true \
  --cap-drop ALL \
  --mount type=bind,src=/opt/grove/config.yaml,dst=/app/config.yaml,readonly \
  --mount type=volume,src=grove-console-logs,dst=/app/logs \
  --mount type=volume,src=grove-console-storage,dst=/app/storage \
  -p 127.0.0.1:8081:8081 \
  grove-console:local
```

容器部署时挂载已经填写完成且权限受控的 `config.yaml`。`.dockerignore` 会从构建上下文中排除本地配置、日志、存储和前端依赖。

`--read-only` 把根文件系统设为只读，只有 `/tmp`、日志和本地存储显式可写。如使用外部对象存储，可删除本地存储卷。不要为了使容器启动而移除只读、非 root 或 `--cap-drop ALL` 约束。

`api` 应由反向代理或负载均衡器暴露，`console` 默认只绑定受控网络，`worker` 的 `:8082` 健康检查与指标端口只对内部监控网络开放。三个进程均会处理 `SIGTERM`；编排系统的终止宽限期必须大于 `server.shutdown_timeout`，不能过早发送 `SIGKILL`。

密钥不进入镜像、命令行、日志或 Git。优先由密钥管理服务在发布时生成仅容器 UID 可读的配置文件并只读挂载。确需使用已有的环境变量覆盖能力时，由运行平台注入；不要把值打印到 CI 日志、shell 历史或 `docker inspect` 可见的命令参数中。

镜像构建和漏洞扫描的约束与命令见[后端镜像指南](../../docker/README.md)。

## Staging 验收

发布到预发布环境前先完成 CI，并按[预发布验收清单](staging-checklist.md)逐项记录 PostgreSQL、MySQL、Redis、迁移状态、就绪检查、队列、静态文件和浏览器结果。该清单是目标环境的验收步骤，不代表当前本地代码已完成这些验证。

## 健康检查与可观测性

HTTP 服务提供三个健康入口：

- `/health/live`：只表示进程存活，不访问数据库或 Redis。
- `/health/ready`：检查当前服务启用的 PostgreSQL 或 MySQL、Redis 和队列后端；任一依赖失败时返回 `503`。
- `/health`：兼容旧部署，当前等价于存活检查；新部署不要继续使用它作为就绪探针。

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

Prometheus 默认从 `/metrics` 采集 HTTP 请求量、延迟、错误、数据库连接池和任务执行结果。该端点不包含密码、SQL 或令牌，但仍应只对监控网络开放；不要通过公网入口暴露。

配置 `observability.otlp_trace_endpoint` 后，Gin、GORM、Redis、HTTP 客户端和 Asynq 的追踪片段（span）会通过 OpenTelemetry 协议（OTLP）的 HTTP 接口上报。生产环境使用 HTTPS；只有本地采集器未启用 TLS 时才设置 `otlp_insecure: true`。

## 运行约束

- `console`、`api`、`worker` 可以独立部署
- `scheduler` 只由 worker 进程承载，适合单实例运行；多 worker 场景应只启用一个实例，或明确允许重复执行
- `pkg/job` 依赖 Redis；未启用 Redis 时不应启动 worker
- 日志统一由 `pkg/logger` 输出，生产环境建议落盘并接入集中日志系统
- 访问日志包含 `trace_id` 和 `span_id`，可与 OTLP 追踪关联
- Prometheus `/metrics` 应通过网络策略、反向代理允许列表或独立内部入口限制访问
- 本地存储默认私有，只有同时明确 `storage.disks.<name>.public: true` 与 `serve_static: true` 才会注册静态路由；私有对象必须经鉴权下载接口或对象存储的受控签名 URL 访问
- 共享预发布或生产数据库禁止用 `migrate down` 做回滚；先停止或回滚应用版本，确认迁移兼容性后另行制定数据回退方案

## 相关文档

- [快速上手](../guide/quickstart.md)
- [配置说明](../guide/configuration.md)
- [测试策略](../development/testing.md)
- [预发布验收清单](staging-checklist.md)
