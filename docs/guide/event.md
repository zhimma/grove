# 事件系统

本文档介绍 grove 框架的事件系统，支持同步和异步事件分发。

## 特性

- 🚀 **同步/异步支持** - 灵活选择执行方式
- 🔄 **Worker Pool** - 异步事件使用工作池，防止 goroutine 爆炸
- 📦 **类型安全** - 基于接口的事件定义
- 🎯 **监听者模式** - 支持多监听者
- ⏹️ **优雅关闭** - 支持等待异步事件完成

## 配置

```yaml
event:
  async: true              # 是否启用异步处理
  queue_size: 1000         # 异步队列大小
  workers: 10              # 工作协程数
```

## 快速开始

### 定义事件

```go
package event

// OrderCreated 订单创建事件
type OrderCreated struct {
    OrderID uint
    UserID  uint
    Amount  float64
}

// EventName 返回事件名称
func (e OrderCreated) EventName() string {
    return "order.created"
}
```

### 监听事件

```go
// 注册监听者
p.Event.Listen("order.created", func(ctx context.Context, payload OrderCreated) {
    // 发送通知
    notificationService.Send(ctx, payload.UserID, "订单已创建")
    
    // 更新统计
    statsService.Increment(ctx, "orders.created")
})
```

### 分发事件

```go
// 在 Service 中分发
func (s *OrderService) Create(ctx context.Context, req CreateRequest) (*model.Order, error) {
    // ... 创建订单逻辑
    
    // 分发事件（根据配置自动选择同步或异步）
    p.Event.Dispatch(ctx, event.OrderCreated{
        OrderID: order.ID,
        UserID:  order.UserID,
        Amount:  order.Amount,
    })
    
    return order, nil
}
```

## 详细用法

### 同步事件

```go
// 同步分发 - 等待所有监听者执行完成
p.Event.DispatchSync(ctx, event.OrderCreated{...})

// 或关闭异步模式
event:
  async: false
```

### 异步事件

```go
// 异步分发 - 立即返回，后台执行
p.Event.DispatchAsync(ctx, event.OrderCreated{...})

// 等待异步事件完成（优雅关闭）
p.Event.Wait()
```

### 多监听者

```go
// 多个监听者监听同一事件
p.Event.Listen("order.created", func(ctx context.Context, payload OrderCreated) {
    // 监听者 1：发送邮件
    emailService.SendOrderConfirmation(ctx, payload.OrderID)
})

p.Event.Listen("order.created", func(ctx context.Context, payload OrderCreated) {
    // 监听者 2：更新搜索索引
    searchService.IndexOrder(ctx, payload.OrderID)
})

p.Event.Listen("order.created", func(ctx context.Context, payload OrderCreated) {
    // 监听者 3：记录日志
    logger.Info().
        Uint("order_id", payload.OrderID).
        Msg("订单已创建")
})

// 分发事件时，所有监听者都会执行
p.Event.Dispatch(ctx, event.OrderCreated{...})
```

### 条件监听

```go
// 只监听特定条件的事件
p.Event.Listen("order.created", func(ctx context.Context, payload OrderCreated) {
    // 只处理大额订单
    if payload.Amount < 1000 {
        return
    }
    
    // 发送大额订单通知
    alertService.Send(ctx, "大额订单: " + strconv.Itoa(int(payload.OrderID)))
})
```

## 事件定义规范

### 命名规范

```go
// ✅ 使用过去时，描述已发生的事情
OrderCreated    // 订单已创建
PaymentReceived // 支付已收到
UserRegistered  // 用户已注册

// ✅ 使用点号分隔命名空间
order.created
user.registered
payment.received

// ❌ 避免使用命令式
CreateOrder     // 不好
SendEmail       // 不好
```

### 事件结构

```go
// ✅ 包含完整上下文
type OrderCreated struct {
    OrderID     uint      // 订单 ID
    UserID      uint      // 用户 ID
    Amount      float64   // 金额
    CreatedAt   time.Time // 创建时间
    RequestID   string    // 请求 ID（用于追踪）
}

// ❌ 避免过于简单
type OrderCreated struct {
    ID uint  // 信息不足
}
```

### 事件方法

```go
type OrderCreated struct {
    OrderID uint
    UserID  uint
}

// 必须实现 EventName 方法
func (e OrderCreated) EventName() string {
    return "order.created"
}

// 可选：实现 String 方法用于日志
func (e OrderCreated) String() string {
    return fmt.Sprintf("OrderCreated{OrderID: %d, UserID: %d}", e.OrderID, e.UserID)
}

// 可选：实现 Validate 方法
func (e OrderCreated) Validate() error {
    if e.OrderID == 0 {
        return errors.New("order_id is required")
    }
    if e.UserID == 0 {
        return errors.New("user_id is required")
    }
    return nil
}
```

## 在 Service 中使用

### 基础用法

```go
type OrderService struct {
    provider *provider.Provider
}

func (s *OrderService) Create(ctx context.Context, req CreateRequest) (*model.Order, error) {
    // 1. 创建订单
    order := &model.Order{
        UserID: req.UserID,
        Amount: req.Amount,
        Status: model.OrderStatusPending,
    }
    
    if err := s.provider.DB.Default().Create(order).Error; err != nil {
        return nil, err
    }
    
    // 2. 分发事件
    s.provider.Event.Dispatch(ctx, event.OrderCreated{
        OrderID:   order.ID,
        UserID:    order.UserID,
        Amount:    order.Amount,
        CreatedAt: order.CreatedAt,
    })
    
    return order, nil
}
```

### 事务中的事件

```go
func (s *OrderService) CreateWithEvent(ctx context.Context, req CreateRequest) (*model.Order, error) {
    var order *model.Order
    
    err := s.provider.Transaction.Execute(ctx, func(tx *gorm.DB) error {
        // 创建订单
        order = &model.Order{...}
        if err := tx.Create(order).Error; err != nil {
            return err
        }
        
        // 注意：不要在事务中直接分发事件
        // 如果事务回滚，事件已经发出会造成数据不一致
        return nil
    })
    
    if err != nil {
        return nil, err
    }
    
    // ✅ 事务提交后再分发事件
    s.provider.Event.Dispatch(ctx, event.OrderCreated{
        OrderID: order.ID,
        UserID:  order.UserID,
    })
    
    return order, nil
}
```

### 延迟事件（使用 Scheduler）

```go
func (s *OrderService) CreateWithReminder(ctx context.Context, req CreateRequest) (*model.Order, error) {
    order, err := s.createOrder(ctx, req)
    if err != nil {
        return nil, err
    }
    
    // 立即分发创建事件
    s.provider.Event.Dispatch(ctx, event.OrderCreated{...})
    
    // 1小时后发送提醒
    s.provider.Scheduler.ScheduleOnce("order_reminder", time.Now().Add(time.Hour), scheduler.JobFunc(func(ctx context.Context) error {
        s.provider.Event.Dispatch(ctx, event.OrderReminder{
            OrderID: order.ID,
            UserID:  order.UserID,
        })
        return nil
    }))
    
    return order, nil
}
```

## 监听者注册

### 集中注册

```go
// internal/bootstrap/event.go
package bootstrap

import (
    "context"
    "github.com/zhimma/grove/internal/event"
    "github.com/zhimma/grove/internal/provider"
)

func RegisterEventListeners(p *provider.Provider) {
    // 订单事件
    registerOrderEvents(p)
    
    // 用户事件
    registerUserEvents(p)
    
    // 支付事件
    registerPaymentEvents(p)
}

func registerOrderEvents(p *provider.Provider) {
    // 订单创建
    p.Event.Listen("order.created", func(ctx context.Context, payload event.OrderCreated) {
        // 发送通知
        p.NotificationService.Send(ctx, payload.UserID, notification.OrderCreated{OrderID: payload.OrderID})
        
        // 更新统计
        p.StatsService.Increment(ctx, "orders.created")
    })
    
    // 订单支付
    p.Event.Listen("order.paid", func(ctx context.Context, payload event.OrderPaid) {
        // 发送邮件
        p.EmailService.SendReceipt(ctx, payload.OrderID)
        
        // 更新库存
        p.InventoryService.Deduct(ctx, payload.OrderID)
    })
    
    // 订单发货
    p.Event.Listen("order.shipped", func(ctx context.Context, payload event.OrderShipped) {
        // 发送物流通知
        p.NotificationService.Send(ctx, payload.UserID, notification.OrderShipped{TrackingNumber: payload.TrackingNumber})
    })
}

func registerUserEvents(p *provider.Provider) {
    // 用户注册
    p.Event.Listen("user.registered", func(ctx context.Context, payload event.UserRegistered) {
        // 发送欢迎邮件
        p.EmailService.SendWelcome(ctx, payload.UserID)
        
        // 创建默认设置
        p.SettingService.CreateDefaults(ctx, payload.UserID)
    })
}
```

### 模块注册

```go
// app/console/service/article.go
func init() {
    // 模块初始化时注册监听者
    bootstrap.RegisterEventListener("article.created", func(ctx context.Context, payload event.ArticleCreated) {
        // 更新搜索索引
    })
}
```

## 常见事件模式

### 领域事件

```go
// 核心业务事件
type OrderCreated struct {
    OrderID uint
    UserID  uint
    Amount  float64
}

type PaymentReceived struct {
    PaymentID uint
    OrderID   uint
    Amount    float64
}

type InventoryChanged struct {
    ProductID uint
    Quantity  int
    Type      string // "in" | "out"
}
```

### 集成事件

```go
// 系统间集成
type UserSync struct {
    UserID    uint
    Action    string // "create" | "update" | "delete"
    Timestamp time.Time
}

type DataExport struct {
    ExportID  string
    Format    string
    Filters   map[string]interface{}
}
```

### 审计事件

```go
type AuditLog struct {
    UserID    uint
    Action    string
    Resource  string
    ResourceID uint
    Changes   map[string]interface{}
    IP        string
    UserAgent string
    Timestamp time.Time
}
```

## 错误处理

### 监听者错误

```go
p.Event.Listen("order.created", func(ctx context.Context, payload event.OrderCreated) {
    // 使用 recover 防止 panic 影响其他监听者
    defer func() {
        if r := recover(); r != nil {
            logger.Error().
                Interface("panic", r).
                Str("event", "order.created").
                Msg("监听器异常已恢复")
        }
    }()
    
    // 业务逻辑
    if err := doSomething(ctx, payload); err != nil {
        logger.Error().Err(err).Msg("事件处理失败")
        // 异步事件中错误不会返回，需要记录日志
    }
})
```

### 重试机制

```go
p.Event.Listen("order.created", func(ctx context.Context, payload event.OrderCreated) {
    // 带重试的监听者
    for i := 0; i < 3; i++ {
        err := sendNotification(ctx, payload)
        if err == nil {
            return
        }
        
        logger.Warn().Err(err).Int("attempt", i+1).Msg("事件处理重试中")
        time.Sleep(time.Second * time.Duration(i+1))
    }
    
    logger.Error().Msg("事件处理重试后仍失败")
})
```

## 监控与调试

### 事件统计

```go
// 获取事件统计
stats := p.Event.Stats()
fmt.Printf("Total events: %d\n", stats.TotalDispatched)
fmt.Printf("Sync events: %d\n", stats.SyncDispatched)
fmt.Printf("Async events: %d\n", stats.AsyncDispatched)
fmt.Printf("Queue size: %d\n", stats.QueueSize)
```

### 事件追踪

```go
// 使用请求 ID 追踪事件
func (s *OrderService) Create(ctx context.Context, req CreateRequest) (*model.Order, error) {
    requestID := reqctx.GetRequestID(ctx)
    
    order := &model.Order{...}
    // ...
    
    s.provider.Event.Dispatch(ctx, event.OrderCreated{
        OrderID:   order.ID,
        UserID:    order.UserID,
        RequestID: requestID,  // 传递请求 ID
    })
    
    return order, nil
}

// 监听者中使用
p.Event.Listen("order.created", func(ctx context.Context, payload event.OrderCreated) {
    // 记录关联的请求 ID
    logger.Info().
        Str("request_id", payload.RequestID).
        Uint("order_id", payload.OrderID).
        Msg("正在处理订单创建事件")
})
```

## 最佳实践

### 1. 事件粒度

```go
// ✅ 细粒度事件 - 单一职责
type OrderCreated struct{}
type OrderPaid struct{}
type OrderShipped struct{}

// ❌ 避免粗粒度事件
type OrderChanged struct {
    Type string // "created" | "paid" | "shipped"
}
```

### 2. 幂等性

```go
// ✅ 监听者应该是幂等的
p.Event.Listen("order.created", func(ctx context.Context, payload event.OrderCreated) {
    // 检查是否已处理
    if processed, _ := cache.Get(ctx, "event:order.created:"+strconv.Itoa(int(payload.OrderID))); processed {
        return
    }
    
    // 处理事件
    processOrder(ctx, payload)
    
    // 标记已处理
    cache.Put(ctx, "event:order.created:"+strconv.Itoa(int(payload.OrderID)), true, 86400)
})
```

### 3. 异步优先

```go
// ✅ 非关键操作使用异步
// 发送邮件、更新统计、记录日志等
p.Event.DispatchAsync(ctx, event.EmailShouldBeSent{...})

// ✅ 关键操作使用同步
// 库存扣减、资金操作等
p.Event.DispatchSync(ctx, event.InventoryShouldBeDeducted{...})
```

### 4. 错误隔离

```go
// ✅ 每个监听者独立处理错误
p.Event.Listen("order.created", func(ctx context.Context, payload event.OrderCreated) {
    // 监听者 1 失败不影响监听者 2
    if err := sendEmail(ctx, payload); err != nil {
        logger.Error().Err(err).Msg("邮件发送失败")
    }
})

p.Event.Listen("order.created", func(ctx context.Context, payload event.OrderCreated) {
    // 监听者 2 继续执行
    if err := updateStats(ctx, payload); err != nil {
        logger.Error().Err(err).Msg("统计更新失败")
    }
})
```

### 5. 上下文传递

```go
// ✅ 传递原始上下文
func (s *Service) DoSomething(ctx context.Context) {
    // 保持上下文链
    s.provider.Event.Dispatch(ctx, event.SomethingHappened{})
}

// ❌ 不要创建新上下文
func (s *Service) DoSomething(ctx context.Context) {
    // 会丢失请求 ID、超时等信息
    s.provider.Event.Dispatch(context.Background(), event.SomethingHappened{})
}
```

## 常见问题

### Q: 同步 vs 异步如何选择？

| 场景 | 建议 |
|------|------|
| 关键业务逻辑（库存、资金） | 同步 |
| 通知、邮件、统计 | 异步 |
| 需要立即看到结果的 | 同步 |
| 可以延迟执行的 | 异步 |

### Q: 异步事件丢失怎么办？

- 使用持久化队列（如 Redis Stream、Kafka）
- 实现事件重试机制
- 记录事件日志，支持重放

### Q: 监听者执行顺序？

监听者按注册顺序执行。如需特定顺序，建议合并为一个监听者。

### Q: 如何停止事件系统？

```go
// 优雅关闭，等待异步事件完成
p.Event.Stop()

// 或立即关闭
p.Event.StopNow()
```
