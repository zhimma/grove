# Console 新增模块指南

本文档是新增 Console 业务模块的 canonical 流程。目标是让一个模块从数据库、后端接口、权限、OpenAPI 到前端页面形成完整闭环。

## 先判断模块范围

不要求每个模块都做完整 CRUD。先明确本次变更属于哪一种：

- 只有后端接口：完成 service、handler、路由、权限和 OpenAPI。
- 只有后台页面：完成前端 API、页面、本地路由和菜单权限。
- 完整业务模块：按本文全部步骤执行。
- 仅内部任务：放入 worker/job，不强行增加 Console 页面。

## 后端实施顺序

### 1. 数据库与模型

需要持久化时：

1. 使用 `make migrate.up` 之前先创建正反向 SQL 迁移。
2. 可用 CLI 创建迁移文件对：

   ```bash
   go run ./cmd/grove migrate create create_articles_table
   ```

3. 共享 GORM 模型放在 `internal/model`。
4. 迁移和模型先补测试，再进入 service。

模型只负责字段、表名和通用查询辅助，不负责 HTTP、权限或业务流程。

### 2. Service

放在：

```text
app/console/internal/service/
```

建议：

- 一个明确的 `XxxService`，只接收实际依赖。
- 方法接收 `context.Context`。
- 使用 `Input / Output` 表达复杂输入输出。
- 在 service 中编排事务、缓存、事件和队列。
- 预期业务错误使用 `pkg/errx`，底层错误用 `WithCause` 保留原因。

不要为了单一实现新增 Repository、ServiceContainer 或字符串依赖注入。

### 3. Handler

放在：

```text
app/console/internal/handler/
```

handler 只负责：

- 使用 `validation.BindJSON/BindQuery/BindURI` 绑定参数。
- 从 request context 读取当前身份。
- 调用 service。
- 使用 `response.Success/Fail` 输出统一响应。

不要在 handler 中直接创建数据库、Redis、Job 或 Casbin 实例。

### 4. 路由与权限

现有模块都在对应 handler 文件中提供 `RegisterXxxRoutes`，由
`app/console/internal/router/router.go` 统一调用。新增模块应保持同样结构：

```go
func RegisterArticleRoutes(protected *gin.RouterGroup, dbs database.Connections, catalog *route.Catalog) {
	h := &ArticleHandler{
		articleSvc: service.NewArticleService(dbs),
	}

	articles := route.Wrap(protected.Group("/articles"), catalog)
	articles.GET("", h.List).Name("内容管理.文章列表")
	articles.POST("", h.Create).Name("内容管理.创建文章")
	articles.PUT("/:id", h.Update).Name("内容管理.更新文章")
	articles.PUT("/:id/status", h.UpdateStatus).Name("内容管理.更新文章状态")
	articles.DELETE("/:id", h.Delete).Name("内容管理.删除文章")
}
```

然后在 `app/console/internal/router/router.go` 的 `// grove:register-routes` 标记附近注册：

```go
handler.RegisterArticleRoutes(protected, r.p.DB, r.p.RouteCatalog)
```

`Provider` 只在 router/server 装配边界出现。handler 和 service 只接收实际依赖；如果模块只使用一个数据库，也可以进一步把 `database.Connections` 收窄为具体的 `*gorm.DB`。

所有需要进入 API 权限目录的接口必须注册在 `protected` 组，且不能使用 `.Ignore()`。`route.Name("模块.动作")` 只影响角色授权页的展示文案，不改变实际权限 key。

API 权限 key 固定为：

```text
METHOD + 空格 + gin full path
```

例如：`POST /console/v1/articles`。

## OpenAPI 契约

Console 的文档入口是：

- Scalar 页面：`GET /console/docs`
- OpenAPI JSON：`GET /console/docs/openapi.json`

新增或修改接口时同步更新：

1. `app/console/internal/docs/contract.go` 的请求、响应 schema 和 operation。
2. 路由 contract test，确保 Gin 路由与 OpenAPI 没有缺失或多余。
3. 响应结构、状态码和字段错误说明。

不要只让接口能运行而不更新 OpenAPI；文档契约是接口变更的回归门槛。

## 前端实施顺序

### 1. API 文件

按领域放在：

```text
web/admin-vben/apps/console/src/api/
```

复用 `requestClient`，不要在页面中直接创建 Axios 实例。请求路径使用后端完整 Console 前缀，例如 `/console/v1/articles`。

### 2. 页面

页面放在：

```text
web/admin-vben/apps/console/src/views/console/
```

目录按业务领域组织，例如 `views/console/content/articles.vue`。

### 3. 本地路由和菜单

在以下目录新增或修改路由：

```text
web/admin-vben/apps/console/src/router/routes/modules/
```

示例：

```ts
{
  name: 'ConsoleArticles',
  path: '/content/articles',
  component: () => import('#/views/console/content/articles.vue'),
  meta: { title: '文章管理' },
}
```

当前菜单授权真相源是前端本地路由树：

- `route.name` 是稳定的 `menu_key`，不要随意修改。
- `meta.title` 是展示文案。
- `path` 可以调整，但修改后要验证跳转、重定向和收藏页。
- 后端只保存角色的 `menu_keys`，不维护菜单表或菜单同步命令。

仓库里部分旧页面仍存在 `meta.permissions`，它不是后端 `menu_keys` 的真相源；新模块优先遵循路由 `name` + API 权限模型。

### 4. 按钮权限

按钮显隐使用 API 权限：

```ts
permissionStore.hasApiPermission('POST', '/console/v1/articles')
permissionStore.hasApiPermission('PUT', '/console/v1/articles/:id')
permissionStore.hasApiPermission('DELETE', '/console/v1/articles/:id')
```

不要新增 `console.articles.post` 这类业务字符串权限。

## 测试与验证清单

后端至少覆盖：

- 正常请求和响应字段。
- JSON、Query、URI 参数错误。
- 未登录 `401` 和无权限 `403`。
- 角色授权后可访问，未授权时被拒绝。
- 资源不存在、唯一冲突和数据库异常。
- 路由与 OpenAPI contract。

前端至少覆盖：

- API 类型与错误解析。
- 菜单过滤和默认首页。
- 按钮权限显隐。
- 页面 typecheck 和 production build。

推荐命令：

```bash
make test
make admin.typecheck
make admin.build
make verify
```

## CLI 生成器边界

可用以下命令减少模板代码：

```bash
go run ./cmd/grove make:model Article
go run ./cmd/grove make:service Article
go run ./cmd/grove make:handler Article
go run ./cmd/grove make:module Article
```

`make:module` 会生成 `internal/model`、Console service、Console handler，并在路由标记处注册后端路由；它不会生成迁移、前端页面、菜单或权限数据。生成后必须人工补齐业务逻辑、OpenAPI、测试和前端。

## 常见错误

### 接口没有出现在角色授权页

- 注册到了公开或普通路由组。
- 使用了 `.Ignore()`。
- 没有重启 Console，运行时路由目录尚未刷新。
- 没有更新角色页面实际请求的权限接口。

### 页面没有出现在菜单

- 本地路由没有加入 `routes/modules`。
- `route.name` 与角色已有 `menu_keys` 不一致。
- 页面被 `filterConsoleRoutesByMenuKeys` 过滤。

### 按钮显隐不正确

- method 没有转为大写或 path 不是完整 `/console/v1/...`。
- 使用了旧字符串权限，而不是 `hasApiPermission`。
- 只做了前端隐藏，没有同步后端受保护路由。

## 完成定义

一个可交付的 Console 模块至少应有：

1. 迁移和模型（如需要）。
2. service、handler、受保护路由和 `route.Name`。
3. OpenAPI schema/operation 和 contract test。
4. 前端 API、页面、本地路由和菜单 key。
5. 按钮权限和错误回填。
6. 后端、前端和关键权限路径验证记录。
