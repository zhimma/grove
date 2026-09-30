# 基于 Grove 建立新项目

Grove 通过派生（fork）或克隆（clone）整个仓库复用。复制后由你的项目自行维护，不依赖上游发布，也不提供自动升级通道。本文说明复制代码后的必要修改。

先按[快速上手](./quickstart.md)启动项目，确认环境可用后，再执行下面的步骤。

## 1. 更换 Go 模块路径

这是唯一一处影响全仓编译的改动。

```bash
OLD=github.com/zhimma/grove
NEW=github.com/your-org/your-app

grep -rl "$OLD" --include='*.go' . | xargs sed -i '' "s|$OLD|$NEW|g"   # macOS
# Linux 用 sed -i "s|$OLD|$NEW|g"

sed -i '' "s|^module $OLD|module $NEW|" go.mod
go build ./... && go test ./...
```

`grove make:module` 从 `go.mod` 读取模块路径，无需手动修改生成器。

前端 `web/admin-vben/**/package.json` 中的 `repository` 和 `homepage` 字段指向 Grove 仓库，不影响构建，按需替换。

## 2. 决定示例代码去留

| 归类 | 内容 | 建议 |
| --- | --- | --- |
| **框架能力** | 管理员、角色权限、会话、操作与登录日志、系统配置、计划任务、文件上传 | 保留 |
| **框架能力** | `users` C 端用户管理（`internal/model/user.go` + console 用户页） | 保留。`console_admins` 是运营者，`users` 是你的终端客户 |
| **示例** | `articles` 文章管理 | 通常删除，见下 |
| **示例** | `app/api` 的 starter 演示接口 | 通常删除 |
| **示例** | `app/worker` 的 echo 任务 | 通常删除 |

### 删除 articles

```
internal/model/article.go
app/console/internal/handler/article.go
app/console/internal/handler/article_response.go
app/console/internal/service/article.go
app/console/internal/service/article_test.go
database/migrations/postgres/202604150013_create_articles.*
database/migrations/mysql/202604150013_create_articles.*
web/admin-vben/apps/console/src/views/content/articles/index.vue
```

同时移除 `app/console/internal/router/router.go` 中的 `handler.RegisterArticleRoutes(...)`、前端 `router/routes/modules/content.ts` 的“内容管理”路由，以及 OpenAPI 与前端契约中的对应条目。运行 `make contracts` 检查是否有遗漏。

### 删除 api starter 与 echo 任务

```
app/api/internal/handler/starter.go
app/api/internal/service/starter.go
app/api/internal/router/demo.go
app/worker/internal/handler/echo.go
internal/jobtask/echo.go              # TaskEcho / EchoPayload
```

`app/api/internal/router/router.go` 里去掉 `r.installDemoRoutes(...)`。

### 没有 C 端用户的项目

若你的系统只有内部运营、不存在终端客户，删 `users` 时要一并处理两处依赖：

- `app/console/internal/service/dashboard.go` 的 `UserCount`
- 前端 `views/dashboard/overview/index.vue` 对应的统计卡片

## 3. 必改配置

`cp config.example.yaml config.yaml` 后，开发环境至少改这几项：

| 配置项 | 说明 |
| --- | --- |
| `databases.default` | 包含 `driver`、`host`、`port`、`user`、`password` 和 `dbname`。`host`、`user`、`dbname` 不能为空。默认值对应 `make deps.up` 启动的本地 PostgreSQL，使用该实例时无需修改 |
| `jwt.secret` | 模板留空；不填无法签发令牌。用 `go run ./cmd/grove key:generate` 生成 |
| `security.initial_root_password` | 留空则 `make seed.bootstrap` 生成并打印一次性随机密码，请妥善保存 |
| `security.config_encryption_key` | 使用系统配置的加密字段时必填，同样由 `key:generate` 生成。上线后不要更换，否则已加密的配置无法解密 |

注意 `job.enabled: true` **强制要求** `redis.enabled: true`（`internal/config/validate.go` 的 `Config.Validate`）。本机没有 Redis 就把两个一起关掉。

生产环境另有以下强制要求，由 `Config.Validate` 的 `production` 分支检查；不满足时启动失败：

- `app.debug` 必须为 `false`
- `jwt.secret` 至少 32 个字符且不是 `change-me`
- `security.initial_root_password` 若填写，必须符合强度要求
- `cors.allowed_origins` 不能含 `*`
- Console 要求默认数据库启用
- Console 要求 `console` Casbin 权限执行器启用

## 4. 改项目标识

| 位置 | 内容 |
| --- | --- |
| `config.yaml`、`config.example.yaml` | `app.name` |
| `README.md`、`AGENTS.md` | 项目定位描述 |
| `docs/` | 按需裁剪；保留与你的项目实际能力匹配的指南 |
| `web/admin-vben/apps/console/.env.development`、`.env.production` | 前端标题与 API 地址 |
| `.gitlab-ci.yml`、`.github/workflows/ci.yml` | 按使用的 CI 平台保留，删除不用的配置 |

## 5. 上游修复

派生项目与 Grove 独立演进。跟进上游改动时，需手动选择并合入所需修复：

```bash
git remote add upstream https://github.com/zhimma/grove.git
git fetch upstream
git log upstream/main --oneline
git cherry-pick <commit>
```

修改 Go 模块路径或业务结构后，上游变更可能冲突；逐项审查需要的修复，并在合入后运行相应测试。不要直接覆盖下游代码。

## 6. 验证

改造完成后跑一遍完整门禁：

```bash
make verify        # Go 测试 + 构建 + 前端类型检查
make quality       # 格式、静态检查、前端 lint 与循环依赖
make contracts     # 路由与 OpenAPI、前端契约双向校验
```

删除示例后，运行 `make contracts` 检查路由、OpenAPI 和前端契约是否有遗漏；任一处不一致都会使检查失败。
