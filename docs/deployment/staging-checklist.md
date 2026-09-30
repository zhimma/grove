# Grove Staging Smoke 清单

> 状态：这是 staging 验收流程，不是本地验证记录。每次发布都要以当前提交 SHA、镜像 digest、配置版本和执行时间重新填写结果；未执行项必须标记“未验证”，不能默认通过。

## 0. 记录与前置条件

在变更单或发布记录中保存下表，避免把构建成功误写成基础设施已经验证。

| 项目 | 必须记录的证据 | 通过条件 |
| --- | --- | --- |
| 代码与制品 | Git 提交 SHA、三个服务镜像 digest、`bin/grove` 构建来源 | 与待发布变更一致 |
| 配置与密钥 | secret manager/version、挂载路径、权限检查结果（不记录值） | secret 不在镜像、命令行或日志中 |
| 数据库 | PostgreSQL 或 MySQL 的隔离环境、迁移前后 `status` 输出 | 没有 dirty 或意外的待执行迁移 |
| Redis/队列 | Redis endpoint、worker 实例、任务 ID 与消费日志 | 任务成功消费且无重复异常 |
| HTTP 与浏览器 | readiness、受控 Console 登录与关键页面操作记录 | 所有预期状态码和界面行为正确 |

开始前确认：

- staging 数据库、Redis、bucket 和 Console 域名与生产隔离；不要用生产数据、生产密钥或生产 root 账号做 smoke。
- 只使用 CI 已构建且经过发布审批的镜像 digest；当前仓库的 GitLab pipeline 验证构建但不自动发布镜像。
- `config.yaml` 由受控 secret manager 或部署系统生成，挂载后仅应用 UID 可读；不把文件上传到 CI artifacts。
- API、Console、Worker 都使用同一份经过审查的配置版本，但端口和 service name 与部署目标匹配。
- 若使用 local storage，提前创建独立 staging volume；若使用对象存储，使用独立 staging bucket/prefix。

## 1. 数据库迁移矩阵

每种数据库使用独立环境和对应的 `databases.default.driver` 配置。先检查，再迁移，再检查；共享 staging 环境绝不执行 `migrate down`。

```bash
bin/grove --config /opt/grove/config.yaml migrate status
bin/grove --config /opt/grove/config.yaml migrate up
bin/grove --config /opt/grove/config.yaml migrate status
```

分别完成下列矩阵：

| 环境 | 必做检查 | 通过条件 |
| --- | --- | --- |
| PostgreSQL staging | 启动数据库、执行上述 status/up/status、启动 API/Console/Worker | migration 状态无 dirty，三个服务 readiness 成功 |
| MySQL staging | 使用 MySQL 配置重复 status/up/status、启动三个服务 | migration 状态无 dirty，约束和服务启动正常 |
| Redis staging | 启动 Redis、API/Worker 指向该 Redis | Redis 和 queue readiness 都成功 |

首次部署才执行 `seed bootstrap`，且只在 root 初始密码已由 secret 注入时执行；重复 bootstrap 不得覆盖现有 root 密码。若 migration 失败，停止推广并保留 `migrate status`、应用日志和数据库错误。应用回滚与数据库回退是不同操作：先回滚兼容的应用制品，数据迁移的回退需要单独审批。

## 2. 服务启动、readiness 与优雅退出

1. 先运行一次受控 migration job，再按 `api`、`console`、`worker` 的顺序启动镜像。容器应保留 `--read-only`、非 root、`no-new-privileges`、`cap-drop ALL` 与明确的 tmpfs/volume 挂载。
2. 从服务所在网络检查（示例端口按默认配置调整）：

   ```bash
   curl -fsS http://127.0.0.1:8080/health/live
   curl -fsS http://127.0.0.1:8080/health/ready
   curl -fsS http://127.0.0.1:8081/health/ready
   curl -fsS http://127.0.0.1:8082/health/ready
   ```

   `/health/live` 只证明进程存活；只有 `/health/ready` 同时证明启用的数据库、Redis 和 queue 后端可用。
3. 对每个服务发送一次 `SIGTERM`，确认在编排系统宽限期内停止、没有 panic、没有未关闭资源错误，并能用原 digest 再次启动并恢复 readiness。宽限期必须大于 `server.shutdown_timeout`。
4. 仅从监控网络检查 `/metrics`；确认公网和普通 Console 网络不能直接访问 metrics。

## 3. Redis 与队列 smoke

1. 记录 Redis `PING` 成功和 worker `/health/ready` 结果；`job.enabled: true` 时 readiness 必须包含 queue 后端检查。
2. 当前脚手架只在非 production 且 `demo.enabled: true` 时注册示例投递接口。可在隔离 staging 使用一个受控 API token 调用 `POST /api/v1/jobs/echo`，请求体为 `{"message":"staging-smoke"}`。
3. 保存返回的 `task_id`，并在 worker 结构化日志中确认同一任务的 `task_type=echo` 和“echo 任务已处理”。不得在生产环境为了 smoke 打开 demo 路由。
4. 若当前 staging 不允许 demo 路由，应记录为“队列端到端未验证”，不要用 Redis ping 替代消费成功。

## 4. 文件与静态路由策略

1. 使用 Console 的受控上传流程上传一个无敏感内容的测试文件，记录 disk、对象路径和 checksum，不记录访问 token。
2. 默认 private disk：直接访问其 `/storage/...` 路径必须失败；使用有权限的 `GET /console/v1/storage/download?disk=<disk>&path=<path>` 必须成功并返回正确文件。
3. 只有产品明确需要公开文件时，单独在 staging 设置 `public: true` 且 `serve_static: true`，再验证静态 URL 可访问。任一开关缺失时不得注册静态路由。
4. 对象存储场景还要检查 bucket/prefix 属于 staging、签名或下载授权的过期行为，以及私有对象不经 CDN/反向代理意外公开。

## 5. 浏览器与权限 smoke

1. 通过 TLS 入口打开 Console，用 staging 管理员登录；检查 token 过期/刷新、菜单和接口权限不串到 API surface。
2. 用最小权限角色访问一个允许路由和一个拒绝路由；拒绝必须是 `403`，而不是因为 enforcer 缺失而放行或返回成功。
3. 上传测试文件后，在日志页面确认审计条目存在且 query/detail 中没有明文 token、密码或签名。
4. 检查 API/Console OpenAPI 页面可打开，浏览器控制台无阻断级错误。
5. 在计划任务页修改一个已注册任务的调度参数、启停并请求执行一次，确认后续 Worker 对账生效、请求标记被处理、上次结果可见。可用隔离数据验证内置会话清理任务；不要用生产会话做清理验收。
6. 从计划任务、日志、会话等关键页面逐项确认列表、筛选、编辑或专用操作结果与 API 一致。对新生成的真实业务模块，还需记录补充业务规则与浏览器 CRUD 的验证结果，不能仅凭生成器回归测试勾选。

## 6. 扫描、监控与结论

- 在推广前运行 [镜像扫描](../../docker/README.md#镜像扫描)；当前 CI 不自动执行镜像漏洞扫描，需要在发布环境保存实际扫描结果。
- 检查错误率、数据库/Redis 指标、worker 失败和重试指标；`/metrics` 不应泄露 SQL、密码或 token。
- 若任何一项失败，停止推广，保留无 secret 的日志片段、请求 ID、task ID、镜像 digest 和 migration status。不要通过关闭 readiness、跳过权限或删除 volume 来“让验收通过”。
- 只有每一项都有本次外部环境证据时，发布记录才能标记为“staging 已验证”；否则只能标记为“本地/CI 已验证，staging 待验收”。
