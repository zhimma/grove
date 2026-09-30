# Console 新增模块指南

本文说明新增 Console 业务模块的标准流程，覆盖数据库、后端接口、权限、OpenAPI 和前端页面。

## 先判断模块范围

不要求每个模块都提供完整的增删改查（CRUD）。先明确本次变更属于哪一种：

- 只有后端接口：完成 service、handler、路由、权限和 OpenAPI。
- 只有后台页面：完成前端 API、页面、本地路由和菜单权限。
- 完整业务模块：按本文全部步骤执行。
- 仅内部任务：由 Worker 处理，不强行增加 Console 页面。

## 后端实施顺序

### 1. 数据库与模型

需要持久化时：

1. 使用 `make migrate.up` 之前先创建正反向 SQL 迁移。
2. 可用 CLI 创建迁移文件对（postgres 与 mysql 各一对，同一版本号）：

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
- 使用 `Input` 和 `Output` 表达复杂输入输出。
- 在 service 中编排事务、缓存、事件和队列。
- 预期业务错误使用 `pkg/errx`，底层错误用 `WithCause` 保留原因。

不要为了单一实现新增 Repository、ServiceContainer 或字符串依赖注入。

### 3. Handler

放在：

```text
app/console/internal/handler/
```

handler 只负责：

- 使用 `validation.BindJSON`、`validation.BindQuery` 或 `validation.BindURI` 绑定参数。
- 从请求上下文读取当前身份。
- 调用 service。
- 使用 `response.Success` 或 `response.Fail` 输出统一响应。

不要在 handler 中直接创建数据库、Redis、Job 或 Casbin 实例。

### 4. 路由与权限

现有模块都在对应 handler 文件中提供 `RegisterXxxRoutes`，由
`app/console/internal/router/router.go` 统一调用。新增模块应保持同样结构：

```go
func RegisterArticleRoutes(protected *gin.RouterGroup, dbs *database.Connections, pages pagination.Policy, catalog *route.Catalog) {
	h := &ArticleHandler{articleSvc: consoleservice.NewArticleService(dbs, pages)}

	articles := wrapRoute(protected.Group("/articles"), catalog)
	articles.GET("", h.List).Name("内容管理.文章列表")
	articles.GET("/:id", h.Detail).Name("内容管理.文章详情")
	articles.POST("", h.Create).Name("内容管理.创建文章")
	articles.PUT("/:id", h.Update).Name("内容管理.更新文章")
	articles.PUT("/:id/status", h.UpdateStatus).Name("内容管理.更新文章状态")
	articles.DELETE("/:id", h.Delete).Name("内容管理.删除文章")
}
```

然后在 `app/console/internal/router/router.go` 的 `// grove:register-routes` 标记附近注册：

```go
handler.RegisterArticleRoutes(protected, r.p.DB, pages, catalog)
```

`Provider` 只在路由或服务启动的装配边界出现。handler 和 service 只接收实际依赖；如果模块只使用一个数据库，也可以进一步把 `*database.Connections` 收窄为具体的 `*gorm.DB`。列表服务另接收路由装配层创建的同一份 `pages pagination.Policy`。

所有需要进入 API 权限目录的接口必须注册在 `protected` 组，且不能使用 `.Ignore()`。`route.Name("模块.动作")` 只影响角色授权页的展示文案，不改变实际权限标识。

API 权限标识固定为：

```text
METHOD + 空格 + gin full path
```

例如：`POST /console/v1/articles`。

## OpenAPI 契约

Console 的文档入口是：

- Scalar 页面：`GET /console/docs`
- OpenAPI JSON：`GET /console/docs/openapi.json`

新增或修改接口时同步更新：

1. `app/console/internal/docs/contract.go` 的请求与响应结构（schema）、接口操作（operation）。
2. 路由契约测试，确保 Gin 路由与 OpenAPI 没有缺失或多余。
3. 响应结构、状态码和字段错误说明。

不要只让接口能运行而不更新 OpenAPI；文档契约是接口变更的回归门槛。

## 前端实施顺序

### 1. API 文件

按领域放在：

```text
web/admin-vben/apps/console/src/api/
```

复用 `requestClient`，不要在页面中直接创建 Axios 实例。路径不手写：先在 `api/console-contract.json` 登记 operation，再用 `consoleEndpoint('consoleListArticles')` 取地址，契约测试会拒绝写死的 `/console/v1` 路径和未登记的 operationId。

### 2. 页面

页面放在 `web/admin-vben/apps/console/src/views/` 下按业务领域组织，例如 `views/content/articles/index.vue`；生成器放在 `views/<模块复数>/index.vue`。

列表页优先用 `components/resource-page`：列、搜索、表单和增删改接口都用配置声明。页面特有的部分用插槽补，不必整页手写：

- `#cell="{ column, record }"`：接管某些列的渲染（如状态标签）；未接管的列仍显示布尔值（是或否）或原值。
- `#actions="{ record }"`：在操作列追加按钮，按钮多时配合 `action-width`。
- 表单字段的 `help`、`placeholder`：字段说明与占位提示。
- 组件 ref 的 `reload()`：自定义操作完成后刷新列表。

参考 `views/system/scheduled-tasks/index.vue`。只有需要概览区、详情抽屉等明显不同的交互时才手写页面。

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
  component: () => import('#/views/content/articles/index.vue'),
  meta: { title: '文章管理' },
}
```

当前菜单授权真相源是前端本地路由树：

- `route.name` 是稳定的 `menu_key`，不要随意修改。
- `meta.title` 是展示文案。
- `path` 可以调整，但修改后要验证跳转、重定向和收藏页。
- 后端只保存角色的 `menu_keys`，不维护菜单表或菜单同步命令。


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
- 路由与 OpenAPI 契约。

前端至少覆盖：

- API 类型与错误解析。
- 菜单过滤和默认首页。
- 按钮权限显隐。
- 页面类型检查和生产构建。

推荐命令：

```bash
make test
make admin.typecheck
make admin.build
make verify
```

## 用生成器起步

新模块优先从生成器开始。生成模板有回归检查；具体模块仍须通过质量检查，并验证业务规则和运行行为。

### 生成代码

以下命令生成前后端基础代码：

```bash
go run ./cmd/grove make:module Invoice --label 发票 \
  --fields "title:string:required,amount:int,paid:bool,note:text,due_at:time"
```

| 生成物 | 位置 |
| --- | --- |
| 双方言迁移（同版本） | `database/migrations/{postgres,mysql}/*_create_invoices.*.sql` |
| 模型 | `internal/model/invoice.go` |
| 分页 CRUD service 与测试 | `app/console/internal/service/invoice.go`、`invoice_test.go` |
| 请求、响应与路由（含权限名 `发票.列表` 等） | `app/console/internal/handler/invoice.go`、`invoice_response.go` |
| OpenAPI 操作 | `app/console/internal/docs/invoice.go` |
| 路由与 OpenAPI 注册 | 写入 `router.go` 与 `contract.go` 中的 `grove:` 标记处 |
| 前端接口模块 | `web/admin-vben/apps/console/src/api/invoice.ts` |
| 基于 `resource-page` 的列表页 | `src/views/invoices/index.vue` |
| 菜单路由（顶级菜单 + 列表页） | `src/router/routes/modules/invoices.ts` |
| 前端契约登记 | 追加到 `src/api/console-contract.json` |

前端部分只在 `console-contract.json` 存在时生成；已删除后台前端的项目只生成后端代码。生成后会用 `web/admin-vben` 自带的 Prettier 格式化前端文件。如果尚未安装前端依赖，生成器会提示先安装，再到 `web/admin-vben` 运行 `pnpm format`。

菜单来自前端路由，角色授权页的菜单树直接列出新页面，无需后端登记。

字段写法 `name:type[:required]`：

| 类型 | Go | PostgreSQL | MySQL |
| --- | --- | --- | --- |
| `string` | `string` | `VARCHAR(255)` | `VARCHAR(255)` |
| `text` | `string` | `TEXT` | `TEXT` |
| `int` | `int` | `INTEGER` | `INT` |
| `bool` | `bool` | `BOOLEAN` | `TINYINT(1)` |
| `time` | `*time.Time` | `TIMESTAMPTZ NULL` | `DATETIME(6) NULL` |

- `time` 在请求与响应里都是 `2006-01-02 15:04:05` 格式的字符串（请求也接受 RFC3339 和 `2006-01-02`）；更新时传 `""` 清空非必填时间，格式不对返回 422。
- `required` 只用于 `string`、`text`、`time`。`int`、`bool` 的零值本身合法，validator 的 `required` 会误拒 0 和 `false`，需要约束请在 service 里写。
- 省略 `--fields` 时默认 `name:string:required`；省略 `--label` 时显示名同模块名。
- 字段来自命令行而不是读库，所以生成不需要数据库连接，两种方言的产物完全对称。

生成后通常还要做：

1. `make migrate.up` 建表。
2. 按业务补充校验（唯一性、状态流转等）。
3. 把页面里的列名、表单 `label` 换成中文，按需换菜单图标与排序（默认 `lucide:folder`、`order: 1000`）。

生成器回归测试会在仓库副本中生成模块，再运行后端 `go vet`、生成的 CRUD 测试、Console 路由与 OpenAPI 及前端契约比对，并检查方言迁移文件规则。前端接口模块也会检查是否只调用已登记的 operation。

这些测试不执行 Vue 类型检查、生产构建、真实数据库迁移或浏览器操作。生成后仍须按本指南完成相应检查，测试入口与范围见[测试策略](development/testing.md#生成器回归的范围)。

### 命名与自定义表单

`HTTPClient` 生成 `http_client.go`、`http_clients` 和 `http-clients`；字段 `user_id` 和 `api_url` 分别生成为 `UserID` 和 `APIURL`。`InvoiceTest`、`InvoiceLinux` 等名称会生成 Go 特殊后缀，命令会在写文件前拒绝。字段不能与 `Base`、`TableName`、模块更新请求的 ID 字段或其他转换后的字段重名。

API 统一引用 `types/pagination.ts` 的 `PageParams`、`PageData<T>`，不从某个业务 API 文件借用分页类型。自定义编辑页直接导入组件并传入 `:form-component="ConfigForm"`；组件暴露 `getFormStateData()` 返回提交数据。页面可通过 `transformPayload` 调整载荷，`fixedParams` 固定查询条件。示例见 `views/configs/`。

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

- HTTP 方法没有转为大写，或路径不是完整的 `/console/v1/...`。
- 使用了旧字符串权限，而不是 `hasApiPermission`。
- 只做了前端隐藏，没有同步后端受保护路由。

## 完成定义

一个可交付的 Console 模块至少应有：

1. 迁移和模型（如需要）。
2. service、handler、受保护路由和 `route.Name`。
3. OpenAPI 数据结构、接口操作和契约测试。
4. 前端 API、页面、本地路由和菜单标识。
5. 按钮权限和错误回填。
6. 后端、前端和关键权限路径验证记录。
