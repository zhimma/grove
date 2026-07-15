# Grove AI 项目上下文

## 项目身份

Grove 是 Go 单体脚手架，不是 Java 企业框架的 Go 翻译版。它借鉴 Laravel 的目录和 CLI 体验，但偏好显式依赖、小接口、直接函数和标准库能力。

## 事实源优先级

发生冲突时按以下顺序判断：

1. 当前代码、测试、迁移和配置
2. Makefile、CI 和实际命令输出
3. canonical 文档：`docs/architecture.md`、`docs/guide/`、`docs/operations.md`
4. `docs/plans/` 中的历史设计和审计

## 目录语义

- `app/api`：对外 API 示例服务
- `app/console`：管理后台后端
- `app/worker`：队列消费者和计划任务
- `cmd/grove`：迁移、seed、RBAC 检查、代码生成 CLI
- `internal/config`：严格 YAML 配置、服务级校验和数据库方言归一化
- `internal/provider`：启动期依赖装配和生命周期
- `internal/model`：共享 GORM 模型
- `pkg/*`：Cache、Event、HTTP Client、Storage、Job、Permission 等通用能力
- `database/migrations/{postgres,mysql}`：按数据库方言分层的正反向 SQL 迁移
- `database/seeds/{postgres,mysql}`：按数据库方言分层的 bootstrap/demo seed
- `web/admin-vben/apps/console`：Vue/Vite 管理后台

## 关键真相源

- 数据库：PostgreSQL 默认，MySQL 8.0.16+ 可选
- Console Session：数据库 `console_sessions`
- API 权限目录：运行时受保护路由扫描
- API 权限规则：Console Casbin
- 菜单目录：前端本地路由
- 菜单授权结果：角色 `menu_keys`
- 配置：`config.yaml`；默认值直接写入 YAML，不使用环境变量占位符
- migration/seed：按数据库 driver 选择对应方言子目录，版本号保持一致
- readiness：Provider 实际成功装配的依赖集合

## 不要默认做的事

- 不新增通用 Repository、BaseService、ServiceContainer 或字符串依赖注入。
- 不把菜单复制到后端表，也不引入权限同步命令。
- 不把 refresh token 明文写入数据库、日志或 metrics。
- 不让 handler 直接 new 数据库、Redis、Job 或 Casbin。
- 不因为一个 CRUD 模块就引入 Factory/Strategy/State。
- 不把 Task 24–29 的 Deferred 能力提前实现。

## 新增 Console 模块的最小链路

1. 迁移（如需要）
2. `internal/model`
3. `app/console/internal/service`
4. `app/console/internal/handler`
5. `app/console/internal/router`
6. `.Name("模块.动作")` 和受保护路由
7. 前端 API、页面、本地路由和按钮权限
8. 单测、路由/OpenAPI contract、必要时 race

详细步骤见 [新增 Console 模块](../03-console-新增模块指南.md)。
