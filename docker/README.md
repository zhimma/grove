# Grove 后端镜像

根目录 `Dockerfile` 通过 `SERVICE` 参数选择要构建的服务。该参数只能是 `api`、`console` 或 `worker`：

```bash
docker build --build-arg SERVICE=api -t grove-api:local .
docker build --build-arg SERVICE=console -t grove-console:local .
docker build --build-arg SERVICE=worker -t grove-worker:local .
```

三个服务构建共享 Go 模块与编译缓存；服务选择在依赖下载之后，避免切换 `SERVICE` 时重复下载依赖。构建上下文排除 `bin/`、`tmp/`、`.tooling/` 等本地产物。

运行镜像只包含对应服务二进制，以用户 ID（UID）和组 ID（GID）均为 `10001` 的非 root 用户执行，并预建 `/app/logs` 和 `/app/storage`。迁移使用发布产物中的 `bin/grove` 在受控的一次性任务中执行；不要通过改变服务镜像的启动入口临时执行迁移。

## 运行时约束

运行时必须把已填写完成且受保护的 `config.yaml` 只读挂载到 `/app/config.yaml`。镜像不包含 `config.yaml`、数据库密码、Redis 密码或 JWT 密钥；不要通过 Docker 构建参数、镜像标签、命令行参数或日志传递密钥。

推荐基线：

- `--read-only` 使根文件系统只读；为 `/tmp` 提供受限 tmpfs。
- 仅为 `./logs` 和启用本地存储时的 `./storage` 挂载独立可写数据卷；对象存储不需要本地存储卷。
- 保留非 root 用户，附加 `--security-opt no-new-privileges:true` 和 `--cap-drop ALL`。
- `console` 仅暴露到受控网络；`api` 由 TLS 反向代理暴露；Worker 的健康检查与指标端口不对公网开放。
- 终止宽限期应大于 `server.shutdown_timeout`，使进程处理 `SIGTERM` 后完成优雅关闭。

完整 `docker run` 示例、挂载权限和预发布验收见[部署指南](../docs/deployment/deploy.md)。

## 镜像扫描

当前 GitLab CI 会构建三个服务镜像，但**尚未在流水线中执行镜像漏洞扫描**：GitLab Runner 的镜像仓库、扫描器缓存和漏洞例外策略尚未作为本仓库配置确认。请将实际镜像扫描作为发布检查执行。

在推广镜像前，由可访问 Docker 套接字且已批准扫描器来源的环境执行至少一次高危与严重漏洞检查。例如（命令中的镜像和版本应纳入组织镜像信任策略）：

```bash
docker run --rm \
  -v /var/run/docker.sock:/var/run/docker.sock \
  aquasec/trivy:0.56.2 \
  image --ignore-unfixed --severity HIGH,CRITICAL --exit-code 1 grove-console:local
```

对 `api`、`console`、`worker` 分别扫描，并保存扫描报告、镜像摘要（digest）、例外审批和修复结论。确认 GitLab Runner 与镜像仓库方案后，应把同一命令或等效的 OCI 镜像归档扫描加入主要 CI 流水线，并使扫描失败阻止发布。

本地没有 Docker 时只能验证 Dockerfile 的静态内容，不能把 `go build` 结果当作镜像、扫描或预发布验收。
