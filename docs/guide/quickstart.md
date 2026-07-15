# 快速上手

本文按当前 `console-first` 模板说明最短启动路径。

## 1. 准备环境

- Go 1.25.12+
- PostgreSQL 14+
- Redis 6+（仅在启用队列、缓存或 worker 时需要）
- Node.js 20.19+
- pnpm 10.28.2

## 2. 初始化配置

```bash
cp .env.example .env
cp config.example.yaml config.yaml
go mod download
```

编辑 `config.yaml` 或 `.env`，至少启用默认数据库：

```bash
export DB_PASSWORD='replace-with-your-postgres-password'
export JWT_SECRET='replace-with-at-least-32-random-characters'
```

```yaml
databases:
  default:
    enabled: true
    driver: postgres
    host: 127.0.0.1
    port: 5432
    user: postgres
    password: ${DB_PASSWORD}
    dbname: golang_web
    ssl_mode: disable
```

如果要启用管理后台 RBAC，请同时启用 `casbin.enforcers.console`。

## 3. 初始化数据库

```bash
createdb golang_web
make migrate.up
make seed.bootstrap
# 仅开发/测试环境按需执行
make seed.demo
```

`seed.bootstrap` 只写入基础配置和 root 管理员，不覆盖已有密码。可通过 `GROVE_ROOT_PASSWORD` 指定初始密码；未指定时 CLI 仅在首次创建 root 时显示一次随机密码。`seed.demo` 会写入演示数据，并在 production 环境直接拒绝执行。

启用 Console RBAC 后，可检查数据库角色真相源和 Casbin 派生数据：

```bash
go run ./cmd/grove rbac check
go run ./cmd/grove rbac repair --dry-run
# 确认差异后再执行
go run ./cmd/grove rbac repair --dry-run=false
```

## 4. 启动服务

```bash
make run.console
```

常用服务入口：

```bash
make run.api
make run.console
make run.worker
```

## 5. 启动管理后台前端

```bash
make admin.install
make admin.dev
```

前端应用位于 `web/admin-vben/apps/console`。

## 6. 新增一个 Console 模块

推荐使用 `grove make:module` 生成当前约定的最小模板：

```bash
go run ./cmd/grove migrate create create_articles_table
go run ./cmd/grove make:module Article
```

命令会生成：

- `internal/model/article.go`
- `app/console/internal/service/article.go`
- `app/console/internal/handler/article.go`
- `app/console/internal/router/router.go` 中的路由注册

生成后继续补齐：

- migration 的建表 SQL
- model 字段
- service 业务逻辑
- handler 请求/响应结构
- 前端 API、页面和本地路由

## 7. 验证

```bash
make test
make build
make admin.typecheck
```

运行时日志统一使用 `pkg/logger`，底层基于 zerolog。日志文案尽量使用中文，字段名保持英文 snake_case。
