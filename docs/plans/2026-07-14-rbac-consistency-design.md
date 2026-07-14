# Task 12：GORM 与 Casbin 一致性边界设计

## Status

Accepted

## Context

Console 管理员与角色数据存储在业务表，角色绑定和接口权限存储在 Casbin 表。当前 service 把两类写入包在 GORM 事务回调中，但 Casbin adapter 不使用该事务连接，因此事务只能回滚业务表，不能回滚已经持久化的 Casbin 变化。

已确认的失败模式：

- 管理员换角色先删除旧 grouping，再添加新 grouping；添加失败会丢失旧绑定。
- 角色权限先删除旧 policy，再批量添加新 policy；添加失败会丢失旧权限。
- service 的 `WithTransaction` 让代码看起来像单事务，实际没有跨越 Casbin adapter 边界。

## Decision

### 1. 明确真相源

- `console_admins.role_id` 是管理员角色归属的真相源。
- `console_roles` 是有效角色集合的真相源。
- Casbin `p` policy 是接口权限集合的唯一存储，不在业务表复制一份权限快照。
- Casbin `g` grouping 是 `console_admins.role_id` 的派生索引，可通过检查命令重建。

### 2. 不伪装成跨存储事务

- 从 `AdminService`、`RoleService` 删除 `WithTransaction` 和围绕 Casbin 写入的 GORM 事务包装。
- 单纯业务表写入仍使用数据库自身事务。
- 跨边界流程采用“提交一个边界 → 同步另一个边界 → 失败补偿”的显式步骤。

### 3. Casbin 集合使用原子替换

在 `pkg/rbac.Enforcer` 增加两个语义明确的方法：

- `ReplaceConsolePoliciesForRole(roleID, permissions)`
- `ReplaceConsoleRoleForUser(userID, roleID)`

两者使用 adapter 的 `UpdateFilteredPolicies`。GORM adapter 会在自己的数据库事务中读取旧集合、删除旧集合并写入新集合；写入失败时旧集合保留。Casbin 内存模型只在 adapter 成功后更新。

### 4. 管理员写入补偿

- 创建：先提交管理员记录，再写 grouping；grouping 失败时删除刚创建的管理员。
- 换角色：先原子清空旧 grouping，再提交 `role_id`，最后写入新 grouping。同步窗口内管理员最多暂时无权限，不保留旧高权限；失败时恢复旧 `role_id` 和 grouping。其他资料在角色同步成功后再提交，避免同步失败却部分修改资料。
- 删除：先原子清空 grouping，再删除管理员；数据库删除失败时恢复旧 grouping。
- 补偿也失败时返回包含主错误和补偿错误的错误链，由 `grove rbac check` 暴露残留差异。

### 5. 角色写入补偿

- 权限更新直接使用原子 policy 替换，不再使用数据库事务包装 `Remove + Add`。
- 删除角色前保存旧 policy，先清空 policy，再删除角色；数据库删除失败时恢复旧 policy。
- 删除角色前仍要求没有管理员引用；因此不需要批量迁移 grouping。

### 6. 一致性命令

新增：

```text
grove rbac check
grove rbac repair --dry-run
```

检查内容：

- 每个有效管理员是否只有一个与 `role_id` 一致的 grouping。
- 是否存在无对应管理员或角色不一致的 grouping。
- policy 和 grouping 引用的角色是否仍有效。

`repair` 默认 dry-run，只输出差异；显式 `--dry-run=false` 才执行：

- 按 `console_admins.role_id` 重建管理员 grouping。
- 删除无对应管理员的 grouping。
- 删除引用无效角色的 policy。

## Consequences

### Positive

- Casbin 旧集合不会因批量写入失败而丢失。
- 代码不再暗示不存在的跨存储 ACID 保证。
- 数据库和 Casbin 的残留差异可检测、可安全修复。

### Negative

- 数据库和 Casbin 之间仍存在短暂同步窗口。
- 极端情况下主操作和补偿都失败，需要运行检查/修复命令。

## Alternatives Considered

- 将业务表和 Casbin 强行放入同一个 GORM transaction：拒绝。当前 adapter 使用独立连接，继续包装只会制造错误保证。
- 在业务表复制完整权限集合并以 outbox 异步同步：当前需求不需要，引入额外真相源和后台任务，违反 YAGNI。
- 继续 `Remove + Add` 并在失败后手工恢复：拒绝。adapter 已提供原子 filtered replace，应直接使用。

## Verification

- SQLite trigger 注入 Casbin 新 policy/grouping 写入失败，确认旧集合仍存在。
- service failure-path 测试确认管理员创建、换角色、删除和角色删除的补偿行为。
- 本机 PostgreSQL 临时库验证 `check`、默认 dry-run 和实际 repair 生命周期。
