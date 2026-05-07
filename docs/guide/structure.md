# 项目结构

本文档说明 Grove 的目录组织方式，以及各目录的职责边界。

## 结构概览

```text
grove/
├── app/                  # 服务应用
├── cmd/                  # CLI 入口
├── internal/             # 仅仓库内部使用的基础设施
├── pkg/                  # 可复用基础组件
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

每个服务目录保持相同的基本组织方式：

- `cmd/`：服务启动入口
- `handler/`：处理 HTTP 或任务输入输出
- `service/`：业务逻辑
- `internal/router/`：路由注册
- `internal/server/`：服务装配

`console` 额外包含：

- `middleware/`：后台认证、权限、审计等中间件

### `cmd/`

`cmd/` 用于放置独立 CLI。当前保留：

- `cmd/artisan`：迁移、seed、代码生成与环境信息查看

### `internal/`

`internal/` 放置只在仓库内部复用的基础设施：

- `bootstrap/`：服务启动期公共装配
- `config/`：配置加载与配置类型
- `docsui/`：文档页与 OpenAPI 页面基础能力
- `middleware/`：通用 Gin 中间件
- `model/`：共享 GORM 模型
- `provider/`：数据库、缓存、存储、认证等资源装配

### `pkg/`

`pkg/` 放置可复用基础组件，不承载具体业务语义。当前包含：

- `auth`
- `cache`
- `casbin`
- `database`
- `event`
- `httpclient`
- `job`
- `logger`
- `migrate`
- `permission`
- `reqctx`
- `response`
- `route`
- `scheduler`
- `server`
- `storage`
- `transaction`
- `validation`

### `database/`

`database/` 用于维护数据库变更与初始化数据：

- `migrations/`：正反向 SQL 迁移文件
- `seeds/`：初始化账号、配置等种子数据

### `web/`

`web/` 用于放置前端工作区。当前保留：

- `web/admin-vben`：管理后台前端 monorepo

主应用为：

- `web/admin-vben/apps/console`

### `docs/`

`docs/` 按用途分为三类：

- 根目录编号文档：开发规范与 console 核心约定
- `guide/`：使用指南
- `deployment/` 与 `development/`：部署、测试与错误处理说明

## 组织原则

### 服务代码放在 `app/*`

- 服务专属 handler、service、router 只放在对应应用目录
- 不把 `console` 业务代码放进 `pkg/`

### 通用能力放在 `pkg/*`

- `pkg/*` 只提供基础能力
- 不在 `pkg/*` 中引入管理后台、业务模块或页面概念

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
- 新增共享模型：放在 `internal/model`
- 新增通用基础组件：放在 `pkg/*`
- 新增服务装配能力：放在 `internal/provider` 或 `internal/bootstrap`

## 相关文档

- [快速上手](./quickstart.md)
- [开发规范](../01-%E5%BC%80%E5%8F%91%E8%A7%84%E8%8C%83.md)
- [pkg 基础组件](./pkg-components.md)
