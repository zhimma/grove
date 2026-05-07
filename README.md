# Grove

A monorepo web framework for modern full-stack Go apps.

Grove 是一个面向现代全栈 Go 应用的 monorepo Web 框架，当前以 `console` 为第一优先级打磨后端与管理后台基线。

它的目标不是做“什么都内置”的重量级平台，而是提供一套足够清晰、足够稳、足够容易继续长业务的起点：

- Go 后端：`api`、`console`、`worker`
- 管理后台前端：`web/admin-vben/apps/console`
- 基础设施：配置、数据库、迁移、种子、认证、权限、存储、任务、审计日志

当前项目已经可以作为一个真实业务的后台起步框架使用，特别适合：

- SaaS 管理后台
- 平台运营后台
- 典型 RBAC + CRUD + 配置管理 + 文件上传 + 审计日志场景

## 当前定位

当前仓库更准确的定位是：

- `console-first` 的通用后台基础框架
- 不是多租户、多后台域、插件化平台的最终形态
- `merchant` 暂不作为当前基线能力的一部分

也就是说，这个仓库当前最重要的事情，是先把 `console` 这条链路做到清晰、稳定、可复用。

## 当前能力

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

### Console 基础业务

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

### Console 权限基线

当前 `console` 权限模型已经切到运行时自洽模式：

- token 只承载最小身份
- 授权态按请求回库恢复
- API 权限标识统一为 `METHOD + path`
- API 权限清单来自已注册受保护路由
- 展示文案来自 `route.Name(...)`
- 菜单真相源是前端本地路由
- 后端只代存储 `console_roles.menu_keys`

## 成熟度评估

从“是否能承接真实后台业务”这个标准看，当前框架已经是可用的。

### 已经比较成熟的部分

- 单仓结构清晰，后端与后台前端协作成本低
- `console` 的认证、权限、菜单过滤、角色授权链路已经闭环
- 基础中后台常见系统模块已经具备
- 有 migration / seed / test / typecheck / build 基本开发闭环
- `artisan` 已具备迁移与基础代码生成能力

### 仍在继续打磨的部分

- 业务模块脚手架能力还偏轻
- README 与框架文档还在持续完善
- `api` 侧目前更偏示例/模板，而不是完整业务域框架
- 仍有少量历史表与 fixture 可继续清理
- 还没有形成完整的 CI/CD、部署、观测标准件说明

### 当前判断

可以把当前仓库理解为：

- 一个已经能起项目的基础框架
- 还不是“全场景通用平台内核”

如果用一个偏主观的成熟度分数来描述，我会给当前状态：

- `7.5/10` 到 `8/10`

## 适合做什么

- 新项目的后台起步模板
- 单租户或平台型 `console`
- 需要快速拥有登录、权限、配置、日志、上传、后台壳子的业务系统

## 暂时不主打什么

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

从模板启动时，建议先复制示例配置，再按需启用数据库、Redis、Casbin 等组件：

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

当前 `artisan` 支持：

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

当前仓库使用 `web/admin-vben` 作为唯一保留的管理后台前端，并已裁成通用 `console` 模板。

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

## 下一阶段路线图

### Phase 1：继续夯实 Console 基线

- 清理 `console` 历史 seed 与迁移残留
- 补充更清晰的权限与模块开发文档
- 统一更多页面的按钮级接口权限判断

### Phase 2：提升框架可扩展性

- 增强 `artisan` 模块脚手架能力
- 提供更标准的 CRUD 模块模板
- 继续沉淀文件上传、日志、配置等公共模式

### Phase 3：再考虑扩展到更多后台域

- 在 `console` 跑稳后，再评估是否把相同模型迁移到 `merchant`
- 抽象多后台域共享的认证、权限、菜单、路由元数据能力

## 开源文档建议阅读顺序

- [README](./README.md)
- [docs/01-开发规范.md](./docs/01-%E5%BC%80%E5%8F%91%E8%A7%84%E8%8C%83.md)
- [docs/02-console-架构与权限.md](./docs/02-console-%E6%9E%B6%E6%9E%84%E4%B8%8E%E6%9D%83%E9%99%90.md)
- [docs/03-console-新增模块指南.md](./docs/03-console-%E6%96%B0%E5%A2%9E%E6%A8%A1%E5%9D%97%E6%8C%87%E5%8D%97.md)
- [docs/guide/pkg-components.md](./docs/guide/pkg-components.md)

## 当前结论

这个仓库已经不是一个“只有壳子”的模板，而是一个：

- 可以支撑真实后台项目启动
- 已经具备现代 `console` 权限模型
- 仍在持续产品化、标准化的基础框架
