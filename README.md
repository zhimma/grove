# Grove

[![CI](https://github.com/zhimma/grove/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/zhimma/grove/actions/workflows/ci.yml)
[![Go](https://img.shields.io/github/go-mod/go-version/zhimma/grove)](go.mod)
[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

**一个带管理后台的 Go 单体脚手架，让新项目从业务开始。**

Grove 把后台开发常用的认证、权限、数据库、上传、日志、队列和管理界面放在同一个仓库。借鉴 Laravel 的开发体验，采用 Go 的显式组合和按需抽象，通过完整 fork / clone 接入新项目。

[快速上手](#快速上手) · [开发文档](docs/README.md) · [创建业务模块](docs/03-console-新增模块指南.md) · [参与贡献](CONTRIBUTING.md) · [反馈问题](https://github.com/zhimma/grove/issues)

## 能做什么

| 能力 | 已有实现 |
| --- | --- |
| 管理后台 | 管理员、终端用户、角色、会话、系统/站点配置、文章示例、审计日志、计划任务页面 |
| 认证与授权 | JWT access/refresh token、持久化 Session、刷新轮换、强制下线、Casbin API 权限与前端菜单授权 |
| HTTP 开发 | Gin 路由、参数校验、统一响应和错误、分页、请求 ID、OpenAPI 与路由契约检查 |
| 数据层 | GORM、PostgreSQL/MySQL、命名连接、事务传递、双方言 SQL 迁移、基础与演示种子 |
| 文件与配置 | Local/S3 存储、上传策略、私有下载、业务配置敏感值加密 |
| 后台任务 | Asynq 队列、同步/异步事件、Cron 调度、后台启停与手动执行 |
| 运维基础 | 结构化日志与轮转、readiness、Prometheus、OpenTelemetry、服务镜像 |
| 开发工具 | 前后端模块生成、密钥生成、RBAC 检查、热重载、本地依赖 Compose、统一 Makefile 命令 |

后端为 **Go + Gin + GORM**；管理后台为 **Vue 3 + TypeScript + Vite + Ant Design Vue**，基于 Vben Admin。

## 适用场景

适合运营后台、内部管理系统，以及需要后台管理能力的业务 API。代码由你的项目自持，可以按需删除示例或替换实现。

当前以单租户、单 Console 后台为基础。邮件、通知、多租户、工作流和通用接口配额限流没有预置；已提供的是登录限流。组件边界与后续方向见[项目范围](docs/status.md)。

## 快速上手

### 1. 准备环境

需要 Go、Node.js/Corepack，以及 PostgreSQL/Redis。仓库固定 Go **1.27.1**、Node.js **20.19.5**；pnpm 由前端 `packageManager` 字段固定。版本分别见 [.mise.toml](.mise.toml) 和 [前端 package.json](web/admin-vben/package.json)。

```bash
git clone https://github.com/zhimma/grove.git
cd grove

# 安装仓库固定的 Go 和 Node.js；已自行安装对应版本可跳过
mise install
```

可通过 [mise](https://mise.jdx.dev) 管理工具链。如果 shell 未启用 mise，以下命令可加 `mise exec --` 前缀，例如 `mise exec -- make help`。

### 2. 启动依赖并配置

已安装 Docker / OrbStack 时：

```bash
make deps.up
cp config.example.yaml config.yaml
go run ./cmd/grove key:generate
```

将生成的值填入 `config.yaml` **已有的** `jwt.secret` 和 `security.config_encryption_key`。模板中的数据库与 Redis 地址已对应 Compose；无需更改。

Compose 仅供本机开发，数据库使用本机回环端口和免密配置。已有数据库、端口冲突或改用 MySQL 的步骤见[快速上手指南](docs/guide/quickstart.md)。后端只读取指定的 YAML 配置及支持的环境变量，不自动加载 `.env`。

### 3. 初始化并启动后台

```bash
make migrate.up
make seed.bootstrap
make run.console
```

初始账号是 **`root`**。未配置初始密码时，首次 `seed.bootstrap` 会生成并显示一次性密码；已有账号的密码不会被重复执行覆盖。

另开一个终端：

```bash
make admin.install
make admin.dev
```

打开 **http://localhost:5666** 登录。开发前端默认连接本机 `8081` 端口的 Console API。

| 入口 | 默认地址 | 用途 |
| --- | --- | --- |
| Console | `http://localhost:8081` | 管理后台 API |
| API | `http://localhost:8080` | 对外业务 API 起点，另行执行 `make run.api` |
| Worker | `http://localhost:8082` | 队列/调度进程的健康端点，另行执行 `make run.worker` |
| OpenAPI | `http://localhost:8081/console/docs` | Console 接口文档，受 `docs.enabled` 控制 |

确认后端已就绪：

```bash
curl -fsS http://localhost:8081/health/ready
```

日常开发可用 `make dev.console`、`make dev.api`、`make dev.worker` 热重载。Worker 需要启用队列或 Scheduler；后台可管理的计划任务还需要数据库和已执行的迁移。

## 创建一个业务模块

在新项目中，从字段定义生成前后端基础代码：

```bash
go run ./cmd/grove make:module Invoice --label 发票 \
  --fields "title:string:required,amount:int,paid:bool,due_at:time"
```

生成内容包括：

- PostgreSQL/MySQL 迁移、共享模型、分页 CRUD service 与测试。
- Handler、路由注册、权限名称和 OpenAPI 声明。
- 前端 API、契约登记、管理页面和菜单路由（保留 Console 前端时）。

生成器读取目标项目的 `go.mod`，支持 fork 后改 module path。生成后还需补充业务规则、审查并执行迁移，以及验证页面和权限。具体步骤见[新增 Console 模块](docs/03-console-新增模块指南.md)。

## 项目结构

```text
app/
  api/                  对外 API；服务入口与内部代码
  console/              管理后台后端
  worker/               队列消费者与计划任务
cmd/grove/              迁移、种子、代码生成等 CLI
internal/               仓库内配置、装配、模型、观测和测试辅助
pkg/                    跨服务共用的技术组件
database/               按 PostgreSQL/MySQL 分层的迁移与种子
web/admin-vben/         管理后台前端工作区
docs/                   当前实现的开发与运行指南
```

请求通常沿着 `router → handler → service → model/database` 执行；依赖在启动层显式装配。`pkg/` 不对外发布，也不得反向依赖 `internal/` 或 `app/`。详见[架构](docs/architecture.md)与[目录职责](docs/guide/structure.md)。

## 验证与部署

```bash
make help              # 全部开发命令
make verify            # Go 测试、二进制构建、前端类型检查
make contracts         # 路由 / OpenAPI / 前端接口契约
make ci                # 完整本地质量门禁，先安装前端依赖
```

真实数据库与 Redis 集成测试需要单独运行，见[测试指南](docs/development/testing.md)。`make ci` 不包含容器构建或完整浏览器验收。

```bash
docker build --build-arg SERVICE=console -t grove-console:local .
```

`SERVICE` 支持 `api`、`console`、`worker`。运行时挂载自己的 `config.yaml`；镜像以非 root 用户运行。生产配置、反向代理、前端发布和数据回退要求见[部署指南](docs/deployment/deploy.md)。

## 参与项目

欢迎提交可复现的问题、文档修正和有明确使用场景的改进：

- [Issues](https://github.com/zhimma/grove/issues)：问题反馈与需求讨论。
- [Pull requests](https://github.com/zhimma/grove/pulls)：代码和文档贡献，提交前请阅读[贡献指南](CONTRIBUTING.md)。
- [Fork 指南](docs/guide/fork.md)：更换项目标识、处理示例、维护上游修复。

## 致谢与许可

感谢 [Gin](https://github.com/gin-gonic/gin)、[GORM](https://github.com/go-gorm/gorm)、[Casbin](https://github.com/casbin/casbin)、[Asynq](https://github.com/hibiken/asynq)、[Vben Admin](https://github.com/vbenjs/vue-vben-admin) 及其他依赖项目。

Grove 自身代码采用 [MIT 许可证](LICENSE)，允许商业使用、修改、分发、再许可和销售。分发软件副本或实质性部分时，须保留版权声明和许可证全文；软件按原样提供，不附带担保。

管理后台保留 Vben Admin 的 [MIT 许可证与版权声明](web/admin-vben/LICENSE)。第三方代码和依赖仍遵循各自的许可证。
