# Console Session 与 Refresh Token 轮换设计

## 目标与边界

Console access token 保持短期 JWT，refresh token 改为 256-bit 随机不透明值。数据库是 session 唯一真相源，Redis 后续只能作为缓存，不能决定 session 是否有效。该设计不引入服务容器、通用 Repository 或额外认证框架。

API 演示 token 不进入 Console session 体系。Console 登录、刷新、退出、修改密码、管理员重置密码和后台强制下线统一操作 `console_sessions`。

## 数据模型

`console_sessions` 保存：session ID、管理员 ID、refresh token SHA-256、设备名称、客户端 IP、User-Agent、最后活跃时间、过期时间、撤销时间、撤销原因和创建更新时间。表不使用软删除；session 生命周期由 `revoked_at` 和 `expires_at` 明确表达。

refresh token 原文只在登录或刷新响应中出现一次。access JWT 增加 `session_id` claim。受保护请求除验证 JWT 和管理员状态外，还必须确认对应 session 未撤销且未过期，因此服务重启、多实例和退出登录后不会恢复已撤销 access token。

## 关键流程

- 登录：校验账号密码后创建 session，保存 refresh hash，再签发携带 session ID 的 access token。
- 刷新：按旧 hash 查询有效 session，生成新 token；使用 `WHERE refresh_token_hash = old_hash AND revoked_at IS NULL` 条件更新抢占旧 token。并发请求只有一个更新成功，其他请求返回 `invalid_refresh_token`。
- 退出：根据当前 access token 的 session ID 撤销 session，不依赖进程内 blacklist。
- 修改或重置密码：密码更新与撤销该管理员全部 session 在同一数据库事务中完成。
- 强制下线：后台会话页可分页查看 session，并撤销指定 session。

## API 与前端

新增受权限保护的 `GET /console/v1/sessions` 和 `DELETE /console/v1/sessions/:id`。列表返回管理员、设备、IP、最后活跃、过期、撤销状态和当前 session 标记。前端在系统管理增加“在线会话”页面，支持筛选和强制下线。

登录请求可选传入 `device_name`；缺失时使用截断后的 User-Agent。refresh 与 logout 保持现有响应结构，避免前端 token 存储协议漂移。改密成功后前端清理本地 token 并返回登录页。

## 验证

- 单元测试验证 token 随机性、hash、session claim、旧 refresh 重放、logout、改密撤销和 session 列表。
- 并发刷新在真实 PostgreSQL 上验证只有一个成功。
- 迁移执行 up/down，确认外键、唯一索引和撤销字段。
- 运行 Console auth/router race 测试、全量 Go 测试、vet、build；前端依赖可用时运行 typecheck/build。
