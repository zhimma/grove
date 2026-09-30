# Grove 开发文档

先按 [README](../README.md) 启动项目，再按当前任务查阅下表。文档描述现有代码的行为；源码、配置、Makefile 和实际执行结果是最终依据。

## 入门与开发

| 任务 | 文档 |
| --- | --- |
| 第一次启动、改用 MySQL、排查环境问题 | [快速上手](guide/quickstart.md) |
| 通过 fork / clone 建立业务项目 | [Fork 指南](guide/fork.md) |
| 理解服务边界与请求链路 | [架构](architecture.md)、[目录职责](guide/structure.md) |
| 了解已有能力与项目边界 | [项目范围](status.md) |
| 编码与提交流程 | [开发规范](01-开发规范.md)、[贡献指南](../CONTRIBUTING.md) |
| 新增一个前后端模块 | [Console 新增模块](03-console-新增模块指南.md) |
| 使用 CLI 和开发命令 | [命令参考](commands.md) |

## 基础能力

| 任务 | 文档 |
| --- | --- |
| 修改配置与密钥 | [配置](guide/configuration.md) |
| 数据库、事务、迁移与回滚 | [数据库](guide/database.md) |
| 身份、Session、角色、菜单和 API 权限 | [Console 架构与权限](02-console-架构与权限.md) |
| 响应、错误码与请求校验 | [响应与错误](04-响应与错误处理规范.md) |
| 查找基础组件入口 | [组件索引](guide/pkg-components.md) |
| 缓存、进程内事件、外部 HTTP | [缓存](guide/cache.md)、[事件](guide/event.md)、[HTTP Client](guide/httpclient.md) |
| 队列与计划任务 | [队列](guide/queue.md)、[Scheduler](guide/scheduler.md) |
| 操作日志与登录审计 | [审计日志](guide/logging.md) |

## 测试与运行

- [测试指南](development/testing.md)：本地门禁、数据库/Redis 集成和生成器测试边界。
- [部署指南](deployment/deploy.md)：二进制、容器、反向代理和前端发布。
- [运行与观测](operations.md)：健康探针、日志、指标、追踪和关闭行为。
- [发布验收清单](deployment/staging-checklist.md)：在自己的环境逐项验证。
- [后端镜像](../docker/README.md)：构建参数、运行约束和镜像扫描。

## 维护约定

修改行为时更新负责该主题的页面，避免多个文档重复维护同一份 API 示例。完成后的计划、临时审计和执行记录不留作使用手册；历史方案通过 Git 查询。AI 协作规则统一在 [AGENTS.md](../AGENTS.md)。
