# 错误处理

本文档介绍 grove 框架的错误处理机制和最佳实践。

## 错误类型

框架定义了统一的错误类型，用于区分不同的错误场景：

```go
type HTTPError struct {
    HTTPStatus int
    Message    string
    Code       string
    Data       map[string]interface{}
    Cause      error
}
```

### 预定义错误

```go
// 400 - 参数错误
pkgerrors.InvalidParams()
pkgerrors.InvalidParams().WithData(map[string]interface{}{
    "errors": map[string][]string{
        "email": {"邮箱格式不正确"},
        "age": {"年龄必须在 18-100 之间"},
    },
})

// 401 - 未认证
pkgerrors.Unauthorized()

// 403 - 无权限
pkgerrors.Forbidden()

// 404 - 未找到
pkgerrors.NotFound().WithMessage("用户不存在")
pkgerrors.NotFound().WithMessage("文章不存在")

// 409 - 冲突
pkgerrors.Conflict().WithMessage("邮箱已被注册")

// 429 - 限流
pkgerrors.TooManyRequests()

// 500 - 系统错误
pkgerrors.Internal()
```

## 使用方式

### Handler 层

```go
func (h *ArticleHandler) Create(c *gin.Context) {
    var req CreateRequest
    if err := validation.BindJSON(c, &req); err != nil {
        // 参数错误
        response.Fail(c, err)
        return
    }
    
    article, err := h.service.Create(c.Request.Context(), req)
    if err != nil {
        // 根据错误类型返回不同响应
        response.Fail(c, err)
        return
    }
    
    response.Success(c, article)
}
```

### Service 层

```go
func (s *ArticleService) Create(ctx context.Context, req CreateRequest) (*model.Article, error) {
    // 检查标题是否已存在
    var exists model.Article
    if err := s.provider.DB.Default().Where("title = ?", req.Title).First(&exists).Error; err == nil {
        return nil, pkgerrors.Conflict().WithMessage("文章标题已存在")
    }
    
    // 检查用户是否存在
    var user model.User
    if err := s.provider.DB.Default().First(&user, req.UserID).Error; err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            return nil, pkgerrors.NotFound().WithMessage("用户不存在")
        }
        return nil, pkgerrors.Internal().WithCause(err)
    }
    
    article := &model.Article{
        Title:   req.Title,
        Content: req.Content,
        UserID:  req.UserID,
    }
    
    if err := s.provider.DB.Default().Create(article).Error; err != nil {
        return nil, pkgerrors.Internal().WithCause(err)
    }
    
    return article, nil
}
```

### 错误包装

```go
// 包装底层错误
func (s *Service) DoSomething() error {
    if err := s.doStep1(); err != nil {
        return fmt.Errorf("步骤1失败: %w", err)
    }
    
    if err := s.doStep2(); err != nil {
        return fmt.Errorf("步骤2失败: %w", err)
    }
    
    return nil
}

// 使用 errors.Is 检查
err := service.DoSomething()
if errors.Is(err, ErrNotFound) {
    // 处理未找到错误
}
```

## 错误响应格式

### 成功响应

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "id": 1,
    "title": "Hello"
  },
  "request_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

### 参数错误（400）

```json
{
  "code": -1,
  "message": "请求参数错误",
  "data": {
    "error_code": "invalid_params",
    "errors": {
      "title": ["标题不能为空"],
      "email": ["邮箱格式不正确"]
    }
  },
  "request_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

### 未认证（401）

```json
{
  "code": -1,
  "message": "请先登录",
  "data": {
    "error_code": "unauthorized"
  },
  "request_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

### 无权限（403）

```json
{
  "code": -1,
  "message": "没有权限访问此资源",
  "data": {
    "error_code": "forbidden"
  },
  "request_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

### 未找到（404）

```json
{
  "code": -1,
  "message": "用户不存在",
  "data": {
    "error_code": "not_found"
  },
  "request_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

### 服务错误（500）

```json
{
  "code": -1,
  "message": "服务器内部错误",
  "data": {
    "error_code": "internal_error"
  },
  "request_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

## 最佳实践

### 1. 分层错误处理

```go
// ✅ Handler 层 - 处理 HTTP 相关错误
func (h *Handler) Create(c *gin.Context) {
    var req CreateRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        response.Fail(c, pkgerrors.InvalidParams().WithData(map[string]interface{}{
            "errors": parseValidationErrors(err),
        }))
        return
    }
    
    result, err := h.service.Create(c.Request.Context(), req)
    if err != nil {
        response.Fail(c, err)  // 直接传递 Service 层错误
        return
    }
    
    response.Success(c, result)
}

// ✅ Service 层 - 处理业务错误
func (s *Service) Create(ctx context.Context, req CreateRequest) (*Result, error) {
    // 业务验证
    if req.Amount <= 0 {
        return nil, pkgerrors.InvalidParams().WithHTTPStatus(422).WithData(map[string]interface{}{
            "errors": map[string][]string{
                "amount": {"金额必须大于0"},
            },
        }).WithMessage("请求参数校验失败")
    }
    
    // 业务逻辑
    if err := s.doBusinessLogic(); err != nil {
        return nil, pkgerrors.Internal().WithCause(err)
    }
    
    return result, nil
}

// ❌ 不要跨层处理错误
func (h *Handler) Create(c *gin.Context) {
    result, err := h.service.Create(...)
    if err != nil {
        // 不要在这里解析错误类型
        if strings.Contains(err.Error(), "not found") {
            c.JSON(404, ...)
        }
    }
}
```

### 2. 错误信息规范

```go
// ✅ 面向用户的错误信息
return nil, pkgerrors.NotFound().WithMessage("用户不存在")
return nil, pkgerrors.Conflict().WithMessage("邮箱已被注册")
return nil, pkgerrors.InvalidParams().WithHTTPStatus(422).WithMessage("密码长度至少8位")

// ✅ 面向开发者的错误信息
return nil, pkgerrors.Internal().WithCause(err)
return nil, pkgerrors.ServiceUnavailable().WithCause(err)

// ❌ 不要暴露敏感信息
return nil, pkgerrors.Internal().WithMessage("SQL: SELECT * FROM users WHERE id = 1")  // 不好
return nil, pkgerrors.Internal().WithMessage("连接 192.168.1.100:5432 失败")           // 不好
```

### 3. 错误日志记录

```go
// ✅ 记录详细错误信息
func (s *Service) Create(ctx context.Context, req CreateRequest) (*Result, error) {
    result, err := s.doSomething(ctx, req)
    if err != nil {
        logger.Error().
            Err(err).
            Str("action", "create_order").
            Interface("request", req).
            Str("user_id", getUserID(ctx)).
            Msg("创建订单失败")
        
        // 返回简化错误给客户端
        return nil, pkgerrors.Internal().WithMessage("创建订单失败，请稍后重试")
    }
    
    return result, nil
}

// ✅ 使用结构化日志
logger.Error().
    Err(err).
    Str("error_code", "DB_CONNECTION_FAILED").
    Str("db_host", cfg.Host).
    Int("db_port", cfg.Port).
    Msg("数据库连接失败")
```

### 4. 错误链追踪

```go
// ✅ 使用 fmt.Errorf 包装错误
func (s *Service) ProcessOrder(ctx context.Context, orderID uint) error {
    order, err := s.getOrder(ctx, orderID)
    if err != nil {
        return fmt.Errorf("获取订单失败: %w", err)
    }
    
    if err := s.validateOrder(ctx, order); err != nil {
        return fmt.Errorf("验证订单失败: %w", err)
    }
    
    if err := s.chargePayment(ctx, order); err != nil {
        return fmt.Errorf("扣款失败: %w", err)
    }
    
    return nil
}

// ✅ 使用 errors.Is 检查特定错误
err := service.ProcessOrder(ctx, 123)
if errors.Is(err, ErrOrderNotFound) {
    return errors.NewNotFound("订单")
}
if errors.Is(err, ErrInsufficientBalance) {
    return errors.NewUnprocessable("余额不足")
}
```

### 5.  panic 恢复

```go
// ✅ 使用 Recovery 中间件
func Recovery() gin.HandlerFunc {
    return func(c *gin.Context) {
        defer func() {
            if r := recover(); r != nil {
                // 记录 panic
                logger.Error().
                    Interface("panic", r).
                    Str("stack", string(debug.Stack())).
                    Msg("异常已恢复")
                
                // 返回 500 错误
                response.Fail(c, pkgerrors.Internal())
            }
        }()
        c.Next()
    }
}
```

### 6. 异步任务错误

```go
// ✅ 异步任务也要处理错误
func (s *Service) AsyncProcess(ctx context.Context, data Data) {
    go func() {
        defer func() {
            if r := recover(); r != nil {
                logger.Error().
                    Interface("panic", r).
                    Interface("data", data).
                    Msg("异步任务发生异常")
            }
        }()
        
        if err := s.process(ctx, data); err != nil {
            logger.Error().
                Err(err).
                Interface("data", data).
                Msg("异步任务执行失败")
        }
    }()
}
```

## 错误码规范

### 错误码格式

```
[类别][模块][具体错误]

示例：
A001 - 认证错误-Token无效
B002 - 业务错误-订单已支付
V003 - 验证错误-邮箱格式不正确
```

### 错误码定义

```go
const (
    // 认证错误 (Axxx)
    ErrCodeTokenInvalid     = "A001"
    ErrCodeTokenExpired     = "A002"
    ErrCodeTokenRevoked     = "A003"
    
    // 权限错误 (Pxxx)
    ErrCodePermissionDenied = "P001"
    ErrCodeRoleNotMatch     = "P002"
    
    // 验证错误 (Vxxx)
    ErrCodeValidationFailed = "V001"
    ErrCodeDuplicateEmail   = "V002"
    ErrCodeInvalidFormat    = "V003"
    
    // 业务错误 (Bxxx)
    ErrCodeOrderPaid        = "B001"
    ErrCodeInsufficientStock = "B002"
    ErrCodePaymentFailed    = "B003"
    
    // 系统错误 (Sxxx)
    ErrCodeDBError          = "S001"
    ErrCodeCacheError       = "S002"
    ErrCodeExternalAPIError = "S003"
)
```

### 使用错误码

```go
func (s *Service) CreateOrder(ctx context.Context, req CreateOrderRequest) (*Order, error) {
    // 检查库存
    if !s.checkStock(req.ProductID, req.Quantity) {
        return nil, &errors.HTTPError{
            Code:      422,
            Message:   "库存不足",
            ErrorCode: ErrCodeInsufficientStock,
        }
    }
    
    // ...
}
```

## 国际化错误

### 多语言支持

```go
// 错误消息映射
var errorMessages = map[string]map[string]string{
    "en": {
        "user_not_found": "User not found",
        "invalid_email":  "Invalid email format",
    },
    "zh": {
        "user_not_found": "用户不存在",
        "invalid_email":  "邮箱格式不正确",
    },
}

// 根据语言获取错误消息
func GetErrorMessage(code string, lang string) string {
    if msgs, ok := errorMessages[lang]; ok {
        if msg, ok := msgs[code]; ok {
            return msg
        }
    }
    return errorMessages["en"][code]  // 默认英文
}
```

### 使用中间件设置语言

```go
func I18n() gin.HandlerFunc {
    return func(c *gin.Context) {
        lang := c.GetHeader("Accept-Language")
        if lang == "" {
            lang = "zh"
        }
        c.Set("lang", lang)
        c.Next()
    }
}
```

## 调试模式

### 开发环境显示详细错误

```go
func ErrorHandler() gin.HandlerFunc {
    return func(c *gin.Context) {
        c.Next()
        
        if len(c.Errors) > 0 {
            err := c.Errors.Last()
            
            if gin.Mode() == gin.DebugMode {
                // 开发环境显示详细错误
                c.JSON(500, gin.H{
                    "error": err.Error(),
                    "stack": err.Err.(stackTracer).StackTrace(),
                })
            } else {
                // 生产环境隐藏细节
                c.JSON(500, gin.H{
                    "error": "服务器内部错误",
                })
            }
        }
    }
}
```

## 常见问题

### Q: 如何自定义错误响应格式？

```go
func CustomErrorResponse(err error) gin.H {
    if httpErr, ok := err.(*errors.HTTPError); ok {
        return gin.H{
            "success": false,
            "error": gin.H{
                "code":    httpErr.ErrorCode,
                "message": httpErr.Message,
            },
        }
    }
    
    return gin.H{
        "success": false,
        "error": gin.H{
            "code":    "unknown",
            "message": err.Error(),
        },
    }
}
```

### Q: 如何处理第三方 API 错误？

```go
func (s *Service) CallExternalAPI(ctx context.Context) error {
    resp, err := s.httpClient.GetWithContext(ctx, "https://api.example.com/data")
    if err != nil {
        return pkgerrors.ServiceUnavailable().WithCause(err).WithMessage("外部 API 调用失败")
    }
    
    if !resp.IsSuccess() {
        // 解析第三方错误
        var apiErr ExternalAPIError
        _ = resp.JSON(&apiErr)
        
        return &errors.HTTPError{
            HTTPStatus: resp.StatusCode,
            Message:    fmt.Sprintf("外部服务错误: %s", apiErr.Message),
            Code:       ErrCodeExternalAPIError,
        }
    }
    
    return nil
}
```

### Q: 如何记录错误但不返回给客户端？

```go
func (s *Service) DoSomething(ctx context.Context) error {
    err := s.step1()
    if err != nil {
        // 记录详细错误
        logger.Error().Err(err).Msg("步骤一执行失败")
        
        // 返回简化错误
        return pkgerrors.Internal().WithMessage("操作失败，请稍后重试")
    }
    
    return nil
}
```
