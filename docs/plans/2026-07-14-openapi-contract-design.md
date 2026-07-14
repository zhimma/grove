# Task 21：OpenAPI 合同与路由漂移设计

## Status

Accepted

## Context

当前 API 文档只描述 health 和可选 demo 路由，Console 文档只覆盖认证、工作台、会话和角色的部分接口。Admin、SystemConfig、Storage、Log 等已注册路由没有合同，已有 operation 也只有 summary、简单参数和 200 描述，没有 operation ID、请求体、响应 envelope 或错误结构。

文档结构由各应用手写 `map[string]any` 间接生成，编译器无法约束字段。Gin 路由与 OpenAPI 没有自动比较，新路由可以在没有文档的情况下进入 CI。Scalar 脚本固定依赖 jsDelivr，不能切换到自托管资源。

## Requirements

- API 和 Console 的全部业务路由都有唯一 operation ID。
- JSON、query、path 和 multipart 请求都有类型化 schema。
- 成功响应描述统一 `code/message/data/request_id` envelope，错误响应描述统一错误 envelope。
- schema 优先从真实 Go request/response 类型生成，避免重复维护字段清单。
- CI 比较 Gin method/path 与 OpenAPI operation，任一方向漂移都失败。
- Scalar 保持默认 CDN 可用，同时允许配置同源路径或其他自托管脚本 URL。
- 不引入新的 Web 框架、代码生成器或运行时依赖。

## Decision

### 1. `internal/docsui` 提供小型类型化 OpenAPI 模型

使用 Go struct 表达 Document、Operation、RequestBody、Response、Parameter、Schema 和 Components。动态 path/method 与 schema properties 使用有明确值类型的 map，不再让应用层维护任意 `map[string]any`。

Document 提供添加 operation 和 component schema 的小型 helper。各应用只负责声明自己的 path、method、operation ID、请求类型和响应类型，不建立注解系统或通用 API 框架。

### 2. 从真实 Go 类型生成 schema

`SchemaFor` 通过 reflection 读取 `json` tag、字段类型和 `binding` 约束，生成 object、array、primitive、required 和 nullable 信息。`ParametersFor` 读取 `form` 或 `uri` tag，并展开嵌入 query 结构。

反射只发生在构建文档时，不进入业务请求链。无法精确表达的 `map[string]any` 使用 `additionalProperties: true`，上传接口的 binary multipart schema 显式声明。

### 3. 应用内 contract 是唯一文档源

API 和 Console 分别在自己的 `internal/docs/contract.go` 维护合同。合同直接引用 handler request/response 和 service output 类型，避免复制字段。API server URL 以真实 `api.prefix` 为准，`docs.base_path` 只作为旧配置的 fallback，避免路由前缀出现第二个真相源。

路由仍由现有 router/handler 注册；不让文档层参与业务 dispatch，也不把路由注册改造成新的 DSL。该选择保留简单 Go 控制流，同时通过测试约束两个来源同步。

### 4. 路由漂移测试比较 method 与规范化 path

`docsui.CompareRoutes` 接收 Gin `RoutesInfo`、Document 和业务前缀：

- Gin `:id` 与 `*path` 转换为 OpenAPI `{id}` 和 `{path}`。
- 忽略 Gin 自动 HEAD/OPTIONS，以及不属于业务前缀的 docs、health 和静态文件路由。
- 检查实际路由缺文档、文档多余路由和 operation ID 重复。

API 测试同时覆盖 demo 启用和关闭；Console 测试覆盖全部 `/console/v1` 路由。CI 增加显式 contract test step，使漂移失败原因可见。

### 5. Scalar 脚本 URL 配置化

`DocsConfig` 增加 `scalar_script_url`。默认值仍为官方 npm CDN 地址，传入 `/assets/scalar.js` 时使用同源自托管资源，也可传入部署方控制的 HTTPS 地址。CSP 根据 URL 只放行对应脚本源。

生产环境继续通过 `docs.enabled=false` 完全关闭文档端点；不在应用内复制第三方 Scalar 文件。

## Failure Modes And Mitigations

- 新路由未写合同：route/spec test 输出缺失的 method/path 并使 CI 失败。
- 删除路由后遗留合同：反向比较输出多余 operation。
- operation ID 重复：合同校验直接失败。
- Go 字段变更：schema 自动跟随真实类型；接口语义仍由 contract 中的 path 和 response type 显式确认。
- 自托管 URL 配错：页面仍可返回，但浏览器脚本加载失败；配置测试确保默认值和 CSP 生成正确。
- 反射遇到递归类型：schema 生成器在递归边界回退为 object，避免无限递归。

## Consequences

### Positive

- 文档字段与真实 Go 类型共享一个结构来源。
- 新增、删除或改名路由不能静默绕过 CI。
- 应用 contract 清晰可搜索，不依赖注解生成器或额外 CLI。
- Scalar 可在离线或受控网络环境自托管。

### Negative

- 路由注册和 contract operation 仍是两个显式来源，需要测试保持同步。
- reflection 不能自动理解所有业务语义，特殊 format、enum 和 multipart 仍需少量显式 schema。
- handler 类型改名会影响 docs package 编译，这是有意的合同耦合。

## Alternatives Considered

### 引入 swag、ogen 或其他生成器

暂不采用。会增加注解、生成步骤、版本管理和 CI 工具依赖，不符合脚手架当前的最小可用边界。未来需要生成客户端时可基于稳定 OpenAPI 输出再评估。

### 把路由注册和文档合成统一 DSL

拒绝。会让普通 Gin 注册被框架包装层吞掉，扩大核心抽象并增加迁移成本。

### 继续全手写 OpenAPI JSON/YAML

拒绝。request/response 字段会与 Go 类型重复维护，当前缺失正是这种漂移的结果。

## Verification

- docsui schema、parameter、envelope、route normalization 和 Scalar script/CSP 单元测试。
- API demo on/off route/spec contract tests。
- Console 全业务路由 route/spec contract test。
- OpenAPI JSON 包含 operation ID、requestBody、typed response、error response 和 components 的测试。
- `go test ./...`、`go test -race ./...`、`go vet ./...`、`make build`、`git diff --check`。
