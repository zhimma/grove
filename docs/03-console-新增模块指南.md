# Console 新增模块指南

本文档说明：如果你要在当前基础框架里给 `console` 新增一个业务模块，推荐的最小路径是什么。

示例场景：

- 新增一个“文章管理”
- 新增一个“标签管理”
- 新增一个“通知模板”

## 1. 先明确你要新增的是哪一层

通常一个 `console` 模块会包含 4 层内容：

1. 后端路由
2. handler
3. service
4. 前端页面与菜单

如果只是补一个操作接口，不一定需要新增页面。  
如果只是补一个页面，也不一定需要新增完整 CRUD。

## 2. 后端推荐步骤

### 2.1 新增 service

放在：

```text
app/console/service/
```

推荐模式：

- 定义 `XxxService`
- 定义 `Input / Output`
- service 只接收 `context.Context`

### 2.2 新增 handler

放在：

```text
app/console/handler/
```

handler 负责：

- 参数绑定
- 从 `reqctx` 读取身份
- 调用 service
- 输出 `response.Success / response.Fail`

### 2.3 注册路由

在 `app/console/internal/router/router.go` 中注册。

所有需要权限控制的路由，都应该：

1. 注册在受保护路由组
2. 使用 `route.Wrap(...)`
3. 尽量补上 `.Name(...)`

示例：

```go
articles := route.Wrap(protected.Group("/articles"))
articles.GET("", h.List).Name("内容管理.文章列表")
articles.POST("", h.Create).Name("内容管理.创建文章")
articles.PUT("/:id", h.Update).Name("内容管理.更新文章")
articles.DELETE("/:id", h.Delete).Name("内容管理.删除文章")
```

## 3. 如何接入权限系统

### 3.1 API 权限

不需要手工同步，也不需要写 catalog 表。

只要路由满足：

- 在 `protected` 路由组里
- 没有 `.Ignore()`

它就会自动进入运行时 API 权限清单。

### 3.2 展示名

推荐总是补 `.Name("模块.动作")`。

这样角色授权页里会看到清晰的中文文案，而不是 fallback 文案。

### 3.3 按钮权限

前端页面中，按钮显隐应该基于：

```ts
permissionStore.hasApiPermission('POST', '/console/v1/articles')
permissionStore.hasApiPermission('PUT', '/console/v1/articles/:id')
permissionStore.hasApiPermission('DELETE', '/console/v1/articles/:id')
```

不要再写旧的字符串权限，例如：

```ts
console.articles.post
```

## 4. 前端推荐步骤

### 4.1 新增页面

页面放在：

```text
web/admin-vben/apps/console/src/views/
```

### 4.2 新增路由

在：

```text
web/admin-vben/apps/console/src/router/routes/modules/
```

把新页面加进本地路由。

示例：

```ts
{
  name: 'ConsoleArticles',
  path: '/content/articles',
  component: () => import('#/views/console/content/articles.vue'),
  meta: { title: '文章管理' },
}
```

### 4.3 菜单权限

前端路由 `name` 就是菜单权限 key。

所以要注意：

- `name` 要稳定
- 改 `path` 问题不大
- 改 `name` 属于破坏性变更

## 5. 角色授权页会自动发生什么

### 接口权限部分

后端新接口会自动出现在：

```text
GET /console/v1/permissions/apis
```

角色页会自动看到它。

### 菜单权限部分

只要你把新页面加进本地路由树，角色页菜单树也会自动看到它。

不需要菜单同步。

## 6. 数据库与迁移

如果模块需要新表：

1. 创建 migration
2. 创建 model
3. 在 service 中使用

推荐命令：

```bash
go run ./cmd/artisan/main.go migrate create create_articles_table
go run ./cmd/artisan/main.go make:model Article
```

如果只是业务代码，不一定每次都需要用脚手架生成。

## 7. 测试建议

至少补两类验证：

### 后端

- 接口正常可用
- 未授权时返回拒绝
- 角色分配后可以访问

### 前端

- 菜单可见性正确
- 按钮显隐正确
- 角色页能看到新增 API 权限项

## 8. 一套最小新增清单

如果你只是新增一个标准 CRUD 模块，通常需要：

1. migration
2. model
3. service
4. handler
5. route registration
6. `route.Name(...)`
7. 前端 API 文件
8. 前端页面
9. 本地路由配置
10. 按钮权限判断

## 9. 常见错误

### 错误 1：接口加了，但没进角色页

通常原因：

- 路由注册在公开组
- 路由被 `.Ignore()`
- 服务没重启

### 错误 2：页面加了，但菜单里不显示

通常原因：

- 本地路由没有配置
- 当前角色没有勾选对应 `menu_key`
- 路由 `name` 改了，历史角色数据还保存旧 key

### 错误 3：按钮显隐不对

通常原因：

- 前端还在用旧字符串权限
- `hasApiPermission()` 的 method/path 写错

## 10. 当前建议

如果你给当前框架新增 `console` 模块，请优先遵守这三个约定：

1. API 权限只认 `METHOD + path`
2. 菜单权限只认前端路由 `name`
3. 接口展示文案统一来自 `route.Name(...)`
