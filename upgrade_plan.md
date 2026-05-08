# Grove 框架修复升级计划

> 整理日期：2026-05-07
> 来源：Codex / Gemini / GLM / Kimi / MiniMax 五份审计报告交叉验证 + 源代码逐一核实

---

## 一、审计报告综述

五份报告对框架整体方向一致认可（架构清晰、分层合理、Provider 模式优秀），主要分歧在于优化深度。以下是各报告的价值评估：

| 报告 | 定位准确度 | 特点 |
|------|-----------|------|
| Codex | ⭐⭐⭐⭐⭐ | 每个问题都有精确代码位置和真实行为分析，质量最高 |
| Kimi | ⭐⭐⭐⭐ | 覆盖面广、代码示例详尽，部分建议偏理想化 |
| GLM | ⭐⭐⭐⭐ | 表格化清单完整，命名问题定位准确 |
| Gemini | ⭐⭐⭐ | 过于简洁，偏架构方向（DDD、DI框架），超出项目当前阶段 |
| MiniMax | ⭐⭐⭐ | 覆盖面广但深度不足，部分判断不够准确 |

---

## 二、分阶段修复计划

### 阶段一：P0 — 修复真实 Bug 和安全边界

以下问题经源代码验证确认为**行为 Bug**，在生产环境会产生错误行为。

#### 1. Redis `Add` 方法返回值错误

- **文件**：`pkg/cache/redis.go:290-308`
- **报告来源**：Codex ✅ / Kimi ✅ / GLM ✅
- **问题**：`SetNX` 的 `bool` 结果被 `.Err()` 丢弃，然后用 `Has()` 做二次查询。在并发场景下，key 已存在时 `SetNX` 不会设置，但 `Has` 仍返回 `true`，导致 `Add` 对已存在的 key 也返回 `true`——完全违背了 "仅在不存在时设置" 的语义。
- **修复方案**：直接读取 `SetNX().Result()` 的 bool 值，删除 `Has` 二次查询。

```go
// 修复前
setErr = r.client.SetNX(ctx, prefixedKey, data, secondsToTTL(seconds)).Err()
// ...
exists, err := r.Has(ctx, key)
return exists, nil

// 修复后
ok, err := r.client.SetNX(ctx, prefixedKey, data, ttl).Result()
return ok, err
```

---

#### 2. CORS 中间件多 Origin + MaxAge 硬编码 + Credentials 冲突

- **文件**：`internal/middleware/cors.go`
- **报告来源**：Codex ✅ / Kimi ✅ / GLM ✅ / MiniMax ✅
- **问题 A**：`Access-Control-Allow-Origin` 用 `strings.Join` 拼接多个 origin 为逗号分隔字符串。**HTTP 规范不允许该头部有多个值**（只能是单个 origin 或 `*`），浏览器会直接拒绝。
- **问题 B**：`Access-Control-Max-Age` 硬编码 `"600"`，但配置中有 `cfg.MaxAge` 字段未使用。
- **问题 C**：`AllowCredentials=true` 且 origin 为 `*` 时，浏览器也会拒绝。
- **修复方案**：
  - 改为按请求 `Origin` 头匹配白名单，命中后回写单个 Origin 值
  - 设置 `Vary: Origin` 头
  - `MaxAge` 使用 `strconv.Itoa(cfg.MaxAge)`
  - 当 `AllowCredentials=true` 时，禁止 `*` 通配

---

#### 3. HTTP Client 重试时 Body 复用失败

- **文件**：`pkg/httpclient/client.go:325-351`
- **报告来源**：Codex ✅
- **问题**：`doWithRetry` 复用同一个 `*http.Request`，body（`io.Reader`）在第一次请求后已被消费，后续重试发送空 body 或直接失败。
- **修复方案**：构建请求时保存 body 字节副本，在 `doWithRetry` 中每次重试前通过 `bytes.NewReader(savedBody)` 重建 `req.Body` 和 `req.GetBody`。

---

#### 4. `PostMultipart` form fields 写成 query param

- **文件**：`pkg/httpclient/client.go:578-592`
- **报告来源**：Codex ✅
- **问题**：`PostMultipart` 方法中对 `fields` 参数使用了 `builder.WithQueryParam(k, v)`，将表单字段写到了 URL 查询参数上，而不是 multipart form field。
- **修复方案**：改为 `builder.Body(fields)`，让 `doMultipart` 中已有的 `formData` 逻辑正确将其写为 multipart form field。

```go
// 修复前
for k, v := range fields {
    builder.WithQueryParam(k, v)
}

// 修复后
builder.Body(fields)
```

---

#### 5. LocalDriver 路径逃逸判断不严谨

- **文件**：`pkg/storage/local.go:133-149`
- **报告来源**：Codex ✅
- **问题**：用 `strings.HasPrefix(absPath, absRoot)` 判断路径安全。当 root 为 `/tmp/storage` 时，`/tmp/storage2/file` 也能通过检查，存在路径逃逸风险。
- **修复方案**：确保 `absRoot` 后加路径分隔符再比较：

```go
// 修复前
if !strings.HasPrefix(absPath, absRoot) {

// 修复后
if !strings.HasPrefix(absPath, absRoot+string(filepath.Separator)) && absPath != absRoot {
```

---

#### 6. Recovery 中间件缺少 stack trace

- **文件**：`internal/middleware/recovery.go`
- **报告来源**：Codex ✅ / Kimi ✅
- **问题**：panic 只记录了 recovered 值，没有输出 stack trace，生产排查困难。
- **修复方案**：增加 `runtime/debug.Stack()` 输出到日志。

---

#### 7. Redis `Remember` 用 `fmt.Printf` 绕过统一 logger

- **文件**：`pkg/cache/redis.go:270`
- **报告来源**：Codex ✅
- **问题**：缓存写入失败时使用 `fmt.Printf` 输出，绕过了统一的 `pkg/logger`，日志不可追踪。
- **修复方案**：改为 `logger.Warn().Err(err).Str("key", key).Msg("缓存 Remember 写入失败")`。

---

#### 8. 生产环境 JWT Secret 校验

- **文件**：`internal/config/load.go`
- **报告来源**：Codex ✅ / GLM ✅ / Kimi ✅ / MiniMax ✅
- **问题**：`JWT.Secret` 默认值为 `"change-me"`，生产环境启动时不会拒绝弱 secret。
- **修复方案**：在 `normalize()` 末尾增加生产环境校验，若 `App.Env == "production"` 且 `JWT.Secret` 为空或 `"change-me"`，返回明确错误。

---

### 阶段二：P1 — 修复一致性和安全问题

#### 9. Admin/Role Service 的 DB 与 Casbin 策略无事务保护

- **文件**：`app/console/service/admin.go` / `app/console/service/role.go`
- **报告来源**：Codex ✅ / Kimi ✅
- **涉及方法**：
  - `CreateAdmin`（L220-226）：先 DB Create，再 Casbin 绑定角色，Casbin 失败则管理员已入库但无权限
  - `UpdateAdmin`（L301-312）：先 DB 更新，再同步 Casbin，失败则角色字段与 Casbin 不一致
  - `DeleteAdmin`（L351-358）：先 DB 删除，再删 Casbin 策略，失败则策略残留
  - `DeleteRole`（L307-317）：先 DB 删除角色，再删策略和 grouping policy
  - `SetRolePermissions`（L353-370）：先删除旧策略，再添加新策略，添加失败则角色失去全部 API 权限
- **修复方案**：对 `AdminService` 和 `RoleService` 注入 `transaction.Manager`，将关键操作包装在事务中。由于 Casbin gorm adapter 与业务共用同一 DB，可以在同一事务内完成。

---

#### 10. Event Dispatcher `Close` 可能导致 `wg.Wait()` 挂死

- **文件**：`pkg/event/dispatcher.go:98-121`
- **报告来源**：Codex ✅
- **问题**：`Close()` 关闭 `stopCh` 后 worker 直接退出（走 `case <-d.stopCh: return`）。此时队列中已 `wg.Add(1)` 的 job 可能无人处理、无人 `Done()`，`wg.Wait()` 永远阻塞。
- **修复方案**：改为关闭 queue channel（`close(d.queue)`）而非 stopCh，让 worker 感知到 channel 关闭后 drain 完队列再退出。

---

#### 11. `AdminPermission` 中 enforcer 为 nil 时放行

- **文件**：`app/console/middleware/admin_auth.go:114`
- **报告来源**：Codex ✅
- **问题**：当 `enforcer == nil` 时直接跳过权限检查放行。开发期友好，但生产环境若 Casbin 配置漏开，等于后台无接口权限控制。
- **修复方案**：增加环境判断，生产环境 `enforcer == nil` 时返回 500 错误（快速失败），非生产环境保持放行。

---

#### 12. `ConsoleOptions` 缺少 `WithRedis`

- **文件**：`internal/provider/provider.go:54-62`
- **报告来源**：Codex ✅
- **问题**：Console 服务预设没有装配 Redis，但如果后续启用了 `WithCache` 等依赖 Redis 的组件，Redis 不可用。而 `APIOptions` 有 `WithRedis` 但 `ConsoleOptions` 没有。
- **修复方案**：在 `ConsoleOptions` 中加入 `WithRedis()`。

---

### 阶段三：P2 — 代码质量与规范统一

#### 13. 修复 `lebel` 拼写错误

- **涉及文件**：
  - `pkg/validation/request.go:305` — 兼容代码 `field.Tag.Get("lebel")`
  - `pkg/validation/request_test.go:64` — 测试中使用 `lebel` tag
  - `app/console/handler/role.go:18` — 生产代码使用 `lebel` tag
- **报告来源**：Codex ✅ / GLM ✅ / Kimi ✅ / MiniMax ✅
- **修复方案**：
  - `request.go` 中删除 `lebel` 兼容分支
  - `role.go` 中 `lebel` 改为 `label`
  - `request_test.go` 中的 `lebel` 测试用例改为 `label`

---

#### 14. Casbin `CheckConsolePermission` 与 `Can` 方法重复

- **文件**：`pkg/casbin/casbin.go:94-104`
- **报告来源**：GLM ✅
- **问题**：`Can` 和 `CheckConsolePermission` 实现完全相同（都是 `e.Enforce(subject, permission)`），功能重复。
- **修复方案**：在 `CheckConsolePermission` 上添加 `// Deprecated: Use Can instead.` 注释，内部调用 `e.Can`。后续逐步将调用方迁移到 `Can`。

---

#### 15. Service 依赖风格统一

- **涉及文件**：
  - `app/console/service/system_config.go` — 使用 `*gorm.DB`
  - `app/console/service/log.go` — 使用 `*gorm.DB`
  - `app/console/service/login_log.go` — 使用 `*provider.Provider`
  - `app/console/service/operation_log.go` — 使用 `*provider.Provider`
  - 对比：`admin.go` / `role.go` 使用 `database.Repo`
- **报告来源**：Codex ✅ / Kimi ✅
- **问题**：4 个 Service 的依赖注入风格不统一，混用 `*gorm.DB`、`*provider.Provider`、`database.Repo` 三种方式。
- **修复方案**：统一为 `database.Repo`，与 `AdminService` / `RoleService` 保持一致。

---

#### 16. Console 菜单目录下沉到 app/console

- **文件**：`pkg/permission/menu_catalog.go`
- **报告来源**：Codex ✅ / GLM ✅ / MiniMax ✅
- **问题**：`ConsoleMenuCatalog()` 包含 `ConsoleDashboard`、`ConsoleRoles` 等 console 专属业务语义，违反了 `pkg` "不承载具体业务语义" 的定位。
- **修复方案**：将 `ConsoleMenuCatalog()`、`FilterConsoleMenuKeys()`、`ValidateConsoleMenuKeys()`、`BuildConsoleMenuTree()` 移到 `app/console/internal/permission/` 或 `app/console/service/` 下。`pkg/permission` 只保留通用的 `BuildAPIIdentifier`、`StaticMenuCatalogItem`、`MenuTreeItem` 等类型和工具。

---

## 三、不纳入范围的建议

以下是各报告提出但经评估**不适合当前阶段**的建议：

| 建议 | 来源 | 不纳入原因 |
|------|------|-----------|
| 引入 wire/fx DI 框架 | Gemini | 项目规模不需要，Provider Option 模式已足够 |
| 引入 Repository 层 | Gemini / GLM / Kimi | 增加复杂度，当前 Service + Model 已够用 |
| 模块化单体拆分（按功能拆目录） | Gemini | 当前文件数量级不构成痛点 |
| 多数据库驱动支持（MySQL 等） | GLM / MiniMax | 项目明确定位 PostgreSQL |
| 事件持久化 | Kimi | 超出基础框架定位 |
| 分布式追踪 (OpenTelemetry) | GLM | 与当前阶段不匹配 |
| Prometheus 监控 | Kimi | 可后续按需引入 |
| 日志切割 (lumberjack) | Kimi / MiniMax | 生产通常用外部日志收集，不在框架层做 |
| Token blacklist 改 Redis | 全部 5 份 | 设计权衡：短期 access token 不可撤销 + refresh token 轮换是更合理方案 |
| `artisan` 拆分文件 | Codex / GLM / Kimi | 合理但优先级不高，属于 P3 |
| 多语言/国际化 | MiniMax | 项目定位中文后台，暂不需要 |
| `id.New()` panic 改返回 error | Kimi | `crypto/rand.Read` 几乎不可能失败，保留 panic 合理 |

---

## 四、预期工作量

| 阶段 | 修复项 | 预计耗时 | 风险 |
|------|--------|---------|------|
| P0 | #1-#8 | 1-2 天 | 低（独立组件修复，影响面小） |
| P1 | #9-#12 | 2-3 天 | 中（事务改造涉及多个 Service 方法） |
| P2 | #13-#16 | 1-2 天 | 低（规范统一，逻辑不变） |

---

## 五、验证方案

- 每个 P0 bug 修复后补对应单元测试
- 运行 `go build ./...` 确保编译通过
- 运行 `go test ./...` 确保不破坏现有测试
- CORS 修复后用浏览器开发者工具验证跨域行为
- Redis `Add` 修复后用并发测试验证原子性
- 事务改造后手动测试 "DB 成功 + Casbin 模拟失败" 场景的回滚行为
