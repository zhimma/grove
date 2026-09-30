# 项目结构

本文档说明 Grove 的目录组织方式，以及各目录的职责边界。整体架构先看 [项目架构](../architecture.md)，命令先看 [命令参考](../commands.md)。

## 结构概览

```text
grove/
├── app/                  # 服务应用
├── cmd/                  # CLI 入口
├── internal/             # 仅仓库内部使用的基础设施
├── pkg/                  # 基础层（跨服务复用的技术能力）
├── database/             # SQL migrations 与 seeds
├── docs/                 # 文档
├── web/                  # 前端工作区
├── config.example.yaml   # 配置示例
├── go.mod                # Go 模块定义
└── Makefile              # 常用开发命令
```

## 顶层目录

### `app/`

`app/` 用于放置服务应用。当前包含三个入口：

- `app/api`：对外 API 示例与基础链路
- `app/console`：管理后台后端
- `app/worker`：异步任务入口

每个服务目录的组织方式相同：`cmd/` 是启动入口，其余代码都在 `internal/` 下，其他服务无法导入：

- `internal/handler/`：处理 HTTP 或任务输入输出
- `internal/service/`：业务逻辑（api、console）
- `internal/middleware/`：认证、权限、审计等中间件（api、console）
- `internal/router/`：路由注册（api、console）
- `internal/docs/`：OpenAPI 声明（api、console）
- `internal/server/`：服务装配
- `internal/task/`：可在后台管理的计划任务注册表（worker），见[计划任务](./scheduler.md#后台管理)

### `cmd/`

`cmd/` 用于放置独立 CLI。当前保留：

- `cmd/grove`：迁移、seed、RBAC 一致性检查、代码生成与环境信息查看

### `internal/`

`internal/` 放置只在仓库内部复用的基础设施：

- `bootstrap/`：服务启动期公共装配
- `config/`：配置加载与配置类型；`load.go` 读文件，`defaults.go` 默认值，`env.go` 环境覆盖，`normalize.go` 归一化，`validate.go` 校验
- `docsui/`：文档页与 OpenAPI 页面基础能力
- `middleware/`：通用 Gin 中间件
- `model/`：共享 GORM 模型
- `datatype/`：数据库字段类型
- `jobtask/`：生产者和消费者共用的任务名与 payload，不放队列实现
- `testkit/`：共享数据库测试夹具
- `observability/`：OpenTelemetry 追踪与指标
- `provider/`：数据库、缓存、存储、认证等资源装配
- `readiness/`：就绪探针
- `server/`：HTTP 服务生命周期与优雅关闭

### `pkg/`

`pkg/` 是本仓库的基础层：跨 api / console / worker 复用的技术能力。它**不是对外发布的通用库**，因此允许出现 console、admin 这类本仓库的领域词（例如 `auth.UserTypeConsole`、`rbac.CheckConsolePermission`）和中文文案。

仍然成立的唯一硬约束：**不得反向依赖 `internal/` 或 `app/`**，用来固定依赖方向。当前包含（具体 API 见[基础组件](pkg-components.md)）：

- `auth`
- `cache`
- `rbac`
- `database`
- `event`
- `errx`
- `httpclient`
- `job`
- `logger`
- `migrate`
- `pagination`
- `password`
- `ratelimit`
- `secretbox`
- `permission`
- `request`
- `response`
- `route`
- `scheduler`
- `storage`
- `transaction`
- `ulid`
- `validation`

### `database/`

`database/` 用于维护数据库变更与初始化数据：

- `migrations/`：正反向 SQL 迁移文件
- `seeds/`：初始化账号、配置等种子数据

### `web/`

`web/` 用于放置前端工作区。当前保留：

- `web/admin-vben`：管理后台前端 monorepo

主应用为 `web/admin-vben/apps/console`。Vben workspace 保留，业务源码位于 `src/`：

- `api/`：按资源拆分，`request.ts` 是传输入口，`console-contract.*` 是接口登记。
- `types/pagination.ts`：唯一的列表参数、元数据和分页结果类型。
- `views/system/{admins,users,roles,sessions,scheduled-tasks,operation-logs,login-logs}/index.vue`：系统管理。
- `views/configs/{system,site}/index.vue` 与 `configs/components/ConfigForm.vue`：配置页面及领域表单。
- `views/content/articles/index.vue`、`views/dashboard/overview/index.vue`：内容与工作台。
- `views/<模块复数>/index.vue`：生成的新业务模块。
- `components/resource-page/`：通用列表和编辑行为，自定义表单通过组件参数传入。
- `router/routes/modules/`：按业务域注册页面；系统管理保持唯一父路由，路由名是菜单授权 key。

CLI 在 `cmd/grove` 中按命令拆为 `migrate.go`、`seed.go`、`make_module.go`、`key.go` 等；`main.go` 只启动 Cobra。所有文件仍属于 `package main`。

### `docs/`

`docs/` 按维护职责分为：

- `architecture.md`、`commands.md`、`operations.md`：canonical 总览和运行手册
- `guide/`：配置、数据库、路由、组件和领域指南
- 根目录编号文档：开发规范与 Console 核心约定
- `deployment/`、`development/`：部署、测试和错误处理专题
- `ai/`：AI 项目上下文与变更清单
- `plans/`：设计计划、历史决策和完成审计

## 组织原则

### 服务代码放在 `app/*`

- 服务专属 handler、service、router 只放在对应应用目录
- 不把 `console` 业务代码放进 `pkg/`

### 通用能力放在 `pkg/*`

- `pkg/*` 只提供基础能力
- 不在 `pkg/*` 放业务流程、页面或示例任务协议；允许身份类型等现有框架语义，但不能反向导入 `app/` 或 `internal/`

### 共享装配放在 `internal/*`

- 配置、Provider、文档页、共享中间件等放在 `internal/*`
- 这些能力服务于整个仓库，但不对外承诺稳定 API

### 前后端同仓维护

- 后端与管理后台前端在同一仓库维护
- 前端路由与后端权限模型协同设计
- 菜单真相源在前端本地路由，不引入后端菜单表

## 新增代码时的放置规则

- 新增后台业务模块：放在 `app/console`
- 新增对外 API 示例或接口：放在 `app/api`
- 新增异步任务处理逻辑：放在 `app/worker`
- 新增可在后台调整调度的计划任务：放在 `app/worker/internal/task`
- 新增共享模型：放在 `internal/model`
- 新增通用基础组件：放在 `pkg/*`
- 新增服务装配能力：放在 `internal/provider` 或 `internal/bootstrap`

## 相关文档

- [快速上手](./quickstart.md)
- [项目架构](../architecture.md)
- [命令参考](../commands.md)
- [开发规范](../01-%E5%BC%80%E5%8F%91%E8%A7%84%E8%8C%83.md)
- [pkg 基础组件](./pkg-components.md)
