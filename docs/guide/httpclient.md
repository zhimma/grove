# HTTP 客户端

`pkg/httpclient` 用于调用第三方 HTTP API。共享 `Client` 只保存 Transport、基础 URL 和默认超时；header、query、body、retry 和 hook 都属于单次 Request。

## 快速开始

```go
client := httpclient.New().BaseURL("https://api.example.com")

resp, err := client.NewRequest(http.MethodGet, "/users").
    WithHeader("Authorization", "Bearer token").
    WithQueryParam("page", "1").
    DoWithContext(ctx)
if err != nil {
    return err
}

var users []User
if err := resp.JSON(&users); err != nil {
    return err
}
```

`Client` 配置方法返回新实例，不修改原值，因此 Provider 中的共享 Client 可被并发复用：

```go
apiClient := provider.HTTPClient.
    BaseURL("https://api.example.com").
    Timeout(10 * time.Second)
```

## 请求体

### JSON

```go
resp, err := client.NewRequest(http.MethodPost, "/users").
    JSON(map[string]string{"name": "grove"}).
    DoWithContext(ctx)
```

`Post/Put/Patch` 便捷方法仍可直接接收结构体并编码为 JSON：

```go
resp, err := client.PostWithContext(ctx, "/users", CreateUserRequest{Name: "grove"})
```

### Form

```go
resp, err := client.NewRequest(http.MethodPost, "/login").
    Form(map[string]string{
        "username": "admin",
        "password": "secret",
    }).
    DoWithContext(ctx)
```

### Multipart 流式上传

文件路径在发送请求时打开，并通过 pipe 流式写入，不会先完整读入内存：

```go
resp, err := client.NewRequest(http.MethodPost, "/upload").
    Form(map[string]string{"purpose": "avatar"}).
    AddFileFromPath("file", "/path/to/avatar.jpg").
    DoWithContext(ctx)
```

小文件或已有字节内容可使用 `AddFile`。普通 JSON/body 不能和 multipart 文件混用。

## 重试与幂等性

- GET、HEAD、OPTIONS 默认最多重试 2 次。
- 默认重试 transport error 和 5xx，不重试 4xx。
- POST、PUT、PATCH、DELETE 默认不重试。
- 非幂等请求必须显式设置 retry policy 或 idempotency key。
- backoff 会响应 context 取消。

显式 retry policy：

```go
resp, err := client.NewRequest(http.MethodPost, "/jobs").
    WithRetry(2, 200*time.Millisecond).
    JSON(payload).
    DoWithContext(ctx)
```

幂等 key 会写入 `Idempotency-Key`，并允许使用默认 retry policy：

```go
resp, err := client.NewRequest(http.MethodPost, "/orders").
    WithIdempotencyKey(orderRequestID).
    JSON(payload).
    DoWithContext(ctx)
```

任意 `io.Reader` body 默认只可消费一次；如果请求需要重试，会返回 `ErrBodyNotReplayable`。字符串、字节、JSON、Form 和 multipart 文件路径可重新创建请求体。

## 响应大小限制

普通响应默认最多读取 10 MiB。可按请求收紧限制：

```go
resp, err := client.NewRequest(http.MethodGet, "/metadata").
    MaxResponseBytes(1 << 20).
    DoWithContext(ctx)
if errors.Is(err, httpclient.ErrResponseTooLarge) {
    // 按上游协议处理超限
}
```

4xx 返回可检查的 `Response` 和 nil error。5xx 重试耗尽后同时返回最后一个 `Response` 和 error。

## 流式读取和下载

```go
err := client.NewRequest(http.MethodGet, "/large-file").
    Stream(ctx, func(chunk []byte) error {
        _, err := file.Write(chunk)
        return err
    })
```

下载到文件使用临时文件和流式复制；失败时删除临时文件，成功后再替换目标文件：

```go
err := client.NewRequest(http.MethodGet, "/report").
    WithHeader("Authorization", "Bearer token").
    DownloadToFile(ctx, "/tmp/report.pdf")
```

无请求级 header/query 时，也可使用：

```go
err := client.DownloadToFile("https://example.com/file.pdf", "/tmp/file.pdf")
```

## 请求级 hook

```go
resp, err := client.NewRequest(http.MethodGet, "/users").
    BeforeRequest(func(req *http.Request) error {
        req.Header.Set("X-Signature", sign(req))
        return nil
    }).
    AfterResponse(func(resp *httpclient.Response) error {
        metrics.Record(resp.StatusCode)
        return nil
    }).
    DoWithContext(ctx)
```

`BeforeRequest` 每次实际尝试都会执行，适合重新生成时间戳或签名。`AfterResponse` 只在普通响应的最终结果上执行。

## 测试与 Transport

测试第三方调用时注入自定义 `http.RoundTripper`，不要依赖真实网络：

```go
client := httpclient.New(httpclient.Config{
    BaseURL:  "https://example.test",
    Timeout:  time.Second,
    Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
        return response, nil
    }),
})
```

默认 Transport 已配置环境代理、连接池、dial timeout、TLS handshake timeout、response header timeout 和 idle timeout。
