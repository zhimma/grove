# Console 登录保护与可信代理设计

## 信任边界

服务启动时始终调用 Gin `SetTrustedProxies`。空列表表示不信任任何代理；仅配置中的 CIDR 或地址可以影响 `ClientIP()`。production 拒绝 `*`、`0.0.0.0/0` 和 `::/0` 等全网信任配置。登录保护只使用 RequestMeta 中已解析的可信客户端 IP。

`X-Request-Id` 只接受 1–120 个 ASCII 字母、数字、点、下划线、冒号和短横线；其它输入重新生成 UUID，避免日志注入和超长索引值。

## 登录限流与失败锁定

登录 key 由小写、去空格后的账号与规范化 IP 组合后做 SHA-256，Redis 和日志不暴露原始账号。请求速率与失败锁定是两个独立状态：

- 单机请求速率使用 `golang.org/x/time/rate` token bucket。
- Redis 启用时使用 Lua 原子递增和 TTL，所有实例共享窗口。
- 连续凭据失败达到阈值后写短期锁；不存在的账号使用相同逻辑，避免账号枚举。
- 成功登录清理失败计数和锁，不重置请求速率。
- 限流或锁定返回 429、稳定错误码和 `retry_after`。

Redis 不可用时返回服务错误，不静默退回本机状态造成多实例语义分裂。

## 安全响应头与 CORS

全局响应默认增加 `X-Content-Type-Options: nosniff`、`X-Frame-Options: DENY`、`Referrer-Policy: strict-origin-when-cross-origin` 和限制敏感能力的 `Permissions-Policy`。HSTS 只有 `security.hsts_enabled=true` 时启用。

production 配置禁止 CORS `*`，无论是否允许凭据。该校验在启动前失败，避免服务以不安全默认值运行。

## 验证

- 本机 limiter 并发、窗口和失败锁定 race 测试。
- 登录失败达到阈值、成功重置、账号/IP key 隔离测试。
- 可信代理、非法 request ID、安全响应头和 production CORS 配置测试。
- Redis 可用时验证两个 guard 实例共享限流与锁定状态。
