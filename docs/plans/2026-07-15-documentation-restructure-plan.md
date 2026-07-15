# Grove 文档重构实施计划

## 状态

- 状态：Completed（文档重构已完成，后续只按维护规则增量更新）
- 日期：2026-07-15
- 适用对象：开发者、AI 编码助手和代码评审者

**目标：** 建立一套以项目总览、开发规范、运行手册和 AI 上下文为核心的单一文档体系，让开发者与 AI 都能从同一份事实源理解 Grove 并继续开发。

**文档架构：** 采用“入口文档 + canonical guides + 历史计划归档”的三层结构。README 只负责项目定位和最短路径；`docs/` 负责可长期维护的架构、开发、运行和组件指南；`docs/plans/` 保留设计决策与完成审计，不再作为日常入口。仓库根目录 `AGENTS.md` 提供给 AI 的短上下文，详细 AI 工作流放在 `docs/ai/`。

**技术栈：** Go 1.25.12、Gin、GORM、PostgreSQL、Redis、Casbin、Asynq、OpenTelemetry、Vue 3、Vite、pnpm 10.28.2、GitHub Actions。

---

### Task 1: 建立文档导航和事实源边界

**Files:**
- Modify: `README.md`
- Modify: `docs/README.md`
- Create: `docs/architecture.md`
- Create: `docs/commands.md`

**Step 1:** 从当前 Makefile、配置、服务入口和 CI 提取命令与运行事实。

**Step 2:** 编写项目总览、命令参考和文档导航，明确 canonical 文档与历史计划的边界。

**Step 3:** 检查所有链接、命令名称和端口是否与当前仓库一致。

### Task 2: 重写开发者主路径

**Files:**
- Modify: `docs/guide/quickstart.md`
- Modify: `docs/guide/configuration.md`
- Modify: `docs/guide/structure.md`
- Modify: `docs/01-开发规范.md`
- Modify: `docs/03-console-新增模块指南.md`

**Step 1:** 统一配置模型、启动流程、目录职责和新增模块流程。

**Step 2:** 增加“什么时候不要抽象”“如何验证”“常见失败”章节。

**Step 3:** 用真实路径、真实 Make target 和当前代码结构复核示例。

### Task 3: 建立运行与组件参考

**Files:**
- Modify: `docs/development/testing.md`
- Modify: `docs/deployment/deploy.md`
- Modify: `docs/guide/pkg-components.md`
- Modify: `docs/guide/permission.md`
- Modify: `docs/guide/routing.md`
- Create: `docs/operations.md`

**Step 1:** 统一测试、构建、部署、健康检查和可观测性说明。

**Step 2:** 以“用途、入口、边界、验证”格式整理基础组件和权限文档。

**Step 3:** 删除或改写与当前 Makefile、配置来源和端口不一致的内容。

### Task 4: 建立 AI 上下文与协作规范

**Files:**
- Create: `AGENTS.md`
- Create: `docs/ai/README.md`
- Create: `docs/ai/project-context.md`
- Create: `docs/ai/change-checklist.md`

**Step 1:** 写入短而稳定的仓库级事实、禁止事项、验证命令和目录边界。

**Step 2:** 写入 AI 读取顺序、任务分析模板、变更前后检查清单和完成证据要求。

**Step 3:** 确认 AI 文档不复制易漂移的业务细节，而是链接到 canonical guides。

### Task 5: 处理历史文档和一致性验证

**Files:**
- Create: `docs/plans/README.md`
- Modify: `docs/04-响应与错误处理规范.md`
- Modify: `docs/02-console-架构与权限.md`

**Step 1:** 给历史计划增加索引、状态和阅读说明。

**Step 2:** 校正 Console、响应错误和权限文档中的当前事实。

**Step 3:** 运行文档路径、Make target、端口、配置键和代码入口的一致性检查。

**Step 4:** 重新阅读全部新增/修改文档，执行 `git diff --check`。

## 完成记录

- 已建立 README、文档中心、架构、命令、运行手册和 AI 上下文入口。
- 已统一 `config.yaml` 单一后端配置模型，明确 `.env` 不由后端自动读取。
- 已修正 Console 实际目录、路由注册方式、API 权限 key、菜单真相源和 OpenAPI 入口。
- 已补充新增模块的迁移、模型、service、handler、路由、权限、OpenAPI、前端和测试闭环。
- 已补充响应协议中的 `400/413/422` 边界和前端错误解析入口。
- 已完成相对链接、旧命令、旧路径和配置残留扫描；最终验证以本次交付实际命令为准。

## 后续维护规则

1. 代码、测试、Makefile 和配置变更后，先更新对应 canonical guide。
2. 设计原因写入 `docs/plans/`，不要把历史方案复制回入门文档。
3. 新增服务、配置键、端口、路由或命令时，必须同步 README、命令参考、AI 上下文或相关领域指南。
4. 每次文档变更至少运行 `git diff --check`，涉及命令或路径时追加 `make help` 和对应验证命令。
