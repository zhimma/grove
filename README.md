# Grove

A monorepo web framework for modern full-stack Go apps.

Grove 是一个面向现代全栈 Go 应用的 monorepo Web 框架，提供 `api`、`console`、`worker` 和管理后台前端的统一工程基线。

项目包含：

- Go 后端：`api`、`console`、`worker`
- 管理后台前端：`web/admin-vben/apps/console`
- 基础设施：配置、数据库、迁移、种子、认证、权限、存储、任务、审计日志

适用场景：

- SaaS 管理后台
- 平台运营后台
- 典型 RBAC + CRUD + 配置管理 + 文件上传 + 审计日志场景

## 定位

Grove 采用 `console-first` 的组织方式：

- `console-first` 的通用后台基础框架
- 当前主线聚焦 `console` 管理后台
- 不包含多租户、多后台域或插件化平台抽象
- `merchant` 暂不作为当前基线能力的一部分

## 核心能力

### 后端基础设施

- 配置加载与多环境配置
- Provider 装配
- 多数据库资源访问
- SQL migrations / seeds
- JWT `access + refresh`
- Casbin RBAC
- 请求上下文 `reqctx`
- 统一响应与错误处理
- 文件存储管理
- worker 入口与任务基础能力

### Console 模块

- 登录 / 刷新 / 登出
- 当前管理员信息
- 修改密码 / 更新个人资料
- 工作台
- 管理员管理
- 角色管理
- API 权限分配
- 菜单权限分配
- 系统配置
- 文件上传
- 操作日志 / 登录日志

### Console 权限约定

`console` 权限模型采用运行时路由与前端本地路由协同的方式：

- token 只承载最小身份
- 授权态按请求回库恢复
- API 权限标识统一为 `METHOD + path`
- API 权限清单来自已注册受保护路由
- 展示文案来自 `route.Name(...)`
- 菜单真相源是前端本地路由
- 后端只代存储 `console_roles.menu_keys`

## 适用范围

- 新项目的后台起步模板
- 单租户或平台型 `console`
- 需要快速拥有登录、权限、配置、日志、上传、后台壳子的业务系统

## 非目标范围

- 通用多租户权限平台
- 插件化低代码平台
- 极重型企业中台
- 已完整覆盖 `merchant` / 多后台域统一抽象

## 目录结构

```text
app/
  api/              对外 API 示例与基础链路
  console/          管理后台后端
  worker/           异步任务入口
cmd/
  artisan/          CLI：迁移、seed、代码生成
internal/           共享内部基础设施
pkg/                可复用基础包
database/           SQL migrations 与 seeds
docs/               开发规范与后续文档
web/admin-vben/     管理后台前端 monorepo（当前聚焦 console）
```

## 快速开始

初始化示例配置：

```bash
cp .env.example .env
cp config.example.yaml config.yaml
go mod download
```

### 后端

```bash
make run.api
make run.console
make run.worker
```

常用命令：

```bash
make help
make test
make fmt
make tidy
make build
make verify
make migrate.status
make migrate.up
make seed.run
```

`make verify` 会执行 Go 测试、三个后端二进制构建和管理后台类型检查；CI 也使用同一组验证口径。

### Artisan CLI

`artisan` 支持：

- `about`
- `migrate up/down/status/create`
- `seed run`
- `make:model`：生成共享 GORM model
- `make:service`：生成 `console` service 模板
- `make:handler`：生成 `console` handler 模板
- `make:module`：生成 `console` model + service + handler，并自动注册路由

推荐先看：

```bash
go run ./cmd/artisan/main.go about
```

查看帮助：

```bash
go run ./cmd/artisan/main.go --help
```

### 模板仓库约定

- `config.yaml`、`.env`、`logs/`、`bin/`、`main`、`node_modules/` 都是本地运行产物，不应提交。
- 运行时日志统一使用 `pkg/logger`，底层基于 zerolog；日志消息尽量使用中文，便于后台项目排查。
- 管理后台菜单真相源在前端路由，后端只保存角色的 `menu_keys`，不要重新引入菜单同步命令。

### 管理后台

仓库使用 `web/admin-vben` 作为管理后台前端工作区，当前保留 `console` 应用。

首次安装：

```bash
pnpm install:admin-vben
```

运行方式：

```bash
make admin.install
make admin.dev
make admin.build
make admin.typecheck

# 或直接使用 pnpm
pnpm dev:admin
pnpm build:admin
pnpm typecheck:admin
```

## 设计原则

### 后端分层

- `handler`：只处理 HTTP、参数绑定、响应拼装
- `service`：处理业务逻辑，接受 `context.Context`
- `model`：放共享 GORM 模型和复用查询辅助

### Console 权限

- 身份与授权分离
- token 不承载授权快照
- 当前授权态请求期恢复
- 前端展示态不等于安全边界
- API 权限与菜单权限严格分离

### 前端菜单

- 菜单真相源是前端本地路由
- 路由 `name` 是菜单授权稳定标识
- 后端不维护 `console` 菜单表，不做菜单同步

## 文档索引

- [README](./README.md)
- [docs/01-开发规范.md](./docs/01-%E5%BC%80%E5%8F%91%E8%A7%84%E8%8C%83.md)
- [docs/02-console-架构与权限.md](./docs/02-console-%E6%9E%B6%E6%9E%84%E4%B8%8E%E6%9D%83%E9%99%90.md)
- [docs/03-console-新增模块指南.md](./docs/03-console-%E6%96%B0%E5%A2%9E%E6%A8%A1%E5%9D%97%E6%8C%87%E5%8D%97.md)
- [docs/guide/pkg-components.md](./docs/guide/pkg-components.md)
