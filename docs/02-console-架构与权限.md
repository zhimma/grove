# Console 认证与权限

本文说明管理后台的身份、Session、API 权限与菜单授权。实现入口是 `app/console/internal/middleware/admin_auth.go`、`service/session.go`、`service/auth_state.go` 与 `pkg/rbac`；新增功能见[模块指南](03-console-新增模块指南.md)。

## 请求链路

1. 从 `Authorization: Bearer <token>` 提取访问令牌。
2. 校验签名算法、issuer、audience、有效期和 Console 身份字段。
3. 用 `admin_id`、`session_id` 验证数据库中的会话状态。
4. 回库恢复管理员状态、当前角色和超管标志。
5. 对受保护接口，以 `METHOD + Gin full path` 检查权限。
6. Handler 绑定参数并调用 service。

token 不是授权快照。管理员禁用、角色状态和会话吊销以数据库当前状态为准；Casbin 策略在各实例内存中维护，跨实例存在重载延迟。缺少 Session 服务时拒绝访问；普通管理员缺少权限执行器时返回 503。超管在通过身份和 Session 校验后按超管规则放行，生产配置仍要求启用 Console 执行器。

## Token 与 Session

后台使用 access/refresh token：access claims 包含 `admin_id`、`session_id`、`user_type` 等身份字段；数据库 `console_sessions` 只保存 refresh token 哈希。

刷新会轮换令牌，退出、修改密码及强制下线会吊销对应会话。请求期必须经过 Session 校验，不能只检查 JWT 是否过期。

API 与 Console 使用不同的 issuer 后缀和 audience，令牌不可混用。浏览器以 `sessionStorage` 保存 token，刷新页面可恢复；这些值仍可被页面脚本读取，前端存储不是 XSS 防护边界。

## API 权限与菜单权限

| 数据 | 标识与来源 | 保存位置 |
| --- | --- | --- |
| API 权限目录 | 已注册的受保护路由，如 `PUT /console/v1/roles/:id` | 实例级 route Catalog，由运行时构建 |
| API 授权 | Casbin `p: role → permission`、`g: admin → role` | `console_casbin_rules` |
| 菜单目录 | 前端本地路由树 | `src/router/routes/modules/` |
| 菜单授权 | 路由 `name`，如 `ConsoleRoles` | `console_roles.menu_keys` |

没有需要人工同步的菜单表或权限目录表。`.Name("角色权限.角色列表")` 提供展示文案，不改变权限 key；`.Ignore()` 只排除目录登记，不替代身份校验。

```go
roles := route.Wrap(protected.Group("/roles"), catalog)
roles.GET("", h.List).Name("角色权限.角色列表")
roles.POST("", h.Create).Name("角色权限.创建角色")
```

前端菜单显隐使用路由 name，按钮使用对应的 API 权限：

```ts
permissionStore.hasApiPermission('POST', '/console/v1/roles')
```

隐藏菜单或按钮不是授权措施，后端仍需独立鉴权。物理文件移动可以保持路由 name 不变；更名需要考虑数据库中已经保存的菜单 key。

## 配置与多实例

```yaml
casbin:
  enforcers:
    console:
      enabled: true
      database: default
      mode: rbac
      table_name: console_casbin_rules
      auto_load_seconds: 30
```

每个实例启动时加载策略，再按 `auto_load_seconds` 重载；设为 `0` 会关闭自动重载。多实例变更不会即时传播，数据库连接或重载失败也可能延长延迟，部署时需监控。生产 Console 必须启用默认数据库和权限执行器。

API 可以启用独立的 `api` enforcer，但不共用 Console 的菜单模型。

## 修改授权与一致性

- 授予的 API 权限必须出现在运行时目录中。
- 菜单 key 由后端校验数量、长度和格式；后端不复制整棵前端路由树。
- 业务表和 Casbin adapter 的写入不能假定属于同一个数据库事务。当前 service 对管理员/角色写入进行补偿，策略集合通过 adapter 原子替换。
- 如果主操作及补偿都失败，先运行 `go run ./cmd/grove rbac check` 排查，再审查 `rbac repair --dry-run` 的结果；修复命令不自动授权新增接口。

排查权限问题时同时核对管理员、角色、Session、路由 key 和实例策略，不要通过移除中间件绕过错误。

## 相关入口

- [模块开发](03-console-新增模块指南.md)
- [统一响应与错误](04-响应与错误处理规范.md)
- [配置](guide/configuration.md)
- [发布验收](deployment/staging-checklist.md)
