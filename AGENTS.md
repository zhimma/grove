# Grove Repository Instructions

本文件是仓库级开发与 AI 协作约定。项目事实以当前代码、配置、Makefile 和 CI 为准；历史计划只用于理解决策背景，不能替代现场代码检查。

## 项目定位

Grove 是一个 console-first 的中大型 Go 单体脚手架，借鉴 Laravel 的开发体验，但采用 Go 的显式组合、直接命名和按需抽象。

- 后端入口：`app/api`、`app/console`、`app/worker`
- CLI：`cmd/grove`
- 共享内部装配：`internal/`
- 可复用基础组件：`pkg/`
- 数据库迁移与种子：`database/`
- 管理后台：`web/admin-vben/apps/console`
- 文档入口：[docs/README.md](docs/README.md)

## 代码风格

- 使用 Go 风格命名和显式依赖，不引入 Java 式 ServiceContainer、BaseService、通用 Repository 或运行时字符串解析。
- `handler` 只负责 HTTP 绑定、身份读取、调用 service 和响应输出。
- `service` 负责业务流程、事务和副作用，方法接收 `context.Context`。
- `model` 只放共享模型和查询辅助，不放 HTTP 或权限流程。
- `internal/provider` 只出现在启动和装配边界；业务对象接收精确依赖。
- 只有存在多个实现或真实变化点时才使用接口、Factory、Strategy、State 等模式。
- 运行时日志使用 `pkg/logger`，不要在业务代码中直接使用标准库 `log`。
- 认证、权限、上传、配置 secret、迁移等关键流程修改前，先检查完整调用链和副作用。

## 配置与运行

- 本地唯一配置文件是 `config.yaml`，模板是 `config.example.yaml`。
- 后端不自动读取 `.env`；部署平台可以用进程环境变量覆盖 `${VAR:default}` 占位符。
- 本地 PostgreSQL 默认按实际配置设置 `DB_ENABLED=true`、`DB_HOST`、`DB_USER`、`DB_PASSWORD` 和 `DB_NAME`。
- Go 使用仓库要求的 1.25.12；本机如存在旧 wrapper，先设置：

  ```bash
  export PATH="/Users/zhimma/.local/share/mise/installs/go/1.25.12/bin:$PATH"
  export GOTOOLCHAIN=local
  ```

## 常用命令

先运行 `make help`。主要入口：

- `make run.api`：API，默认 `:8080`
- `make run.console`：Console，默认 `:8081`
- `make run.worker`：Worker，默认 `:8082`
- `make migrate.up`、`make seed.bootstrap`
- `make test`、`make build`、`make verify`
- `make admin.dev`、`make admin.typecheck`、`make admin.build`

## 验证要求

修改后按风险逐级验证：

1. 先跑受影响包的测试。
2. 跨模块或关键流程修改跑 `make test`、`go test -race ./...`、`go vet ./...`。
3. 构建相关修改跑 `make build`；前端修改跑 `make admin.typecheck` 和 `make admin.build`。
4. 路由、权限、OpenAPI 修改必须跑对应 contract tests。
5. 文档命令、路径、配置键必须与当前代码和 Makefile 对照检查。

完成声明必须说明实际运行的命令、通过结果和未验证的外部条件。不要用历史日志替代当前验证。

## AI 工作顺序

1. 先读本文件和 [docs/ai/project-context.md](docs/ai/project-context.md)。
2. 再读 [docs/architecture.md](docs/architecture.md)、[docs/commands.md](docs/commands.md) 和对应领域指南。
3. 使用 `rg` 搜索真实符号、路由、配置键和测试；不要只根据旧计划推断。
4. 先判断是诊断、文档更新还是代码实现，再选择最小改动范围。
5. 交付前遵循 [docs/ai/change-checklist.md](docs/ai/change-checklist.md)。
