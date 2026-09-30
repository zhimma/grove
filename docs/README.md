# Grove 文档中心

Grove 文档按“先理解项目，再开发功能，最后运行验证”的顺序组织。当前行为查阅下列指南；完成范围和下一步查阅[状态页](status.md)。`docs/plans/` 中的工作清单和历史讨论不能替代源码事实。

## 事实与进度的读取顺序

1. 当前源码、配置、Makefile、CI 和实际执行结果是事实依据。
2. 架构、开发与运行指南解释当前行为；发生冲突时修正文档。
3. [当前状态](status.md)区分已实现、部分完成、暂缓和待验收。
4. [升级清单](plans/2026-08-29-grove-framework-upgrade-plan.md)保留任务 ID、勾选状态和退出条件；旧审计与历史计划只作背景。

## 推荐阅读路径

### 第一次接触项目

1. [项目架构](architecture.md)
2. [命令参考](commands.md)
3. [快速上手](guide/quickstart.md)
4. [项目结构](guide/structure.md)
5. [开发规范](01-开发规范.md)

### 基于 Grove 建立新项目

[fork 指南](guide/fork.md)：改 module path、示例代码去留、必改配置。

Grove 的[升级清单与历史背景](plans/README.md)用于本仓库维护；下游项目的接入步骤以 fork 指南为准。

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
- 指南记录“现在怎么使用”；状态页记录完成范围；工作清单记录尚待完成的动作和验收条件。
- 按用户要求保留可勾选的升级清单及任务 ID，已完成条目链接到源码或指南；旧计划中的数字、版本和验证记录不作为当前基线。
- 修改能力、验证边界或任务状态时，同步相关指南、状态页与清单；没有实际执行证据的检查不能勾选。
- 文档示例不得包含真实密码、token、固定 JWT secret 或需要猜测的隐式前置条件。
