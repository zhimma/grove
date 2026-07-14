# Task 23：Console 自定义代码测试基线设计

## Status

Accepted

## Context

Console 已有菜单过滤、权限 Store、token refresh、logout 和角色授权等自定义逻辑，但当前只有菜单工具的两个测试。全仓 Vitest 主要覆盖 Vben 上游包，不能证明 Grove 自定义认证和权限边界正确。

现场复核发现：权限 Store 在授权数据加载完成前返回允许访问；logout 的关键状态清理由 `resetAllStores()` 隐式完成；角色页的叶子权限收集函数位于 SFC 内部；Console 请求客户端工厂未导出，无法验证并发 401 的 refresh 组装行为。

## Decision

### 1. 权限加载默认 fail closed

`isLoaded=false` 时 API 和菜单权限均返回 false。授权总览成功加载后按列表和 `*` 判断；加载失败时使用空权限并标记已加载，避免失败后回到 fail open。

测试直接使用真实 Pinia Store 和模块级 API mock，不挂载 Vue 组件。

### 2. 菜单工具保持纯函数

扩展 `menu-access.test.ts`，覆盖：

- `*` 保留全部路由。
- 空或未知 key 返回空路由。
- 子路由授权保留必要父节点。
- 隐藏父节点只提升可见子节点到权限目录。
- 首页路径优先选择首个可访问子路由，并支持 redirect/path fallback。

不修改路由数据源，不新增菜单服务层。

### 3. Console 请求客户端验证并发 refresh

导出 `createRequestClient` 供同目录测试使用。使用 Axios MockAdapter 对该客户端真实 instance 模拟两个并发 401，在 refresh Promise 完成前确保第二个请求进入等待队列；断言只调用一次 refresh，两个请求均使用新 token 重放。

只测试 Console 的 interceptor 组装和 Store token 更新，不重复测试 Axios 或 Vben RequestClient 内部实现。

### 4. logout 关键状态使用纯函数显式清理

新增 `clearConsoleAuthState`，显式清空 access token、refresh token、access codes、动态菜单、动态路由、权限已检查状态、登录过期状态和 Grove Permission Store。`logout` 继续先调用 `resetAllStores()` 清理其他 Store，再调用该函数锁定认证安全边界。

纯函数使用最小 `Pick<Store>` 类型，不引入 class、容器或通用状态管理抽象。

### 5. 角色授权 helper 从 SFC 提取

把 `collectLeafPermissionKeys` 移到同目录 `permission-helpers.ts`，测试只提交目录叶子权限、忽略父节点和未知 key，并保持目录顺序。

## Verification

- 先运行新增 Console 测试文件，确认 fail-open 用例在实现前失败。
- `pnpm vitest run --dom apps/console/src`。
- `make admin.typecheck`。
- `pnpm test:unit`。
- `make admin.build`。
- `git diff --check`。

不执行 `pnpm install`，使用仓库现有依赖。
