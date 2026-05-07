# Grove 文档

Grove 是一个面向现代全栈 Go 应用的 monorepo Web 框架，当前主线是 `console-first` 管理后台模板。

## 推荐阅读顺序

- [快速上手](guide/quickstart.md)
- [pkg 基础组件](guide/pkg-components.md)
- [开发规范](01-开发规范.md)
- [Console 架构与权限](02-console-架构与权限.md)
- [Console 新增模块指南](03-console-新增模块指南.md)
- [响应与错误处理规范](04-响应与错误处理规范.md)
- [部署指南](deployment/deploy.md)

## 指南

- [项目结构](guide/structure.md)
- [快速上手](guide/quickstart.md)
- [配置说明](guide/configuration.md)
- [路由](guide/routing.md)
- [服务层](guide/service.md)
- [数据库](guide/database.md)
- [权限](guide/permission.md)
- [pkg 基础组件](guide/pkg-components.md)
- [缓存](guide/cache.md)
- [事件](guide/event.md)
- [队列任务](guide/queue.md)
- [计划任务](guide/scheduler.md)
- [HTTP 客户端](guide/httpclient.md)

## 模板约定

- `app/console` 是当前最完整的业务模板。
- `app/api` 保持对外 API 示例定位。
- `cmd/artisan` 是唯一保留的 CLI 入口。
- 优先使用 `go run ./cmd/artisan/main.go about` 查看当前框架约定。
- 默认验收命令是 `make verify`。
- 管理后台菜单来自前端本地路由，后端不做菜单同步。
- 运行时日志统一使用 `pkg/logger`，底层基于 zerolog，日志文案尽量使用中文。
