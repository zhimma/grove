# 路由与控制器

本文档说明 Grove 中路由注册和 handler 编写的基本约定。

## 路由注册位置

- `app/api/internal/router`
- `app/console/internal/router`

路由注册应按服务边界放置，不跨服务复用具体业务路由。

## 最短路径

### 注册路由

```go
articles := route.Wrap(protected.Group("/articles"))
articles.GET("", h.List).Name("内容管理.文章列表")
articles.POST("", h.Create).Name("内容管理.创建文章")
```

### 编写 handler

```go
type ArticleHandler struct {
	service *service.ArticleService
}

func (h *ArticleHandler) List(c *gin.Context) {
	result, err := h.service.List(c.Request.Context(), service.ListArticlesInput{})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, result)
}
```

## Handler 约定

- 负责参数绑定、上下文读取、调用 service、输出响应
- 不直接创建数据库连接、缓存实例或任务客户端
- 简单字段映射直接写在 handler 内，不额外拆 trivial mapper

## 路由约定

- 受保护接口应注册在受保护路由组
- `console` 路由建议补充 `route.Name(...)`
- `.Name(...)` 影响接口展示文案，不影响实际鉴权
- `.Ignore()` 仅用于明确不进入权限目录的接口

## 相关文档

- [开发规范](../01-%E5%BC%80%E5%8F%91%E8%A7%84%E8%8C%83.md)
- [Console 架构与权限](../02-console-%E6%9E%B6%E6%9E%84%E4%B8%8E%E6%9D%83%E9%99%90.md)
