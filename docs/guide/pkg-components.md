# 基础组件索引

`pkg/` 是 Grove 仓库内跨服务共用的技术层，不对外发布独立库。业务对象接收所需的具体依赖；`internal/provider` 仅在启动与装配边界使用。

| 能力 | 代码入口 | 用法与边界 |
| --- | --- | --- |
| 令牌与 Bearer 认证 | `pkg/auth` | [认证与会话](../02-console-架构与权限.md) |
| API 权限与路由元数据 | `pkg/rbac`、`pkg/permission`、`pkg/route` | [权限目录与菜单](../02-console-架构与权限.md) |
| 请求上下文、验证、响应与错误 | `pkg/request`、`pkg/validation`、`pkg/response`、`pkg/errx` | [HTTP 协议](../04-响应与错误处理规范.md) |
| 数据库、事务与迁移 | `pkg/database`、`pkg/transaction`、`pkg/migrate` | [数据库指南](database.md) |
| 日志 | `pkg/logger` | [运行日志](../operations.md#日志与观测)、[轮转配置](configuration.md#log) |
| 缓存 | `pkg/cache` | [内存与 Redis 缓存、JSON 辅助函数](cache.md) |
| 事件 | `pkg/event` | [进程内同步与异步分发](event.md) |
| 队列与调度 | `pkg/job`、`pkg/scheduler` | [持久队列](queue.md)、[计划任务](scheduler.md) |
| HTTP 客户端 | `pkg/httpclient` | [超时、重试与流式处理](httpclient.md) |
| 文件 | `pkg/storage` | 见下文；不以公开 URL 替代下载授权 |
| 分页 | `pkg/pagination` | 见下文；列表协议共用 |
| 密码与配置加密 | `pkg/password`、`pkg/secretbox` | 见下文及[敏感配置](configuration.md#业务配置敏感值) |
| 登录保护 | `pkg/ratelimit` | 登录频率与失败锁定，不是所有 API 的配额组件 |
| ID | `pkg/ulid` | `ulid.New()` 生成 26 个字符的 ULID，不用于生成密钥或访问凭据 |

## 文件上传

在装配层获取 `*storage.Manager` 并注入 service。服务端上传入口：

```go
file, err := storageManager.SaveUploadedFile(ctx, "local", "avatar", header)
if err != nil {
    return nil, err
}
```

第三个参数是上传策略名称，应与 `storage.upload_policies` 中的键一致，例如模板的 `avatar`。

策略限制文件大小、扩展名和探测后的 MIME；请求总大小由 `server.max_body_bytes` 限制。本地文件通过临时写入后提交，路径需经过校验。

默认文件私有。本地磁盘只有同时开启 `public` 和 `serve_static` 才由应用提供匿名静态访问；私有文件走受保护的 Console 下载接口。S3 公开访问由存储桶（bucket）策略控制。普通后台上传使用服务端模式；使用安全令牌服务（STS）的临时凭据进行直传时，需要额外配置存储权限。

## 分页

Service 接收 `pagination.Policy`，输入可以嵌入 `pagination.Request`：

```go
page := pages.Resolve(input.Request)
query = page.Apply(query)
meta := pagination.NewMeta(total, page)
```

`Policy` 零值使用每页 20 条、上限 100 条；Console 装配时使用 `api.default_per_page` 和 `api.max_per_page`。`list_all=true` 不加 `LIMIT`，应仅用于有明确数据量边界的操作。不要在 handler 和 service 再各定义一套分页结果。

## 密码与配置加密

账号密码统一使用 `password.Hash`、`password.Verify`。登录时账号不存在的分支使用 `password.VerifyMiss`，避免与真实密码比较产生明显耗时差异。业务代码不要重复指定 bcrypt 的计算成本参数。

`secretbox` 使用独立应用密钥对业务配置做 AES-GCM 加密；它不是密码哈希组件，也不复用 JWT 密钥。模型不隐式解密，业务通过配置 service 的 `ResolveEffectiveValue` 读取有效值。
