# 服务层

本文档介绍 grove 框架的 Service 层设计和最佳实践。

## 职责划分

### 三层架构

```
HTTP Request → Handler → Service → Model → Database
                    ↓
              Transaction/Cache/Event
```

| 层级 | 职责 | 示例 |
|------|------|------|
| Handler | HTTP 请求处理、参数绑定、响应格式化 | 验证请求、调用 Service、返回 JSON |
| Service | 业务逻辑、事务管理、任务分发 | 创建订单、发送通知、更新统计 |
| Model | 数据模型、查询封装 | 定义结构体、数据库查询方法 |

## Service 定义

### 基础 Service

```go
package service

import (
    "context"
    "github.com/zhimma/grove/internal/model"
    "github.com/zhimma/grove/internal/provider"
)

type ArticleService struct {
    provider *provider.Provider
}

func NewArticleService(p *provider.Provider) *ArticleService {
    return &ArticleService{provider: p}
}
```

### 完整 CRUD Service

```go
package service

import (
    "context"
    "fmt"
    
    "github.com/zhimma/grove/internal/model"
    "github.com/zhimma/grove/internal/provider"
    pkgerrors "github.com/zhimma/grove/pkg/errors"
)

// ArticleService 文章服务
type ArticleService struct {
    provider *provider.Provider
}

// NewArticleService 创建服务实例
func NewArticleService(p *provider.Provider) *ArticleService {
    return &ArticleService{provider: p}
}

// CreateRequest 创建请求
type CreateRequest struct {
    Title   string
    Content string
    UserID  uint
}

// Create 创建文章
func (s *ArticleService) Create(ctx context.Context, req CreateRequest) (*model.Article, error) {
    article := &model.Article{
        Title:   req.Title,
        Content: req.Content,
        UserID:  req.UserID,
        Status:  1,
    }
    
    if err := s.provider.DB.Default().Create(article).Error; err != nil {
        return nil, pkgerrors.Internal().WithCause(err)
    }
    
    return article, nil
}

// ListRequest 列表请求
type ListRequest struct {
    Page    int
    PerPage int
    Status  int
    Keyword string
}

// ListResponse 列表响应
type ListResponse struct {
    List  []*model.Article
    Total int64
}

// List 获取文章列表
func (s *ArticleService) List(ctx context.Context, req ListRequest) (*ListResponse, error) {
    var articles []*model.Article
    var total int64
    
    db := s.provider.DB.Default().Model(&model.Article{})
    
    // 条件过滤
    if req.Status > 0 {
        db = db.Where("status = ?", req.Status)
    }
    if req.Keyword != "" {
        db = db.Where("title LIKE ?", "%"+req.Keyword+"%")
    }
    
    // 统计总数
    if err := db.Count(&total).Error; err != nil {
        return nil, pkgerrors.Internal().WithCause(err)
    }
    
    // 分页查询
    if err := db.Order("created_at DESC").
        Offset((req.Page - 1) * req.PerPage).
        Limit(req.PerPage).
        Find(&articles).Error; err != nil {
        return nil, pkgerrors.Internal().WithCause(err)
    }
    
    return &ListResponse{
        List:  articles,
        Total: total,
    }, nil
}

// Get 获取文章详情
func (s *ArticleService) Get(ctx context.Context, id uint) (*model.Article, error) {
    article := &model.Article{}
    
    if err := s.provider.DB.Default().First(article, id).Error; err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            return nil, pkgerrors.NotFound().WithMessage("文章不存在")
        }
        return nil, pkgerrors.Internal().WithCause(err)
    }
    
    return article, nil
}

// UpdateRequest 更新请求
type UpdateRequest struct {
    ID      uint
    Title   string
    Content string
}

// Update 更新文章
func (s *ArticleService) Update(ctx context.Context, req UpdateRequest) (*model.Article, error) {
    article, err := s.Get(ctx, req.ID)
    if err != nil {
        return nil, err
    }
    
    article.Title = req.Title
    article.Content = req.Content
    
    if err := s.provider.DB.Default().Save(article).Error; err != nil {
        return nil, pkgerrors.Internal().WithCause(err)
    }
    
    return article, nil
}

// Delete 删除文章
func (s *ArticleService) Delete(ctx context.Context, id uint) error {
    result := s.provider.DB.Default().Delete(&model.Article{}, id)
    if result.Error != nil {
        return pkgerrors.Internal().WithCause(result.Error)
    }
    
    if result.RowsAffected == 0 {
        return errors.NewNotFound(fmt.Sprintf("article %d", id))
    }
    
    return nil
}
```

## 事务管理

### 使用 Transaction Manager

```go
func (s *OrderService) Create(ctx context.Context, req CreateOrderRequest) (*model.Order, error) {
    var order *model.Order
    
    err := s.provider.Transaction.Execute(ctx, func(tx *gorm.DB) error {
        // 1. 创建订单
        order = &model.Order{
            UserID: req.UserID,
            Amount: req.Amount,
            Status: model.OrderStatusPending,
        }
        if err := tx.Create(order).Error; err != nil {
            return err
        }
        
        // 2. 扣减库存
        if err := tx.Model(&model.Product{}).
            Where("id = ? AND stock >= ?", req.ProductID, req.Quantity).
            UpdateColumn("stock", gorm.Expr("stock - ?", req.Quantity)).Error; err != nil {
            return err
        }
        
        // 3. 创建订单项
        item := &model.OrderItem{
            OrderID:   order.ID,
            ProductID: req.ProductID,
            Quantity:  req.Quantity,
            Price:     req.Price,
        }
        if err := tx.Create(item).Error; err != nil {
            return err
        }
        
        return nil
    })
    
    if err != nil {
        return nil, pkgerrors.Internal().WithCause(err)
    }
    
    return order, nil
}
```

### 嵌套事务

```go
func (s *OrderService) Process(ctx context.Context, orderID uint) error {
    return s.provider.Transaction.Execute(ctx, func(tx *gorm.DB) error {
        // 获取订单
        order := &model.Order{}
        if err := tx.First(order, orderID).Error; err != nil {
            return err
        }
        
        // 扣款（嵌套事务）
        if err := s.deductBalance(ctx, tx, order.UserID, order.Amount); err != nil {
            return err
        }
        
        // 发货（嵌套事务）
        if err := s.createShipment(ctx, tx, order); err != nil {
            return err
        }
        
        // 更新订单状态
        order.Status = model.OrderStatusPaid
        return tx.Save(order).Error
    })
}

func (s *OrderService) deductBalance(ctx context.Context, tx *gorm.DB, userID uint, amount int64) error {
    return s.provider.Transaction.Execute(ctx, func(tx2 *gorm.DB) error {
        return tx2.Model(&model.User{}).
            Where("id = ? AND balance >= ?", userID, amount).
            UpdateColumn("balance", gorm.Expr("balance - ?", amount)).Error
    }, transaction.WithDB(tx))
}
```

## 缓存使用

### 基础缓存

```go
func (s *ArticleService) Get(ctx context.Context, id uint) (*model.Article, error) {
    cacheKey := fmt.Sprintf("article:%d", id)
    
    // 尝试从缓存获取
    result, err := s.provider.Cache.Default().Remember(ctx, cacheKey, 3600, func() (any, error) {
        article := &model.Article{}
        if err := s.provider.DB.Default().First(article, id).Error; err != nil {
            return nil, err
        }
        return article, nil
    })
    
    if err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            return nil, errors.NewNotFound(fmt.Sprintf("article %d", id))
        }
        return nil, err
    }
    
    return result.(*model.Article), nil
}
```

### 缓存更新策略

```go
func (s *ArticleService) Update(ctx context.Context, req UpdateRequest) (*model.Article, error) {
    article, err := s.Get(ctx, req.ID)
    if err != nil {
        return nil, err
    }
    
    article.Title = req.Title
    article.Content = req.Content
    
    if err := s.provider.DB.Default().Save(article).Error; err != nil {
        return nil, err
    }
    
    // 删除缓存
    cacheKey := fmt.Sprintf("article:%d", req.ID)
    s.provider.Cache.Default().Forget(ctx, cacheKey)
    
    // 删除列表缓存
    s.provider.Cache.Default().Forget(ctx, "articles:list:*")
    
    return article, nil
}
```

## 事件分发

### 业务事件

```go
func (s *OrderService) Create(ctx context.Context, req CreateOrderRequest) (*model.Order, error) {
    // ... 创建订单逻辑
    
    // 分发事件
    s.provider.Event.Dispatch(ctx, event.OrderCreated{
        OrderID: order.ID,
        UserID:  order.UserID,
        Amount:  order.Amount,
    })
    
    return order, nil
}

// 事件定义
package event

type OrderCreated struct {
    OrderID uint
    UserID  uint
    Amount  int64
}

func (e OrderCreated) EventName() string {
    return "order.created"
}
```

### 事件监听

```go
// 在 bootstrap 中注册
func RegisterEventListeners(p *provider.Provider) {
    // 订单创建事件
    p.Event.Listen("order.created", func(ctx context.Context, payload event.OrderCreated) {
        // 发送通知
        notificationService.Send(ctx, payload.UserID, "订单已创建")
        
        // 更新统计
        statsService.Increment(ctx, "orders.created")
    })
    
    // 支付成功事件
    p.Event.Listen("order.paid", func(ctx context.Context, payload event.OrderPaid) {
        // 发送邮件
        emailService.SendReceipt(ctx, payload.OrderID)
    })
}
```

## 复杂业务场景

### 状态机

```go
type OrderStatus int

const (
    OrderStatusPending  OrderStatus = 1 // 待支付
    OrderStatusPaid     OrderStatus = 2 // 已支付
    OrderStatusShipped  OrderStatus = 3 // 已发货
    OrderStatusComplete OrderStatus = 4 // 已完成
    OrderStatusCanceled OrderStatus = 5 // 已取消
)

var statusTransitions = map[OrderStatus][]OrderStatus{
    OrderStatusPending:  {OrderStatusPaid, OrderStatusCanceled},
    OrderStatusPaid:     {OrderStatusShipped, OrderStatusCanceled},
    OrderStatusShipped:  {OrderStatusComplete},
    OrderStatusComplete: {},
    OrderStatusCanceled: {},
}

func (s *OrderService) TransitionStatus(ctx context.Context, orderID uint, newStatus OrderStatus) error {
    return s.provider.Transaction.Execute(ctx, func(tx *gorm.DB) error {
        order := &model.Order{}
        if err := tx.First(order, orderID).Error; err != nil {
            return err
        }
        
        // 检查状态转换是否合法
        allowed := statusTransitions[order.Status]
        if !contains(allowed, newStatus) {
            return pkgerrors.InvalidParams().WithHTTPStatus(422).WithMessage("状态流转不合法")
        }
        
        order.Status = newStatus
        return tx.Save(order).Error
    })
}
```

### 批量操作

```go
func (s *ArticleService) BatchUpdateStatus(ctx context.Context, ids []uint, status int) error {
    if len(ids) == 0 {
        return nil
    }
    
    result := s.provider.DB.Default().
        Model(&model.Article{}).
        Where("id IN ?", ids).
        Update("status", status)
    
    if result.Error != nil {
        return result.Error
    }
    
    // 清除相关缓存
    for _, id := range ids {
        cacheKey := fmt.Sprintf("article:%d", id)
        s.provider.Cache.Default().Forget(ctx, cacheKey)
    }
    
    return nil
}
```

### 导入导出

```go
func (s *ArticleService) Export(ctx context.Context, req ExportRequest) ([]byte, error) {
    // 查询数据
    var articles []*model.Article
    db := s.provider.DB.Default().Model(&model.Article{})
    
    if req.Status > 0 {
        db = db.Where("status = ?", req.Status)
    }
    if req.StartTime != nil {
        db = db.Where("created_at >= ?", req.StartTime)
    }
    
    if err := db.Find(&articles).Error; err != nil {
        return nil, err
    }
    
    // 生成 CSV
    var buf bytes.Buffer
    writer := csv.NewWriter(&buf)
    
    // 写入表头
    writer.Write([]string{"ID", "Title", "Content", "Status", "Created At"})
    
    // 写入数据
    for _, article := range articles {
        writer.Write([]string{
            fmt.Sprintf("%d", article.ID),
            article.Title,
            article.Content,
            fmt.Sprintf("%d", article.Status),
            article.CreatedAt.Format("2006-01-02 15:04:05"),
        })
    }
    
    writer.Flush()
    return buf.Bytes(), nil
}
```

## 最佳实践

### 1. 单一职责

```go
// ✅ 每个 Service 只负责一个领域
type ArticleService struct{}
type CommentService struct{}
type TagService struct{}

// ❌ 避免大而全的 Service
type ContentService struct {  // 包含文章、评论、标签...
}
```

### 2. 依赖注入

```go
// ✅ 通过 Provider 获取依赖
type ArticleService struct {
    provider *provider.Provider
}

// ❌ 避免全局变量
var db = database.GetDB()  // 不好
```

### 3. 错误处理

```go
// ✅ 包装底层错误，返回标准错误
return nil, pkgerrors.Internal().WithCause(err)

// ✅ 使用特定错误类型
return nil, pkgerrors.NotFound().WithMessage("文章不存在")
return nil, pkgerrors.InvalidParams().WithHTTPStatus(422).WithMessage("标题不能为空")

// ❌ 不要直接返回原始错误
return nil, err
```

### 4. 上下文传递

```go
// ✅ 始终传递 context
func (s *ArticleService) Get(ctx context.Context, id uint) (*model.Article, error) {
    // 使用 ctx 进行数据库查询、缓存操作等
}

// ❌ 不要使用 context.Background()
db.First(&article, id)  // 不好，无法取消和超时
```

### 5. 日志记录

```go
// ✅ 记录关键操作
logger.Info().
    Str("action", "create_article").
    Uint("article_id", article.ID).
    Uint("user_id", req.UserID).
    Msg("文章已创建")

// ✅ 记录错误
logger.Error().
    Err(err).
    Str("action", "create_article").
    Interface("request", req).
    Msg("文章创建失败")
```

## 测试

### 单元测试

```go
func TestArticleService_Create(t *testing.T) {
    // 初始化测试 Provider
    p := test.NewProvider()
    
    service := NewArticleService(p)
    
    article, err := service.Create(context.Background(), CreateRequest{
        Title:   "Test",
        Content: "Content",
        UserID:  1,
    })
    
    assert.NoError(t, err)
    assert.NotNil(t, article)
    assert.Equal(t, "Test", article.Title)
}
```

### 集成测试

```go
func TestArticleService_Integration(t *testing.T) {
    // 使用真实数据库
    p := test.NewProviderWithDB(testDBConfig)
    defer p.Close()
    
    service := NewArticleService(p)
    
    // 测试完整流程
    ctx := context.Background()
    
    // 创建
    article, err := service.Create(ctx, CreateRequest{...})
    require.NoError(t, err)
    
    // 获取
    found, err := service.Get(ctx, article.ID)
    require.NoError(t, err)
    assert.Equal(t, article.Title, found.Title)
    
    // 更新
    updated, err := service.Update(ctx, UpdateRequest{ID: article.ID, ...})
    require.NoError(t, err)
    
    // 删除
    err = service.Delete(ctx, article.ID)
    require.NoError(t, err)
}
```
