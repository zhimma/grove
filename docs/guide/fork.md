# 基于 Grove 建立新项目

Grove 以 fork / clone 方式复用：整仓拿走后由你的项目自持，不依赖上游发版，也不提供自动升级通道。本文说明拿到代码后要改什么。

先按[快速上手](./quickstart.md)把它跑起来，确认环境没问题，再做下面的改造。

## 1. 改 module path

这是唯一一处影响全仓编译的改动。

```bash
OLD=github.com/zhimma/grove
NEW=github.com/your-org/your-app

grep -rl "$OLD" --include='*.go' . | xargs sed -i '' "s|$OLD|$NEW|g"   # macOS
# Linux 用 sed -i "s|$OLD|$NEW|g"

sed -i '' "s|^module $OLD|module $NEW|" go.mod
go build ./... && go test ./...
```

涉及约 126 个 Go 文件。代码生成器不需要手改——`grove make:module` 从 `go.mod` 读 module path，改完就跟着变。

前端 `web/admin-vben/**/package.json` 里的 `repository` / `homepage` 字段指向 Grove 仓库，不影响构建，按需替换。

## 2. 决定示例代码去留

| 归类 | 内容 | 建议 |
| --- | --- | --- |
| **框架能力** | 管理员、角色权限、会话、操作/登录日志、系统配置、计划任务、文件上传 | 保留 |
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
web/admin-vben/apps/console/src/views/console/content/articles.vue
```

同时移除 `app/console/internal/router/router.go` 里的 `handler.RegisterArticleRoutes(...)`、前端 `router/routes/modules/console.ts` 的「内容管理」路由，以及 OpenAPI 与前端契约里的对应条目（`make contracts` 会告诉你漏了哪些）。

### 删除 api starter 与 echo 任务

```
app/api/internal/handler/starter.go
app/api/internal/service/starter.go
app/api/internal/router/demo.go
app/worker/internal/handler/default_job.go
pkg/job/tasks.go              # TaskEcho / EchoPayload
```

`app/api/internal/router/router.go` 里去掉 `r.installDemoRoutes(...)`。

### 没有 C 端用户的项目

若你的系统只有内部运营、不存在终端客户，删 `users` 时要一并处理两处依赖：

- `app/console/internal/service/dashboard.go` 的 `UserCount`
- 前端 `views/console/dashboard/overview.vue` 对应的统计卡片

## 3. 必改配置

`cp config.example.yaml config.yaml` 后，开发环境至少改这几项：

| 配置项 | 说明 |
| --- | --- |
| `databases.default` | driver / host / port / user / password / dbname。host、user、dbname 不能为空 |
| `jwt.secret` | 模板留空；不填无法签发 token。用 `go run ./cmd/grove key:generate` 生成 |
| `security.initial_root_password` | 留空则 `make seed.bootstrap` 生成一次性随机密码并打印，注意从输出里抄走 |
| `security.config_encryption_key` | 使用系统配置的加密字段时必填，同样由 `key:generate` 生成。上线后不要更换，否则已加密的配置无法解密 |

注意 `job.enabled: true` **强制要求** `redis.enabled: true`（`internal/config/load.go` 的 `Config.Validate`）。本机没有 Redis 就把两个一起关掉。

生产环境额外强制（不满足直接启动失败，均在 `Config.Validate` 的 production 分支）：

- `app.debug` 必须为 `false`
- `jwt.secret` 至少 32 个字符且不是 `change-me`
- `security.initial_root_password` 若填写，必须符合强度要求
- `cors.allowed_origins` 不能含 `*`
- console 要求默认数据库启用
- console 要求 console casbin enforcer 启用

## 4. 改项目标识

| 位置 | 内容 |
| --- | --- |
| `config.yaml` / `config.example.yaml` | `app.name` |
| `README.md`、`AGENTS.md` | 项目定位描述 |
| `docs/` | 按需裁剪；`docs/plans/` 是 Grove 自己的决策归档，通常整个删掉 |
| `web/admin-vben/apps/console/index.html` | 标题 |
| `.gitlab-ci.yml` / `.github/workflows/ci.yml` | 二选一，删掉不用的那份 |

## 5. 上游修复

fork 后两边独立演进，Grove 不提供升级通道。要跟进上游改动只能手工：

```bash
git remote add upstream <grove-repo>
git fetch upstream
git log upstream/main --oneline
git cherry-pick <commit>
```

改过 module path 之后几乎每个 cherry-pick 都会冲突，属预期。建议只在有明确安全修复时才做，日常不同步。

## 6. 验证

改造完成后跑一遍完整门禁：

```bash
make verify        # Go 测试 + 构建 + 前端类型检查
make quality       # 格式、静态检查、前端 lint 与循环依赖
make contracts     # 路由与 OpenAPI、前端契约双向校验
```

`make contracts` 是删示例后最有用的一项：路由、OpenAPI 和前端契约任何一处没删干净都会红。
