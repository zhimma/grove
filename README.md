# Grove

Grove 是一个面向中大型项目的 console-first Go 单体脚手架。它借鉴 Laravel 的目录约定和 CLI 体验，但坚持 Go 的显式组合、直接命名和按需抽象。

## 项目定位

适合从零搭建：

- SaaS 管理后台
- 平台运营后台
- RBAC + CRUD + 配置管理 + 文件上传 + 审计日志系统

当前主线是单租户、单 Console 后台；多租户、插件系统、数据权限 DSL、工作流和通知平台不属于当前基线。

## 当前能力

- `api / console / worker` 三个 Go 服务入口
- PostgreSQL/MySQL 迁移、bootstrap/demo seed 和可回滚生命周期
- Console access/refresh token、持久化 Session、退出和强制下线
- Casbin API RBAC、前端路由菜单权限
- 配置、数据库资源、Redis、Cache、Event、Job、Scheduler、Storage
- 统一响应、错误处理、请求校验、上传限制和审计日志
- readiness、Prometheus metrics、OpenTelemetry trace、安全 CI
- Vue 3 + Vite 管理后台前端

## 三分钟了解项目

按以下顺序阅读：

1. [项目架构](docs/architecture.md)
2. [命令参考](docs/commands.md)
3. [快速上手](docs/guide/quickstart.md)
4. [项目结构](docs/guide/structure.md)
5. [开发规范](docs/01-开发规范.md)

基于 Grove 建立新项目：[fork 指南](docs/guide/fork.md)。

了解完成范围和待验收事项：[当前状态与下一步](docs/status.md)。

AI 或自动化工具先读取仓库根目录的 [AGENTS.md](AGENTS.md) 和 [AI 项目上下文](docs/ai/project-context.md)。

## 最短启动路径

```bash
cp config.example.yaml config.yaml
# 编辑 config.yaml，启用 PostgreSQL 或 MySQL 并填写本地凭据
make migrate.up
make seed.bootstrap
make run.console
```

管理后台前端：

```bash
make admin.install
make admin.dev
```

默认端口：API `8080`、Console `8081`、Worker `8082`、前端 `5666`。完整步骤见 [快速上手](docs/guide/quickstart.md)。

## 目录速览

```text
app/api/                         对外 API 服务
app/console/                     管理后台后端
app/worker/                      队列和计划任务
cmd/grove/                       迁移、seed、RBAC、代码生成 CLI
internal/config/                 严格配置加载和服务校验
internal/provider/               启动装配和资源生命周期
internal/model/                  共享 GORM 模型
pkg/                             基础层（跨服务复用的技术能力）
database/migrations/{postgres,mysql}/ 正反向 SQL 迁移
database/seeds/{postgres,mysql}/     bootstrap/demo seed
web/admin-vben/apps/console/     Vue 管理后台
docs/                            canonical 指南、运行手册和历史计划
```

## 开发与验证

```bash
make help
make test
make build
make verify
```

后端 CLI 统一入口：

```bash
go run ./cmd/grove --help
go run ./cmd/grove about
```

关键变更额外运行：

```bash
go test -race ./...
go vet ./...
make quality.govuln
```

路由、权限、OpenAPI、迁移和认证的验证要求见 [AI 变更检查清单](docs/ai/change-checklist.md)。

## 文档导航

- [文档中心](docs/README.md)
- [当前状态与下一步](docs/status.md)
- [架构](docs/architecture.md)
- [命令](docs/commands.md)
- [配置](docs/guide/configuration.md)
- [数据库](docs/guide/database.md)
- [Console 架构与权限](docs/02-console-架构与权限.md)
- [新增 Console 模块](docs/03-console-新增模块指南.md)
- [部署与运维](docs/operations.md)
- [升级清单与历史背景](docs/plans/README.md)
