# Grove 命令参考

所有命令从仓库根目录执行。先确认 Go 版本符合项目要求，再运行 `make help`。

```bash
export PATH="/Users/zhimma/.local/share/mise/installs/go/1.25.12/bin:$PATH"
export GOTOOLCHAIN=local
make help
```

## 启动服务

- `make run.api`：启动 API，默认监听 `:8080`
- `make run.console`：启动 Console，默认监听 `:8081`
- `make run.worker`：启动 Worker，默认监听 `:8082`
- `make admin.dev`：启动管理后台前端，开发配置默认监听 `:5666`

Worker 只有在启用 Job 或 Scheduler 后才应启动；默认配置不会让 Worker 空运行。

## 数据库和种子

- `make migrate.up`：执行全部待执行迁移
- `make migrate.down`：回滚最近一个迁移
- `make migrate.status`：查看迁移状态
- `make seed.bootstrap`：创建基础配置和 root 管理员，不覆盖已有 root 密码
- `make seed.demo`：写入开发/测试演示数据，production 环境拒绝执行

迁移和 seed 会根据 `databases.default.driver` 选择对应方言目录：

```text
database/migrations/postgres 或 database/migrations/mysql
database/seeds/postgres 或 database/seeds/mysql
```

典型顺序：

```bash
make migrate.up
make seed.bootstrap
```

## 代码与验证

- `make test`：Go 全量测试
- `make fmt`：格式化 Go 代码
- `make tidy`：整理 Go modules；只在依赖变更后使用
- `make build`：构建 `bin/api`、`bin/console`、`bin/worker`
- `make verify`：Go 测试、后端构建、Console typecheck
- `make admin.install`：按 lockfile 安装前端依赖
- `make admin.typecheck`：Console TypeScript 类型检查
- `make admin.build`：Console production build

高风险或合并前建议额外运行：

```bash
go test -race ./...
go vet ./...
govulncheck ./...
go test -tags=integration ./tests/integration -v
```

## CLI

`cmd/grove` 是唯一保留的后端 CLI：

```bash
go run ./cmd/grove about
go run ./cmd/grove doctor
go run ./cmd/grove --help
go run ./cmd/grove migrate create create_articles_table
go run ./cmd/grove make:module Article
go run ./cmd/grove rbac check
go run ./cmd/grove rbac repair --dry-run
```

`rbac repair` 默认是 dry-run，只有显式传入 `--dry-run=false` 才会修改派生 RBAC 数据。`make:module` 不会自动生成前端页面，也不会替代数据库迁移设计。

## 健康与观测端点

每个 HTTP 服务提供：

- `/health/live`：只表示进程存活
- `/health/ready`：检查当前服务已启用的依赖
- `/health`：兼容入口，等价于 live
- `/metrics`：Prometheus exposition（启用 observability 时）

文档入口：

- API：`/docs`、`/docs/openapi.json`
- Console：`/console/docs`、`/console/docs/openapi.json`
