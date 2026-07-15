# 设计计划与历史记录

`docs/plans/` 记录 Grove 的架构决策、实施计划、验证结果和完成审计。

## 如何阅读

- 日常开发先读 `docs/README.md`、`docs/architecture.md` 和对应 guide。
- 需要理解“为什么这样做”时，再读对应 design 文档。
- 需要确认任务是否完成时，查看路线图和 completion audit。
- 旧计划可以保留历史背景，但如果与当前代码冲突，以当前代码、测试和 canonical guide 为准。

## 当前主线

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
