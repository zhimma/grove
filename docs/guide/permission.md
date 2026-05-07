# 权限控制

本文档说明当前 `console` 管理后台的权限模型，以及框架里 Casbin 的实际使用方式。

当前仓库的权限设计只对 `console` 这条链路做了定型：

- 菜单权限 key：前端路由 `name`
- API 权限 key：`METHOD + path`
- 后端只存角色授权结果，不维护人工权限节点主数据

## 核心约定

### 1. Console API 权限

`console` 的接口权限统一使用：

```text
METHOD + 空格 + gin full path
```

例如：

```text
GET /console/v1/roles
POST /console/v1/roles
PUT /console/v1/roles/:id
```

后端鉴权时只认这一类 key，不再使用 `console.roles.list` 之类的业务字符串。

### 2. Console 菜单权限

`console` 的菜单权限统一使用前端路由 `name`。

例如前端路由：

```ts
{
  name: 'ConsoleRoles',
  path: '/system/role',
  meta: { title: '角色权限' },
}
```

其中：

- `ConsoleRoles` 是菜单权限 key
- `meta.title` 是菜单展示文案

`path` 可以调整，`name` 应视为稳定标识。

### 3. 数据库存储职责

后端数据库只保存授权结果：

- `console_roles.menu_keys`
  - 保存角色被授予的菜单权限 key 列表
- `console_casbin_rules`
  - 保存角色被授予的 API 权限 key

后端不再维护：

- 菜单表
- 权限清单表
- 菜单授权中间表

这些历史表如果出现在旧迁移里，只代表迁移过程，不代表当前运行时模型。

## Casbin 配置

当前 `console` 使用单独的 Casbin 表：

```yaml
casbin:
  enforcers:
    console:
      enabled: true
      database: default
      mode: rbac
      table_name: console_casbin_rules
```

运行时检查示例：

```go
enforcer := p.GetEnforcer("console")
allowed, err := enforcer.CheckConsolePermission(
    adminID,
    "GET /console/v1/roles",
)
```

## 权限清单来源

### API 权限清单

角色授权页里看到的 API 权限项，不来自数据库目录表。

它来自运行时路由扫描：

1. `console` 路由全部注册完成
2. 扫描 `engine.Routes()`
3. 只收集受保护的 `console` 路由
4. 忽略 `.Ignore()` 标记的接口
5. 生成接口权限选项

接口展示名优先来自：

```go
route.Wrap(group).GET(...).Name("角色权限.角色列表")
```

`Name(...)` 只影响展示，不影响实际鉴权。

### 菜单权限树

角色授权页里的菜单树，不来自后端数据库。

它来自前端本地路由树。后端只负责：

- 校验 `menu_keys` 是否属于当前已注册菜单 key
- 保存角色勾选结果
- 查询时返回角色已有的 `menu_keys`

## 使用方式

### 路由注册

所有需要进入角色授权页 API 权限清单的接口，都应该：

1. 注册到 `protected` 路由组
2. 不要标记 `.Ignore()`
3. 尽量补上 `.Name("模块.动作")`

示例：

```go
roles := route.Wrap(protected.Group("/roles"))
roles.GET("", h.List).Name("角色权限.角色列表")
roles.POST("", h.Create).Name("角色权限.创建角色")
roles.PUT("/:id", h.Update).Name("角色权限.更新角色")
```

### 前端按钮权限

前端按钮显隐应该按 API 权限 key 判断：

```ts
permissionStore.hasApiPermission('POST', '/console/v1/roles')
permissionStore.hasApiPermission('PUT', '/console/v1/roles/:id')
permissionStore.hasApiPermission('DELETE', '/console/v1/roles/:id')
```

不要再写旧式字符串权限。

### 菜单授权

角色菜单权限应直接传路由 `name` 列表：

```json
{
  "menu_keys": ["ConsoleDashboard", "ConsoleSystem", "ConsoleRoles"]
}
```

## 错误约束

角色分配权限时，后端会做两类校验：

- API 权限必须存在于运行时路由目录
- 菜单权限必须是已注册的菜单 key

否则返回 `422 Unprocessable Entity`。

## 设计边界

当前仓库只把这套模型定在 `console` 管理后台：

- 不要求对外业务 `app/api` 也使用同一套菜单/权限清单模型
- 不重新引入菜单同步命令
- 不重新引入权限 catalog 持久化表

如果后续扩展到其他后台域，应复用这套原则，而不是恢复人工维护权限节点。
