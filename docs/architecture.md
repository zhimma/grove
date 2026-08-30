# Grove 项目架构

## 一句话理解

Grove 是一个 `api / console / worker` 三入口的 Go 单体脚手架：启动入口负责配置和依赖装配，业务入口负责路由与流程，共享能力放在 `internal` 和 `pkg`，管理后台前端与后端同仓维护。

## 运行拓扑

```text
config.yaml
        │
        ▼
app/*/cmd → config.Load → internal/provider.Provider
        │                         │
        │                         ├─ PostgreSQL or MySQL / Redis
        │                         ├─ JWT / Casbin / Storage
        │                         ├─ Cache / Event / Job / Scheduler
        │                         └─ Observability / Readiness
        ▼
CoreServer → middleware → router → handler → service → model / database
        │
        ├─ API      :8080
        ├─ Console  :8081
        └─ Worker   :8082（health/metrics；任务由 Redis 驱动）
```

## 服务边界

### API

位置：`app/api`

面向对外 API 的示例服务。它可以启用数据库、Redis、认证、任务、存储和 OpenAPI，但不承载 Console 菜单模型。

### Console

位置：`app/console`，前端位置：`web/admin-vben/apps/console`

当前主线服务，包含管理员认证、Session、RBAC、系统配置、文件上传、审计日志和管理页面。

### Worker

位置：`app/worker`

只承载队列消费者和 Scheduler。没有启用 Job 或 Scheduler 时，Worker 会拒绝空运行。

## 依赖装配

`internal/provider.Provider` 是启动期装配对象，不是业务层 Service Locator。

- `provider.APIOptions()`：API 所需依赖
- `provider.ConsoleOptions()`：Console 所需依赖
- `provider.WorkerOptions()`：Worker 所需依赖
- Provider 负责按逆序关闭资源，并暴露 readiness checks。
- handler、service、job 不应接收完整 Provider，只接收实际需要的依赖。

## 请求链路

### 通用链路

1. CoreServer 创建 Gin engine、基础中间件和健康端点。
2. 服务 router 注册公开和受保护路由。
3. middleware 处理 request id、鉴权、限流、body limit、观测和错误出口。
4. handler 绑定请求并调用 service。
5. service 使用数据库、缓存、事件或任务组件完成业务流程。
6. response 统一输出成功或失败 envelope。

### Console 鉴权链路

1. `AdminAuthn` 验证 access token 和 Session。
2. 请求期从数据库恢复管理员状态和当前授权态。
3. `AdminPermission` 生成 `METHOD + full path`。
4. Console Casbin 根据 `admin -> role -> API permission` 判断放行。
5. 菜单权限由前端本地路由 `name` 决定，后端只保存角色的 `menu_keys`。

## 数据与配置边界

- `config.yaml` 是唯一的本地配置文件；默认值直接写在 YAML 中。启动时支持代码明确列出的环境变量覆盖和 `${ENV}` / `${ENV:default}` 展开，但不会自动读取 `.env` 文件。
- PostgreSQL 是默认数据库；MySQL 8.0.16+ 通过同一 GORM/Connections 抽象提供支持。迁移、管理员、角色、Session、系统配置和审计数据的持久化真相源仍是配置选定的关系数据库。
- migration 和 seed 按数据库 driver 分目录，版本号保持一致。
- Redis 是缓存和队列后端，不替代 Session 数据库真相源。
- `console_casbin_rules` 保存 API 权限；菜单不进入 Casbin。
- 敏感系统配置使用加密存储；基础设施密钥必须来自配置文件保护区、环境变量或外部 secret manager。

## 代码放置规则

- 服务专属代码：`app/<service>`
- 启动装配、配置、共享中间件：`internal/*`
- 可复用基础组件：`pkg/*`
- 共享模型：`internal/model`
- SQL 迁移和种子：`database/migrations`、`database/seeds`
- 前端路由和页面：`web/admin-vben/apps/console/src`

## 当前边界与非目标

当前不预先实现：多租户、插件系统、通用数据权限 DSL、Transactional Outbox、通知渠道、Webhook、工作流和 Feature Flag。这些能力只有出现明确业务触发条件后，才进入新的设计和实施计划。

## 进一步阅读

- [开发规范](01-开发规范.md)
- [配置](guide/configuration.md)
- [项目结构](guide/structure.md)
- [Console 权限](02-console-架构与权限.md)
- [Provider 生命周期设计](plans/2026-07-14-provider-lifecycle-config-design.md)
