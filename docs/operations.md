# Grove 运行与运维

## 启动前检查

1. 准备受保护的 `config.yaml`。
2. 确认 PostgreSQL 或 MySQL 已创建且连接字段正确。
3. 执行 `make migrate.up`。
4. 首次环境执行 `make seed.bootstrap`。
5. 检查 `/health/live` 和 `/health/ready`。

生产环境必须提供强 JWT 密钥、明确的跨域来源、默认数据库和 Console Casbin 权限执行器。

## 健康检查

```bash
curl -fsS http://127.0.0.1:8081/health/live
curl -fsS http://127.0.0.1:8081/health/ready
curl -fsS http://127.0.0.1:8081/metrics
```

存活检查（`live`）不访问外部依赖；就绪检查（`ready`）会并行检查当前服务实际启用的数据库、Redis 和队列后端。失败只返回依赖名称和归一化状态，详细原因写入日志。

## 日志与观测

- 日志统一由 `pkg/logger` 管理，文件按 `log.max_size_mb` 轮转、按 `log.max_age_days` 清理，不需要再配 logrotate；见 [配置说明](guide/configuration.md#log)。
- HTTP 请求中间件把请求 ID 和有效的追踪 ID 放入上下文 logger。service 使用 `log := logger.FromContext(ctx)` 后记录日志，即可携带 `request_id`，以及已启用追踪时的 `trace_id` 和 `span_id`；进程日志使用全局 logger。
- 异常恢复通过结构化日志记录错误，不转储原始请求头和查询参数。连接已断开时记录连接错误并停止处理，不继续写响应。
- OpenTelemetry（OTel）运行时由 Provider 持有并按逆序关闭。
- HTTP、数据库、Redis、外部 HTTP 和队列任务提供追踪与指标扩展点。
- 指标标签中不要包含 SQL、令牌、密码、Redis 参数或完整错误文本，避免泄露敏感信息或产生过多不同的标签值。

## 发布检查

```bash
make verify
go test -race ./...
go vet ./...
make quality.govuln
make admin.build
```

数据库发布至少验证正向迁移、迁移未完成状态（dirty）的拒绝处理、基础种子重复执行的幂等性和可回滚性。PostgreSQL 与 MySQL 使用相同版本号，但使用不同的方言目录；发布失败时不要手工删除 `grove_migrations`。

## 关闭与故障

- Provider 按注册逆序关闭资源。
- HTTP 服务先停止接收新请求，再等待现有请求完成。
- Worker 的健康检查服务、队列服务和调度器通过统一错误通道汇报。
- 就绪检查失败不等于进程崩溃；不要把就绪探针配置成存活探针。

详细部署示例见[部署指南](deployment/deploy.md)。
