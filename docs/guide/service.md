# 服务层

本文档说明 Grove 中 `service` 层的职责和编写方式。

## 职责

`service` 负责：

- 业务逻辑
- 事务编排
- 任务投递
- 缓存协调
- 事件分发

`service` 不负责：

- HTTP 参数绑定
- HTTP 响应格式
- 页面展示逻辑

## 最短路径

### 定义服务

```go
type ArticleService struct {
	dbs *database.Connections
}

func NewArticleService(dbs *database.Connections) *ArticleService {
	return &ArticleService{dbs: dbs}
}
```

### 定义输入结构

```go
type CreateArticleInput struct {
	Title   string
	Content string
}
```

### 服务方法

```go
func (s *ArticleService) Create(ctx context.Context, input CreateArticleInput) (*model.Article, error) {
	if s.dbs == nil || s.dbs.Default() == nil {
		return nil, errx.ServiceUnavailable().WithMessage("默认数据库未配置")
	}
	article := &model.Article{
		Title:   input.Title,
		Content: input.Content,
	}
	if err := s.dbs.Default().WithContext(ctx).Create(article).Error; err != nil {
		return nil, errx.Internal().WithCause(err)
	}
	return article, nil
}
```

## 使用约定

- `service` 方法统一接收 `context.Context`
- 推荐使用 `Input` / `Output` 结构体
- 事务、缓存、事件、任务都在 service 中协调
- 业务错误返回 `pkg/errx` 定义的错误类型
- 依赖在启动或路由装配层从 `Provider` 取出，再通过构造函数传入；service 不持有完整 `Provider`

## 边界

- 复杂查询可以放在 model 查询辅助中，但业务决策仍在 service
- `service` 不直接依赖 Gin Context
- `service` 不负责前端字段结构适配

## 相关文档

- [开发规范](../01-%E5%BC%80%E5%8F%91%E8%A7%84%E8%8C%83.md)
- [数据库与模型](./database.md)
- [错误处理](../development/error-handling.md)
