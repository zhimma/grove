# 路由与控制器

本文档介绍 grove 框架的路由定义和 Handler（控制器）编写规范。

## 路由定义

### 基础路由

框架使用 Gin 作为 HTTP 框架，路由定义在 `app/{app}/internal/router/router.go`。

```go
package router

import (
    "github.com/gin-gonic/gin"
    "github.com/zhimma/grove/internal/provider"
)

func RegisterRoutes(r *gin.Engine, p *provider.Provider) {
    // 基础路由
    r.GET("/ping", func(c *gin.Context) {
        c.JSON(200, gin.H{"message": "pong"})
    })
}
```

### 路由分组

```go
func RegisterRoutes(r *gin.Engine, p *provider.Provider) {
    // API 版本分组
    api := r.Group("/api/v1")
    
    // 文章路由组
    articles := api.Group("/articles")
    {
        articles.GET("", articleHandler.List)
        articles.POST("", articleHandler.Create)
        articles.GET("/:id", articleHandler.Get)
        articles.PUT("/:id", articleHandler.Update)
        articles.DELETE("/:id", articleHandler.Delete)
    }
    
    // 用户路由组（带中间件）
    users := api.Group("/users")
    users.Use(middleware.JWTAuth(p.TokenManager))
    {
        users.GET("", userHandler.List)
        users.GET("/me", userHandler.Me)
    }
}
```

### 路由参数

```go
// 路径参数
r.GET("/articles/:id", handler.Get)

// 可选参数（Gin 不支持，需要手动处理）
r.GET("/articles/:id/*action", handler.Action)

// 在 Handler 中获取
func (h *Handler) Get(c *gin.Context) {
    id := c.Param("id")
    action := c.Param("action")
}
```

### 路由方法

```go
// GET - 获取资源
r.GET("/articles", handler.List)

// POST - 创建资源
r.POST("/articles", handler.Create)

// PUT - 全量更新
r.PUT("/articles/:id", handler.Update)

// PATCH - 部分更新
r.PATCH("/articles/:id", handler.PartialUpdate)

// DELETE - 删除资源
r.DELETE("/articles/:id", handler.Delete)

// HEAD - 获取元信息
r.HEAD("/articles/:id", handler.Head)

// OPTIONS - 获取支持的方法
r.OPTIONS("/articles", handler.Options)
```

## Handler 编写

### 基础 Handler

```go
package handler

import (
    "github.com/gin-gonic/gin"
    "github.com/zhimma/grove/internal/provider"
    "github.com/zhimma/grove/pkg/response"
)

type ArticleHandler struct {
    provider *provider.Provider
}

func NewArticleHandler(p *provider.Provider) *ArticleHandler {
    return &ArticleHandler{provider: p}
}

func (h *ArticleHandler) List(c *gin.Context) {
    response.Success(c, gin.H{
        "message": "article list",
    })
}
```

### 参数绑定

#### Query 参数

```go
type ListRequest struct {
    Page    int    `form:"page" binding:"min=1" default:"1"`
    PerPage int    `form:"per_page" binding:"min=1,max=100" default:"20"`
    Keyword string `form:"keyword"`
    Status  int    `form:"status" binding:"oneof=0 1"`
}

func (h *ArticleHandler) List(c *gin.Context) {
    var req ListRequest
    if err := validation.BindQuery(c, &req); err != nil {
        response.Fail(c, err)
        return
    }
    
    // req.Page, req.PerPage...
}
```

#### JSON Body

```go
type CreateRequest struct {
    Title   string `json:"title" binding:"required,max=255"`
    Content string `json:"content" binding:"required"`
    Tags    []string `json:"tags"`
}

func (h *ArticleHandler) Create(c *gin.Context) {
    var req CreateRequest
    if err := validation.BindJSON(c, &req); err != nil {
        response.Fail(c, err)
        return
    }
    
    // req.Title, req.Content...
}
```

#### URI 参数

```go
type GetRequest struct {
    ID uint `uri:"id" binding:"required,min=1"`
}

func (h *ArticleHandler) Get(c *gin.Context) {
    var req GetRequest
    if err := validation.BindUri(c, &req); err != nil {
        response.Fail(c, err)
        return
    }
    
    // req.ID
}
```

#### Form 数据

```go
type UploadRequest struct {
    File     *multipart.FileHeader `form:"file" binding:"required"`
    Filename string                `form:"filename"`
}

func (h *ArticleHandler) Upload(c *gin.Context) {
    var req UploadRequest
    if err := validation.BindForm(c, &req); err != nil {
        response.Fail(c, err)
        return
    }
    
    // 保存文件
    c.SaveUploadedFile(req.File, "./uploads/"+req.File.Filename)
}
```

### 参数验证规则

```go
type Request struct {
    // 必填
    Name string `json:"name" binding:"required"`
    
    // 长度限制
    Title string `json:"title" binding:"required,min=3,max=255"`
    
    // 数值范围
    Age int `json:"age" binding:"min=0,max=150"`
    
    // 枚举值
    Status int `json:"status" binding:"oneof=0 1 2"`
    
    // 邮箱
    Email string `json:"email" binding:"required,email"`
    
    // URL
    Website string `json:"website" binding:"url"`
    
    // 正则
    Phone string `json:"phone" binding:"regexp=^1[3-9]\\d{9}$"`
    
    // 自定义验证
    Password string `json:"password" binding:"required,min=8"`
    Confirm  string `json:"confirm" binding:"eqfield=Password"`
    
    // 嵌套结构
    Address Address `json:"address" binding:"required"`
    
    // 数组
    Tags []string `json:"tags" binding:"max=10"`
}
```

## 响应处理

### 成功响应

```go
// 简单响应
response.Success(c, nil)

// 带数据
response.Success(c, article)

// 带分页
response.Success(c, gin.H{
    "list":  articles,
    "total": total,
    "page":  page,
})
```

### 错误响应

```go
// 通用错误
response.Fail(c, errors.New("something went wrong"))

// 参数错误
response.Fail(c, pkgerrors.InvalidParams().WithData(map[string]interface{}{
    "errors": map[string][]string{
    "email": {"邮箱格式不正确"},
    },
}))

// 未找到
response.Fail(c, pkgerrors.NotFound().WithMessage("文章不存在"))

// 权限不足
response.Fail(c, pkgerrors.Forbidden().WithMessage("无权限访问"))

// 服务错误
response.Fail(c, pkgerrors.Internal())
```

### 自定义响应

```go
// 原始 JSON
c.JSON(200, gin.H{
    "code": 0,
    "data": data,
})

// 带 HTTP 状态码
c.JSON(201, gin.H{
    "code": 0,
    "message": "created",
})

// 重定向
c.Redirect(302, "/new-url")

// 文件下载
c.File("./file.pdf")

// 流式响应
c.Stream(func(w io.Writer) bool {
    w.Write([]byte("data"))
    return true
})
```

## 中间件使用

### 全局中间件

在 `internal/bootstrap/middleware.go` 中注册：

```go
func SetupMiddleware(r *gin.Engine, p *provider.Provider) {
    // 恢复 panic
    r.Use(middleware.Recovery())
    
    // 请求 ID
    r.Use(middleware.RequestID())
    
    // 访问日志
    r.Use(middleware.AccessLog(p.Logger))
    
    // CORS
    r.Use(middleware.CORS(p.Config.CORS))
}
```

### 路由组中间件

```go
// 认证路由组
auth := r.Group("/auth")
auth.Use(middleware.JWTAuth(p.TokenManager))
{
    auth.GET("/me", handler.Me)
    auth.POST("/logout", handler.Logout)
}

// 多中间件
admin := r.Group("/admin")
admin.Use(middleware.JWTAuth(p.TokenManager))
admin.Use(middleware.Permission(p.Casbin, "admin"))
{
    admin.GET("/users", handler.ListUsers)
}
```

### 自定义中间件

```go
package middleware

import (
    "time"
    "github.com/gin-gonic/gin"
)

// RateLimit 限流中间件
func RateLimit(maxRequests int, window time.Duration) gin.HandlerFunc {
    return func(c *gin.Context) {
        // 限流逻辑
        if !allowRequest(c.ClientIP()) {
            c.AbortWithStatus(429)
            return
        }
        c.Next()
    }
}

// Auth 认证中间件
func JWTAuth(manager *auth.Manager) gin.HandlerFunc {
    return func(c *gin.Context) {
        token := c.GetHeader("Authorization")
        if token == "" {
            c.AbortWithStatusJSON(401, gin.H{"message": "未登录或登录已失效"})
            return
        }
        
        claims, err := manager.Parse(token)
        if err != nil {
            c.AbortWithStatusJSON(401, gin.H{"message": "访问令牌无效"})
            return
        }
        
        // 设置用户信息到上下文
        c.Set("user_id", claims.UserID)
        c.Set("user", claims)
        c.Next()
    }
}
```

## 请求上下文

### 获取当前用户

```go
func (h *ArticleHandler) Create(c *gin.Context) {
    // 从上下文中获取用户 ID
    userID, exists := c.Get("user_id")
    if !exists {
        response.Fail(c, pkgerrors.Unauthorized().WithMessage("缺少用户身份信息"))
        return
    }
    
    // 使用用户 ID
    article.UserID = userID.(uint)
}
```

### 获取请求元信息

```go
func (h *ArticleHandler) List(c *gin.Context) {
    // 请求 ID
    requestID := c.GetString("request_id")
    
    // 客户端 IP
    clientIP := c.ClientIP()
    
    // User-Agent
    userAgent := c.GetHeader("User-Agent")
}
```

## RESTful API 设计

### URL 设计

```
GET    /api/v1/articles          # 列表
GET    /api/v1/articles/:id      # 详情
POST   /api/v1/articles          # 创建
PUT    /api/v1/articles/:id      # 更新
PATCH  /api/v1/articles/:id      # 部分更新
DELETE /api/v1/articles/:id      # 删除
```

### 嵌套资源

```
GET    /api/v1/articles/:id/comments     # 文章评论列表
POST   /api/v1/articles/:id/comments     # 添加评论
GET    /api/v1/articles/:id/comments/:cid # 评论详情
DELETE /api/v1/articles/:id/comments/:cid # 删除评论
```

### 动作资源

```
POST   /api/v1/articles/:id/publish      # 发布文章
POST   /api/v1/articles/:id/like         # 点赞
POST   /api/v1/articles/batch-delete     # 批量删除
```

## 最佳实践

### 1. 保持 Handler 简洁

```go
// ✅ 好的做法
func (h *ArticleHandler) Create(c *gin.Context) {
    var req CreateRequest
    if err := validation.BindJSON(c, &req); err != nil {
        response.Fail(c, err)
        return
    }
    
    article, err := h.service.Create(c.Request.Context(), req)
    if err != nil {
        response.Fail(c, err)
        return
    }
    
    response.Success(c, article)
}

// ❌ 不好的做法 - 业务逻辑在 Handler 中
func (h *ArticleHandler) Create(c *gin.Context) {
    var req CreateRequest
    c.ShouldBindJSON(&req)
    
    // 验证
    if req.Title == "" {
        c.JSON(400, gin.H{"error": "title required"})
        return
    }
    
    // 业务逻辑
    article := &model.Article{Title: req.Title}
    h.provider.DB.Default().Create(article)
    
    // 发送通知
    sendNotification(article.UserID)
    
    // 更新统计
    updateStats()
    
    c.JSON(200, article)
}
```

### 2. 统一的错误处理

```go
// ✅ 使用统一的错误处理
if err != nil {
    response.Fail(c, err)
    return
}

// ❌ 不要直接返回错误
c.JSON(500, gin.H{"error": err.Error()})
```

### 3. 参数验证

```go
// ✅ 使用结构体验证
type Request struct {
    Title string `json:"title" binding:"required,max=255"`
}

// ❌ 不要手动验证
title := c.PostForm("title")
if title == "" {
    // 错误处理
}
if len(title) > 255 {
    // 错误处理
}
```

### 4. 使用 Service 层

```go
// ✅ Handler 只负责 HTTP 层
func (h *ArticleHandler) Create(c *gin.Context) {
    // ... 参数绑定
    article, err := h.service.Create(ctx, req)
    // ... 响应
}

// Service 处理业务逻辑
func (s *ArticleService) Create(ctx context.Context, req CreateRequest) (*Article, error) {
    // 业务逻辑
    // 事务管理
    // 事件分发
}
```

## 调试技巧

### 打印请求信息

```go
func (h *ArticleHandler) Debug(c *gin.Context) {
    // 请求方法
    fmt.Println("Method:", c.Request.Method)
    
    // 请求路径
    fmt.Println("Path:", c.Request.URL.Path)
    
    // Query 参数
    fmt.Println("Query:", c.Request.URL.Query())
    
    // Header
    fmt.Println("Headers:", c.Request.Header)
    
    // Body
    body, _ := c.GetRawData()
    fmt.Println("Body:", string(body))
}
```

### 使用 curl 测试

```bash
# GET 请求
curl http://localhost:8081/api/v1/articles

# POST 请求
curl -X POST http://localhost:8081/api/v1/articles \
  -H "Content-Type: application/json" \
  -d '{"title":"Test","content":"Content"}'

# 带认证
curl http://localhost:8081/api/v1/articles \
  -H "Authorization: Bearer <token>"

# 保存响应
curl http://localhost:8081/api/v1/articles -o response.json
```
