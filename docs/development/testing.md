# 测试策略

本文档说明 Grove 当前采用的测试方式与编写约定。

## 测试范围

当前仓库主要包含：

- Go 单元测试与集成测试
- Testcontainers 驱动的真实 PostgreSQL 生命周期测试
- 路由与服务层测试
- 前端 unit、类型检查和 production build

## 最短路径

### 运行 Go 测试

```bash
go test ./...
```

### 运行统一验证

```bash
make verify
```

`make verify` 包含：

- Go 测试
- 后端构建
- 管理后台类型检查

前端专项验证：

```bash
make admin.typecheck
make admin.build
```

前端 unit 测试使用仓库现有 Vitest 配置：

```bash
cd web/admin-vben
pnpm test:unit
```

### 运行 PostgreSQL 集成测试

```bash
go test -tags=integration ./tests/integration -v
```

集成测试会通过 Testcontainers 启动 PostgreSQL，并验证迁移、bootstrap seed、重复执行密码不覆盖、dirty 状态和完整 down 生命周期。本地需要 Docker、OrbStack 或其他兼容容器运行时；本地容器不可用时测试会 Skip，CI 中容器启动失败会直接失败。

## 编写约定

- 测试文件使用 `*_test.go`
- 优先使用表驱动测试
- service 测试关注输入输出与错误分支
- router 测试关注认证、权限和响应状态
- 共享组件测试放在对应 `pkg/*` 或 `internal/*` 目录

## 重点场景

- 参数校验失败
- 认证失败
- 权限拒绝
- 正常成功响应
- 关键业务错误分支

## 边界

- `tests/integration/` 仅放依赖真实基础设施的跨包生命周期测试。
- 前端 unit 目前覆盖请求错误解析、认证状态、权限菜单过滤和基础组件；端到端浏览器测试不属于当前基线。

## 相关文档

- [快速上手](../guide/quickstart.md)
- [错误处理](./error-handling.md)
