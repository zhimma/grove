# HTTP 客户端

本文档介绍当前框架里的 `pkg/httpclient`。它用于调用第三方 API，风格接近链式客户端，但接口以当前仓库真实实现为准。

## 当前能力

- 基础请求：`Get/Post/Put/Patch/Delete`
- 链式客户端配置：`BaseURL/WithHeader/WithQueryParam/WithRetry/Timeout`
- 请求构建器：`NewRequest(...).JSON(...).Form(...).AddFile(...).Do()`
- 文件下载：`Download`、`DownloadToFile`
- 流式请求：`Stream`
- 请求前后钩子：`BeforeRequest`、`AfterResponse`

## 快速开始

```go
client := httpclient.New().
    BaseURL("https://api.example.com").
    WithHeader("Accept", "application/json").
    WithRetry(3, time.Second)

resp, err := client.Get("/users")
if err != nil {
    return err
}

var users []User
if err := resp.JSON(&users); err != nil {
    return err
}
```

## 常见用法

### GET 与查询参数

```go
client := httpclient.New().
    BaseURL("https://api.example.com").
    WithQueryParam("page", "1").
    WithQueryParam("limit", "10")

resp, err := client.Get("/users")
```

### POST JSON

```go
payload := map[string]any{
    "name":  "John",
    "email": "john@example.com",
}

resp, err := client.Post("/users", payload)
```

### 使用请求构建器

```go
resp, err := client.NewRequest(http.MethodPost, "/users").
    WithHeader("Authorization", "Bearer token").
    JSON(map[string]string{"name": "test"}).
    Do()
```

### 表单请求

```go
resp, err := client.PostForm("/login", map[string]string{
    "username": "admin",
    "password": "secret",
})
```

### 文件上传

```go
resp, err := client.NewRequest(http.MethodPost, "/upload").
    AddFileFromPath("avatar", "/path/to/avatar.jpg").
    Do()
```

### 下载文件

```go
resp, err := client.Download("https://example.com/file.pdf")
if err != nil {
    return err
}

err = client.DownloadToFile("https://example.com/file.pdf", "/tmp/file.pdf")
```

### 流式读取

```go
err := client.Stream(ctx, http.MethodGet, "/large-file", nil, func(chunk []byte) error {
    _, err := file.Write(chunk)
    return err
})
```

## 在服务层中使用

推荐通过 `provider.Provider` 注入，而不是在 handler 里直接创建客户端。

```go
type PaymentService struct {
    provider *provider.Provider
}

func NewPaymentService(p *provider.Provider) *PaymentService {
    return &PaymentService{provider: p}
}

func (s *PaymentService) Query(ctx context.Context, paymentID string) error {
    resp, err := s.provider.HTTPClient.
        Clone().
        BaseURL("https://api.example.com").
        WithHeader("Authorization", "Bearer token").
        GetWithContext(ctx, "/payments/"+paymentID)
    if err != nil {
        return err
    }

    if !resp.IsSuccess() {
        return fmt.Errorf("上游请求失败: %d", resp.StatusCode)
    }
    return nil
}
```

## 约定与注意事项

- `Clone()` 会复制基础 URL、超时、请求头、查询参数、重试策略以及自定义 `Transport`。
- 5xx 响应会被视为可重试错误；4xx 不会重试。
- 推荐把认证头、基础 URL、重试策略放在客户端配置层，不要在每个 handler 里重复拼接。
- 测试第三方调用时，优先注入自定义 `Transport`，不要依赖真实网络。
- 日志仍统一走 `pkg/logger`，重试日志会记录请求 URL 与尝试次数。
