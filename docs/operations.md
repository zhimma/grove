# Grove 运行与运维

## 启动前检查

1. 准备受保护的 `config.yaml`。
2. 确认 PostgreSQL 已创建且连接字段正确。
3. 执行 `make migrate.up`。
4. 首次环境执行 `make seed.bootstrap`。
5. 检查 `/health/live` 和 `/health/ready`。

生产环境必须提供强 JWT secret、明确 CORS origin、默认数据库和 Console Casbin enforcer。

## 健康检查

```bash
curl -fsS http://127.0.0.1:8081/health/live
curl -fsS http://127.0.0.1:8081/health/ready
curl -fsS http://127.0.0.1:8081/metrics
```

`live` 不访问外部依赖；`ready` 会并行检查当前服务实际启用的数据库、Redis 和 queue 后端。失败只返回依赖名称和归一化状态，详细原因写入日志。

## 日志与观测

- 日志统一由 `pkg/logger` 管理。
- OTel runtime 由 Provider 持有并按逆序关闭。
- HTTP、数据库、Redis、外部 HTTP 和 Job 支持 trace/metrics 扩展点。
- 不记录 SQL、token、密码、Redis 参数或完整错误文本到高基数指标标签。

## 发布检查

```bash
make verify
go test -race ./...
go vet ./...
govulncheck ./...
make admin.build
```

数据库发布至少验证：迁移 up、dirty 状态拒绝、bootstrap 幂等和可回滚性。发布失败时不要手工删除 `grove_migrations`。

## 关闭与故障

- Provider 按注册逆序关闭资源。
- HTTP 服务先停止接收新请求，再等待现有请求完成。
- Worker 的 health listener、queue server 和 scheduler 由统一错误通道汇报。
- readiness 失败不等于进程崩溃；不要把 readiness 探针配置成 liveness。

详细部署示例见 [deployment/deploy.md](deployment/deploy.md)。
