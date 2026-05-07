# Console 架构与权限

本文档说明当前基础框架中 `console` 的整体设计，重点覆盖：

- 请求如何完成认证与授权
- API 权限和菜单权限分别由谁负责
- 角色授权数据存在哪里
- 新增接口后，为什么不再需要同步

## 1. 设计目标

当前 `console` 采用的是“运行时自洽”的权限模型。

它主要解决三个问题：

1. token 一旦签发后，角色变更、禁用状态能够立即生效
2. 新增后端接口后，不需要手工同步权限清单
3. 前端菜单显示和后端 API 安全边界严格分离

这意味着：

- token 不承载授权快照
- API 权限清单来自后端运行时路由
- 菜单权限树来自前端本地路由

## 2. 当前模块边界

### 后端

- `app/console/handler`
  负责 HTTP Request / Response
- `app/console/service`
  负责业务逻辑
- `app/console/middleware`
  负责认证、权限、审计
- `app/console/internal/router`
  负责路由注册与中间件链路

### 前端

- `web/admin-vben/apps/console/src/router`
  负责本地路由与菜单过滤
- `web/admin-vben/apps/console/src/store/permission.ts`
  负责当前用户权限状态
- `web/admin-vben/apps/console/src/views/system/role`
  负责角色授权页面

## 3. 请求执行流程

当前 `console` 的请求链路如下：

1. 客户端携带 `Bearer access_token`
2. `AdminAuthn` 校验 token 有效性
3. `AdminAuthStateResolver` 按 `admin_id` 回库恢复当前授权态
4. 将当前身份写入 `reqctx.Identity`
5. `AdminPermission` 生成 `METHOD + path`
6. Casbin 根据 `admin -> role -> permission` 判断是否放行
7. 业务 handler / service 执行

这条链路的关键点是：

- token 只证明“你是谁”
- 当前是否可用、当前角色是谁、是否超管，必须回数据库看

## 4. Token 模型

当前后台使用 `access + refresh`：

- `access_token`
- `refresh_token`
- `expires_in`
- `token_type`

`access_token` 当前只把最小身份放进 claims：

- `admin_id`
- `user_type`

即使 claims 里还保留少量历史字段，也不应作为请求期最终授权依据。

## 5. API 权限模型

### 5.1 权限标识

API 权限统一使用：

```text
METHOD + 空格 + gin full path
```

例如：

```text
GET /console/v1/roles
POST /console/v1/roles
PUT /console/v1/roles/:id
```

### 5.2 目录来源

API 权限清单不是数据库真相源，而是运行时从已注册路由扫描得到：

1. 路由全部注册完成
2. 扫描 `engine.Routes()`
3. 只收集 `console` 受保护路由
4. 忽略 `.Ignore()` 路由
5. 生成树形权限选项

### 5.3 展示文案

接口展示名来自：

```go
route.Wrap(group).GET(...).Name("角色权限.角色列表")
```

`Name(...)` 只影响展示，不影响真正鉴权。

真正鉴权永远只认：

```text
METHOD + path
```

## 6. 菜单权限模型

### 6.1 菜单真相源

`console` 菜单真相源是前端本地路由，不在后端。

当前约定：

- 菜单授权 key = 前端路由 `name`
- 菜单展示 title = 前端路由 `meta.title`

### 6.2 后端职责

后端只负责代存储角色勾选结果：

- 存到 `console_roles.menu_keys`
- 读取时过滤历史脏 key
- 保存时校验 key 是否属于当前已注册菜单

后端不再负责：

- 菜单表维护
- 菜单同步

这是一种有意的取舍：

- 优点：没有双真相源，不需要同步
- 代价：菜单清单的真相源在前端路由，不在数据库

## 7. Casbin 数据职责

当前 `console` Casbin 只负责 API 权限：

- `p`：`role -> permission_identifier`
- `g`：`admin -> role`

也就是说：

- 菜单权限不进 Casbin
- 菜单权限只在角色表里保存

## 8. 角色授权页面的数据来源

角色授权页会同时消费两类数据：

### 接口权限树

来自：

```text
GET /console/v1/permissions/apis
```

特点：

- 节点展示名来自后端 `Name(...)`
- 节点 key 就是 `METHOD + path`

### 菜单权限树

来自前端本地路由：

- 不请求后端菜单表
- 由本地路由树直接转换

## 9. 为什么新增接口不需要同步

因为接口目录不是存表后再读取，而是运行时直接扫描路由。

所以新增一个接口，只要：

1. 路由被注册
2. 这个路由受保护
3. 最好补上 `Name(...)`

它就会自动出现在：

- `/console/v1/permissions/apis`
- 角色授权页接口树

这就是当前模型最重要的收益之一。

## 10. 当前 Console 基础业务

当前 `console` 已经覆盖这些基础业务：

- 登录 / 刷新 / 登出
- 当前管理员 / 更新资料 / 修改密码
- 工作台
- 管理员管理
- 角色管理
- 角色 API 权限
- 角色菜单权限
- 系统配置
- 文件上传
- 登录日志 / 操作日志

## 11. 当前模型的取舍

### 优点

- 授权变更立即生效
- 不再依赖权限同步
- 后端 API 安全边界明确
- 前后端职责更清晰

### 有意保留的简单性

- 菜单权限由前端负责识别和渲染
- 后端只做菜单 key 合法性校验，不做菜单表持久化
- 当前只聚焦 `console`

### 暂不解决的问题

- 多后台域统一抽象
- 更复杂的数据权限 DSL
- 插件化模块系统
- 前后端共享菜单 manifest

## 12. 推荐阅读

- [01-开发规范.md](./01-%E5%BC%80%E5%8F%91%E8%A7%84%E8%8C%83.md)
- [03-console-新增模块指南.md](./03-console-%E6%96%B0%E5%A2%9E%E6%A8%A1%E5%9D%97%E6%8C%87%E5%8D%97.md)
