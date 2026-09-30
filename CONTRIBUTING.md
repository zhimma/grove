# 参与 Grove

欢迎通过 [Issue](https://github.com/zhimma/grove/issues) 或 [Pull Request](https://github.com/zhimma/grove/pulls) 改进 Grove。先阅读 [README](README.md) 和[开发规范](docs/01-开发规范.md)。

## 提交问题

请提供使用的提交、Go/Node 版本、数据库类型、复现步骤、预期结果和实际结果。日志保留错误类型与请求 ID，并去掉密码、token、DSN 和业务数据。

涉及漏洞或凭据泄露时，不要在公开 Issue 中贴利用细节或真实密钥；先查看 GitHub 仓库 Security 页是否提供私密报告入口，或通过维护者公开提供的私密渠道联系。

## 修改代码

从 `main` 创建工作分支，一个 PR 聚焦一个问题。依赖升级与大范围重构请先说明实际需求和兼容影响。

- 使用 Go 的显式依赖、具体类型和真实变化点上的小接口。
- Handler 适配 HTTP，service 编排业务和事务，model 不处理权限或响应。
- 新 API 同步路由名称、OpenAPI、前端契约及权限使用方式。
- 测试覆盖行为与失败路径；修复问题时保留能复现原问题的回归测试。
- 接口、命令、配置或部署行为改变时，同一 PR 更新对应文档。
- 不提交本地 `config.yaml`、凭据、依赖缓存或构建产物。

## 提交前检查

从受影响范围开始，按变更风险选择：

| 变更 | 检查 |
| --- | --- |
| Go 逻辑 | 受影响包的 `go test`；共享能力另跑 `make test test.race quality.go.vet quality.go.lint` |
| 路由、鉴权、API | 上述测试加 `make contracts admin.contract` |
| 前端 | `make admin.typecheck admin.test admin.lint admin.circular admin.build` |
| 构建或依赖 | `make build quality.govuln`，以及受影响的镜像或集成测试 |
| 文档 | `make docs.check`、`git diff --check`，核对涉及的命令与代码 |

前端检查前执行 `make admin.install`。完整本地门禁是 `make ci`；真实 PostgreSQL/MySQL/Redis 检查见[测试指南](docs/development/testing.md)。没有实际运行或因缺少环境而跳过的测试，请在 PR 中写明。

PR 描述说明问题、修改后的行为、验证命令和剩余限制。提交标题可以使用 `fix:`、`feat:`、`refactor:`、`docs:`、`test:`、`chore:` 等前缀。

## 文档维护

文档只描述当前实现和使用方法。删除重复内容、失效命令和完成后的工作计划；需要追溯旧方案时使用 Git 历史。临时的共享计划可放在 `work/`，完成后把有效结论写入对应指南并删除计划。

不要把测试数量、临时评分或一次本地验证写成长期质量承诺。包和配置的版本以 `go.mod`、`.mise.toml`、前端 `package.json` 及 CI 为准。
