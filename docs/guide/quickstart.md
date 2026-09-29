# 快速上手

本指南只维护一个本地配置文件：`config.yaml`。模板是 `config.example.yaml`；后端不会自动读取 `.env`。

## 1. 准备环境

- Go 1.25.14+
- PostgreSQL 14+ 或 MySQL 8.0.16+
- Node.js 20.19+
- pnpm 10.28.2（仅启动前端需要）
- Redis 6+（启用 Cache、Job 或 Worker 时需要）

确认 Go 环境：

```bash
go version                 # 需要 1.25.14 及以上
export GOTOOLCHAIN=local   # 固定使用本机工具链，避免自动下载
```

仓库带有 `.mise.toml`，用 [mise](https://mise.jdx.dev) 管理版本的话执行 `mise install` 即可装齐 Go 与 Node。

本机装了 Docker 的话，数据库和 Redis 可以不手装：

```bash
make deps.up    # PostgreSQL 17 + Redis 7，端口只绑 127.0.0.1
```

`compose.yaml` 与 `config.example.yaml` 的默认值一致（`postgres@127.0.0.1:5432/grove_dev`、空密码），复制出来的 `config.yaml` 不用改数据库段，第 3 步的建库也可以跳过。要 MySQL 就执行 `docker compose --profile mysql up -d --wait mysql`，再把 `driver` 改成 `mysql`、端口改成 `3306`、用户改成 `root`。`make deps.down` 停止，数据卷保留。

## 2. 创建配置

```bash
cp config.example.yaml config.yaml
```

编辑 `config.yaml`（不要把凭据提交到 Git），开发环境至少填写（用 `make deps.up` 的话数据库段保持默认即可）：

```yaml
app:
  env: development

databases:
  default:
    enabled: true
    driver: postgres
    host: 127.0.0.1
    port: 5432
    user: <your-postgres-user>
    password: <your-postgres-password>
    dbname: grove_dev
    ssl_mode: disable

jwt:
  secret: <at-least-32-random-characters>
```

`jwt.secret` 与 `security.config_encryption_key` 不用手造：`go run ./cmd/grove key:generate` 打印一组强随机值，粘进 `config.yaml` 即可。

如果需要后台 API 权限，再打开：

```yaml
casbin:
  enforcers:
    console:
      enabled: true
      database: default
      mode: rbac
      table_name: console_casbin_rules
```

## 3. 初始化数据库

数据库只需创建一次：

```bash
createdb -h 127.0.0.1 -p 5432 -U <your-postgres-user> -W grove_dev
```

如果使用 MySQL，先创建数据库：

```bash
mysql -h 127.0.0.1 -P 3306 -u root -p -e 'CREATE DATABASE grove_dev CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;'
```

并在 `config.yaml` 中设置 `databases.default.driver: mysql`、端口 `3306` 和 MySQL 账号。

执行迁移和基础种子：

```bash
make migrate.up
make seed.bootstrap
```

`seed.bootstrap` 创建 `root` 管理员和基础角色，不会覆盖已有 root 密码。初始密码填写在 `config.yaml` 的 `security.initial_root_password`，CLI 会在写入数据库前生成 bcrypt 哈希；留空时才回退到兼容的 `GROVE_ROOT_PASSWORD`，两者都为空才生成一次性随机密码。

数据库中不会保存明文密码。已有 root 账号不会因修改配置或重复执行 bootstrap 而改变密码，请登录后台后通过账号设置修改。

开发/测试环境如需要演示数据：

```bash
make seed.demo
```

## 4. 启动后端

Console：

```bash
make run.console
```

开发时改成 `make dev.console`（`dev.api`、`dev.worker` 同理），保存代码后自动重新编译并重启。

API（可选）：

```bash
make run.api
```

Worker 只有启用 Job 或 Scheduler 后才启动：

```bash
make run.worker
```

默认端口：

- API：`http://127.0.0.1:8080`
- Console：`http://127.0.0.1:8081`
- Worker health：`http://127.0.0.1:8082`

健康检查：

```bash
curl -fsS http://127.0.0.1:8081/health/live
curl -fsS http://127.0.0.1:8081/health/ready
```

## 5. 启动管理后台前端

首次安装依赖：

```bash
make admin.install
```

开发启动：

```bash
make admin.dev
```

默认地址：`http://127.0.0.1:5666`。前端开发配置位于 `web/admin-vben/apps/console/.env.development`，它只属于 Vite 前端，不是后端配置入口。

## 6. 验证

```bash
make test
make build
make verify
```

高风险改动追加：

```bash
go test -race ./...
go vet ./...
govulncheck ./...
```

下一步阅读：[项目结构](structure.md)、[开发规范](../01-开发规范.md)、[新增 Console 模块](../03-console-新增模块指南.md)。
