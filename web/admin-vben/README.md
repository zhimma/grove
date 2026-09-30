# Grove Console 前端

基于 Vben Admin 的 Vue 3 / TypeScript 管理后台。此工作区只交付 `apps/console`；启动整个项目请从仓库根目录的 [README](../../README.md) 开始。

## 开发

在仓库根目录执行：

```bash
make admin.install
make admin.dev
```

默认打开 `http://localhost:5666`，连接 `http://127.0.0.1:8081` 的 Console API。开发环境设置见 `apps/console/.env.development`。Makefile 通过 Corepack 使用 `package.json` 固定的 pnpm。

## 目录

- `apps/console/src/api/`：按资源拆分的接口与统一请求客户端。
- `apps/console/src/views/`：按业务域组织的页面，`_core/` 保留登录、个人资料和错误页。
- `apps/console/src/locales/`：业务语言文件和语言加载入口。
- `apps/console/src/components/`：页面共用组件；ResourcePage 的自定义表单由页面传入。
- `apps/console/src/router/`：路由、菜单与访问控制。
- `packages/`：Vben UI、状态、请求等工作区包。
- `internal/`：构建、类型与代码检查配置。

业务模块接入见[新增 Console 模块](../../docs/03-console-新增模块指南.md)，路由 name 是菜单授权 key，不随文件移动而改变。

## 验证与发布

```bash
make admin.typecheck admin.test admin.lint admin.circular admin.build
```

产物为 `apps/console/dist/`。生产 API 默认同源，需由反向代理转发 `/console/v1/`；发布方式见[部署指南](../../docs/deployment/deploy.md#5-发布管理后台前端)。

## 上游

前端源自 [Vben Admin](https://github.com/vbenjs/vue-vben-admin)，保留其 [MIT 许可证](LICENSE)与版权声明。工作区包的版本号不代表 Grove 后端的发布版本。
