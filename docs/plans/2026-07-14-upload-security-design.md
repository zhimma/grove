# 请求体与文件上传安全设计

## 边界与目标

所有 HTTP 入口统一使用 `http.MaxBytesReader` 限制请求体，配置项为 `server.max_body_bytes`。已知 `Content-Length` 超限时立即返回 413；分块传输或长度未知时由读取端识别 `http.MaxBytesError` 并返回相同错误语义。

上传接口不再接受任意目录作为安全策略。客户端只提交命名用途 `purpose`，服务端从配置的上传策略中决定目录、最大字节数、扩展名、MIME 和是否公开托管。未指定用途时使用显式的默认策略。

## 上传校验

上传文件必须依次通过：

1. 文件存在且大小大于零。
2. 实际大小不超过用途策略限制。
3. 小写扩展名在白名单中。
4. 读取前 512 字节，通过 `http.DetectContentType` 得到实际 MIME。
5. 实际 MIME 在策略白名单中。
6. 常用格式使用文件签名再次确认：JPEG、PNG、GIF、WebP、PDF、ZIP/OOXML、OLE 文档和文本。
7. 公开策略始终拒绝 SVG、HTML、JavaScript、XML 等可执行或主动内容，即使未来配置误加白名单。

校验只读取固定大小文件头，随后使用 `io.MultiReader` 把文件头接回原始流。驱动收到的是已校验流、实际 MIME 和可信大小，不再自行重新打开或整体读取文件。

## 存储写入

`Driver` 使用 `PutStream` 接收流。Local 驱动在目标目录创建临时文件，通过 `io.Copy` 流式写入，成功后原子重命名，失败或取消时删除临时文件。S3 驱动把同一已校验流直接交给对象存储客户端，并使用实际 MIME 作为 `Content-Type`。

Local 静态文件路由增加 `X-Content-Type-Options: nosniff`，防止浏览器把允许的文本或下载内容嗅探成可执行类型。

## 前端协议

前端删除 COS、OSS 和未实现 S3 直传适配器，只接受后端 `ClientConfig` 返回的 `local` 或 `s3` 驱动。当前两种驱动都通过 Console multipart 接口上传；S3 STS 直传只有在独立实现并验证 SigV4、策略范围和进度/重试后才启用。

上传组件提交 `disk` 和 `purpose`，前端大小与 accept 校验只用于体验，服务端策略始终是最终安全边界。

## 默认策略

- `avatar`：5 MiB，目录 `avatars`，允许 JPEG、PNG、GIF、WebP，公开托管。
- `document`：20 MiB，目录 `documents`，允许 PDF、TXT、CSV、DOC、DOCX、XLS、XLSX，公开托管并强制拒绝主动内容。

## 验证

- JSON 和 multipart 请求体超限返回 413。
- 扩展名、MIME、magic bytes、空文件和大小限制均有失败测试。
- HTML/SVG/脚本伪装成允许扩展名时被拒绝。
- Local 写入不使用 `io.ReadAll`，取消或失败不留下部分文件。
- Local 静态响应包含 `nosniff`。
- Console 路由覆盖 avatar/document 成功和超限/伪装失败场景。
- Vue 类型检查覆盖新的 `disk / purpose / ClientConfig` 协议。
