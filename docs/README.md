# Grove 文档中心

Grove 文档按“先理解项目，再开发功能，最后运行验证”的顺序组织。日常开发只需要关注下列 canonical 文档；`docs/plans/` 是设计决策和历史实施记录，不是重复的入门手册。

## 推荐阅读路径

### 第一次接触项目

1. [项目架构](architecture.md)
2. [命令参考](commands.md)
3. [快速上手](guide/quickstart.md)
4. [项目结构](guide/structure.md)
5. [开发规范](01-开发规范.md)

升级审查与实施记录：[Grove Web 框架组件与工程规范升级计划](plans/2026-08-29-grove-framework-upgrade-plan.md)

### 开发 Console 模块

1. [Console 架构与权限](02-console-架构与权限.md)
2. [新增 Console 模块](03-console-新增模块指南.md)
3. [路由与控制器](guide/routing.md)
4. [响应与错误处理](04-响应与错误处理规范.md)
5. [权限控制](guide/permission.md)

### 使用基础设施

- [配置](guide/configuration.md)
- [数据库与迁移](guide/database.md)
- [基础组件](guide/pkg-components.md)
- [缓存](guide/cache.md)
- [事件](guide/event.md)
- [队列](guide/queue.md)
- [计划任务](guide/scheduler.md)
- [HTTP Client](guide/httpclient.md)
- [Console 日志与审计](guide/logging.md)

### 测试、部署与维护

- [命令参考](commands.md)
- [测试策略](development/testing.md)
- [错误处理实践](development/error-handling.md)
- [部署与运行](deployment/deploy.md)
- [Staging smoke 清单](deployment/staging-checklist.md)
- [运行状态与可观测性](operations.md)

### AI 协作

- [AI 文档入口](ai/README.md)
- [项目上下文](ai/project-context.md)
- [变更检查清单](ai/change-checklist.md)
- 仓库级短规则：[AGENTS.md](../AGENTS.md)

## 文档维护规则

- 代码、配置、Makefile、CI 是运行事实源；文档不能创造与代码不一致的命令。
- 新增稳定能力先更新对应 canonical guide，再在需要时补充设计计划。
- 设计计划记录“为什么这样做”；指南记录“现在怎么使用”。
- 历史计划保留原始状态，不把已废弃方案继续放在快速开始路径上。
- 文档示例不得包含真实密码、token、固定 JWT secret 或需要猜测的隐式前置条件。
