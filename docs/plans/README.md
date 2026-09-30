# 升级清单与历史背景

本目录包含用户要求持续维护的升级清单，以及此前的设计、审查和实施讨论。它不是当前架构或发布状态的事实来源。

## 如何阅读

- 日常开发先读 `docs/README.md`、`docs/architecture.md` 和对应 guide。
- 需要理解“为什么这样做”时，再读对应 design 文档。
- 需要确认任务是否完成时，先看[当前状态](../status.md)，再查[升级清单](2026-08-29-grove-framework-upgrade-plan.md)中的范围和验收记录；旧 completion audit 不能证明今天的工作已全部完成。
- 旧计划可以保留历史背景，但如果与当前代码冲突，以当前代码、测试和 canonical guide 为准。

## 当前维护入口

- [框架升级计划](2026-08-29-grove-framework-upgrade-plan.md)

保留原路径与任务 ID 便于逐项勾选；完成、部分完成、暂缓与待验收分别标识，不再把历史问题表当作当前待办。

## 历史背景

下列文档描述各自创建时的方案或验证，不应据此选择当前 Go 版本、目录结构或判断本次 CI 结果。

- [Foundation roadmap](2026-07-13-grove-foundation-roadmap.md)
- [Completion audit](2026-07-15-completion-audit.md)
- [Documentation restructure plan](2026-07-15-documentation-restructure-plan.md)
- [Database dialect layering plan](2026-07-15-database-dialect-layering-plan.md)
- [PostgreSQL/MySQL support implementation](2026-07-15-mysql-support-implementation.md)

## 设计文档索引

- Provider 生命周期与服务配置
- Console Session 与 refresh token
- 登录保护与可信代理
- 上传安全
- 系统配置敏感值
- RBAC 一致性
- Cache、HTTP Client、Scheduler、Event 契约
- OpenAPI 合同与路由漂移
- Readiness、OpenTelemetry 与安全 CI
- Console 前端测试基线

文件名中的日期表示记录时间，不代表当前代码一定仍按原始草案实现。
