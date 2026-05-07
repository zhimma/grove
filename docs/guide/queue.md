# 队列任务

本文档介绍 grove 框架的队列任务系统，基于 Asynq 封装。

## 特性

- 📦 **多种队列** - 支持默认、关键、低优先级队列
- ⏱️ **延迟任务** - 支持定时和延迟执行
- 🔄 **自动重试** - 失败任务自动重试
- 📊 **任务监控** - 内置 Web UI 监控
- 🎯 **优先级** - 任务优先级控制

## 配置

```yaml
job:
  enabled: true
  concurrency: 10              # 并发处理数
  queues:
    default: 3                 # 默认队列权重
    critical: 6                # 关键队列权重
    low: 1                     # 低优先级队列权重
```

## 快速开始

### 定义任务

```go
package task

import (
    "context"
    "encoding/json"
    
    "github.com/hibiken/asynq"
)

// SendEmailTask 发送邮件任务
const SendEmailTask = "email:send"

// SendEmailPayload 任务负载
type SendEmailPayload struct {
    To      string `json:"to"`
    Subject string `json:"subject"`
    Body    string `json:"body"`
}

// NewSendEmailTask 创建任务
func NewSendEmailTask(payload SendEmailPayload) (*asynq.Task, error) {
    data, err := json.Marshal(payload)
    if err != nil {
        return nil, err
    }
    
    return asynq.NewTask(SendEmailTask, data), nil
}

// HandleSendEmailTask 处理任务
func HandleSendEmailTask(ctx context.Context, t *asynq.Task) error {
    var payload SendEmailPayload
    if err := json.Unmarshal(t.Payload(), &payload); err != nil {
        return err
    }
    
    // 发送邮件
    return sendEmail(ctx, payload.To, payload.Subject, payload.Body)
}
```

### 注册处理器

```go
// internal/bootstrap/job.go
package bootstrap

import (
    "github.com/zhimma/grove/internal/provider"
    "github.com/zhimma/grove/internal/task"
)

func RegisterJobHandlers(p *provider.Provider) {
    // 注册任务处理器
    p.JobServer.RegisterHandler(task.SendEmailTask, task.HandleSendEmailTask)
    p.JobServer.RegisterHandler(task.ProcessImageTask, task.HandleProcessImageTask)
    p.JobServer.RegisterHandler(task.GenerateReportTask, task.HandleGenerateReportTask)
}
```

### 分发任务

```go
func (s *OrderService) Create(ctx context.Context, req CreateRequest) (*Order, error) {
    // ... 创建订单
    
    // 分发邮件任务
    task, err := task.NewSendEmailTask(task.SendEmailPayload{
        To:      user.Email,
        Subject: "订单创建成功",
        Body:    fmt.Sprintf("您的订单 %d 已创建", order.ID),
    })
    if err != nil {
        return nil, err
    }
    
    _, err = s.provider.JobClient.Enqueue(task)
    if err != nil {
        // 记录错误但不影响主流程
        logger.Error().Err(err).Msg("邮件任务入队失败")
    }
    
    return order, nil
}
```

## 详细用法

### 任务选项

```go
import "github.com/hibiken/asynq"

// 立即执行（默认）
_, err := client.Enqueue(task)

// 延迟执行
_, err := client.Enqueue(task, asynq.ProcessIn(5*time.Minute))

// 指定时间执行
_, err := client.Enqueue(task, asynq.ProcessAt(time.Now().Add(time.Hour)))

// 指定队列
_, err := client.Enqueue(task, asynq.Queue("critical"))

// 设置超时
_, err := client.Enqueue(task, asynq.Timeout(30*time.Second))

// 设置重试
_, err := client.Enqueue(task, asynq.MaxRetry(5))

// 设置保留时间
_, err := client.Enqueue(task, asynq.Retention(24*time.Hour))

// 设置任务 ID
_, err := client.Enqueue(task, asynq.TaskID("unique-task-id"))

// 组合选项
_, err := client.Enqueue(task,
    asynq.Queue("critical"),
    asynq.MaxRetry(3),
    asynq.Timeout(60*time.Second),
)
```

### 任务优先级

```go
// 关键任务 - 高优先级
criticalTask, _ := task.NewCriticalTask(payload)
_, err := client.Enqueue(criticalTask, asynq.Queue("critical"))

// 普通任务 - 默认优先级
defaultTask, _ := task.NewDefaultTask(payload)
_, err := client.Enqueue(defaultTask, asynq.Queue("default"))

// 低优先级任务
lowTask, _ := task.NewLowTask(payload)
_, err := client.Enqueue(lowTask, asynq.Queue("low"))
```

### 批量入队

```go
// 批量入队任务
var tasks []*asynq.Task
for _, email := range emails {
    task, _ := task.NewSendEmailTask(task.SendEmailPayload{
        To: email,
        Subject: "Newsletter",
        Body: content,
    })
    tasks = append(tasks, task)
}

// 批量入队
_, err := client.EnqueueBatch(tasks)
```

### 任务去重

```go
// 使用唯一任务 ID 防止重复
_, err := client.Enqueue(task, asynq.TaskID(fmt.Sprintf("email:%s:%d", email, orderID)))

// 使用唯一选项（带 TTL）
_, err := client.Enqueue(task, 
    asynq.Unique(24*time.Hour),  // 24 小时内不重复
)
```

## 在 Service 中使用

### 邮件服务

```go
type EmailService struct {
    provider *provider.Provider
}

func (s *EmailService) SendAsync(ctx context.Context, to, subject, body string) error {
    task, err := task.NewSendEmailTask(task.SendEmailPayload{
        To:      to,
        Subject: subject,
        Body:    body,
    })
    if err != nil {
        return err
    }
    
    _, err = s.provider.JobClient.Enqueue(task)
    return err
}

func (s *EmailService) SendBatchAsync(ctx context.Context, emails []string, subject, body string) error {
    var tasks []*asynq.Task
    
    for _, email := range emails {
        task, _ := task.NewSendEmailTask(task.SendEmailPayload{
            To:      email,
            Subject: subject,
            Body:    body,
        })
        tasks = append(tasks, task)
    }
    
    _, err := s.provider.JobClient.EnqueueBatch(tasks)
    return err
}
```

### 图片处理

```go
type ImageService struct {
    provider *provider.Provider
}

func (s *ImageService) ProcessAsync(ctx context.Context, imageID uint, operations []string) error {
    task, err := task.NewProcessImageTask(task.ProcessImagePayload{
        ImageID:    imageID,
        Operations: operations,
    })
    if err != nil {
        return err
    }
    
    _, err = s.provider.JobClient.Enqueue(task, 
        asynq.Queue("low"),
        asynq.MaxRetry(3),
    )
    return err
}
```

### 报表生成

```go
type ReportService struct {
    provider *provider.Provider
}

func (s *ReportService) GenerateAsync(ctx context.Context, req GenerateRequest) error {
    task, err := task.NewGenerateReportTask(task.GenerateReportPayload{
        ReportType: req.Type,
        StartDate:  req.StartDate,
        EndDate:    req.EndDate,
        UserID:     req.UserID,
    })
    if err != nil {
        return err
    }
    
    // 延迟到夜间执行
    nextNight := time.Now().Truncate(24*time.Hour).Add(20*time.Hour)
    if nextNight.Before(time.Now()) {
        nextNight = nextNight.Add(24 * time.Hour)
    }
    
    _, err = s.provider.JobClient.Enqueue(task, 
        asynq.ProcessAt(nextNight),
        asynq.Queue("low"),
    )
    return err
}
```

## 任务处理器

### 基础处理器

```go
func HandleTask(ctx context.Context, t *asynq.Task) error {
    // 解析负载
    var payload Payload
    if err := json.Unmarshal(t.Payload(), &payload); err != nil {
        return fmt.Errorf("unmarshal payload: %w", err)
    }
    
    // 获取日志记录器
    logger := reqctx.GetLogger(ctx)
    logger.Info().
        Str("task_id", t.ID()).
        Str("type", t.Type()).
        Msg("正在处理任务")
    
    // 执行业务逻辑
    if err := doWork(ctx, payload); err != nil {
        logger.Error().Err(err).Msg("任务执行失败")
        return err
    }
    
    logger.Info().Msg("任务执行完成")
    return nil
}
```

### 带进度报告

```go
func HandleLongRunningTask(ctx context.Context, t *asynq.Task) error {
    var payload Payload
    json.Unmarshal(t.Payload(), &payload)
    
    total := 100
    for i := 0; i < total; i++ {
        // 检查取消信号
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
        }
        
        // 执行工作
        doStep(i)
        
        // 报告进度
        asynq.GetTaskMetadata(ctx).SetProgress(i, total)
    }
    
    return nil
}
```

### 错误处理

```go
func HandleTaskWithRetry(ctx context.Context, t *asynq.Task) error {
    var payload Payload
    json.Unmarshal(t.Payload(), &payload)
    
    err := doWork(ctx, payload)
    if err != nil {
        // 检查是否应该重试
        if isRetryable(err) {
            return fmt.Errorf("retryable error: %w", err)
        }
        
        // 不可重试错误，记录并跳过
        logger.Error().Err(err).Msg("不可重试错误，跳过处理")
        return nil
    }
    
    return nil
}

func isRetryable(err error) bool {
    // 网络错误、超时等可以重试
    var netErr net.Error
    if errors.As(err, &netErr) {
        return true
    }
    
    // 特定错误码可以重试
    if errors.Is(err, ErrRateLimited) {
        return true
    }
    
    return false
}
```

## 任务监控

### Web UI

Asynq 提供内置的 Web UI：

```go
// 启动 Web UI
func StartMonitoringServer(addr string, inspector *asynq.Inspector) {
    http.HandleFunc("/", asynqmonitor.IndexHandler(inspector))
    http.HandleFunc("/queues", asynqmonitor.QueuesHandler(inspector))
    http.HandleFunc("/tasks", asynqmonitor.TasksHandler(inspector))
    
    log.Fatal(http.ListenAndServe(addr, nil))
}
```

访问 `http://localhost:8080` 查看：
- 队列状态
- 活跃任务
- 失败任务
- 任务统计

### 程序化管理

```go
inspector := asynq.NewInspector(asynq.RedisClientOpt{Addr: "localhost:6379"})

// 列出队列
queues, err := inspector.Queues()

// 获取任务列表
tasks, err := inspector.ListScheduledTasks("default")

// 删除任务
err := inspector.DeleteTask("default", taskID)

// 归档所有失败任务
err := inspector.ArchiveAllRetryTasks("default")

// 运行历史任务
info, err := inspector.RunAllArchivedTasks("default")
```

## 最佳实践

### 1. 任务粒度

```go
// ✅ 细粒度任务 - 单一职责
func HandleSendEmailTask(ctx context.Context, t *asynq.Task) error {
    // 只发送一封邮件
}

func HandleUpdateStatsTask(ctx context.Context, t *asynq.Task) error {
    // 只更新一项统计
}

// ❌ 避免粗粒度任务
func HandleEverythingTask(ctx context.Context, t *asynq.Task) error {
    // 发送邮件
    // 更新统计
    // 清理缓存
    // 生成报表
    // ... 太多操作
}
```

### 2. 幂等性

```go
// ✅ 幂等任务 - 可以安全地重复执行
func HandleChargeTask(ctx context.Context, t *asynq.Task) error {
    var payload ChargePayload
    json.Unmarshal(t.Payload(), &payload)
    
    // 检查是否已处理
    if alreadyProcessed(payload.OrderID) {
        return nil  // 已处理，直接返回
    }
    
    // 执行扣款
    err := charge(payload)
    if err != nil {
        return err
    }
    
    // 标记已处理
    markProcessed(payload.OrderID)
    
    return nil
}
```

### 3. 超时控制

```go
// ✅ 设置合理的超时
_, err := client.Enqueue(task, asynq.Timeout(30*time.Second))  // 快速任务
_, err := client.Enqueue(task, asynq.Timeout(5*time.Minute))   // 慢任务

// ✅ 在处理器中检查取消
func HandleTask(ctx context.Context, t *asynq.Task) error {
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()  // 任务被取消
        default:
        }
        
        // 执行工作
    }
}
```

### 4. 错误处理

```go
// ✅ 区分可重试和不可重试错误
func HandleTask(ctx context.Context, t *asynq.Task) error {
    err := doWork()
    
    if errors.Is(err, ErrValidation) {
        // 验证错误，不需要重试
        logger.Error().Err(err).Msg("校验失败，跳过处理")
        return nil
    }
    
    if errors.Is(err, ErrNotFound) {
        // 资源不存在，不需要重试
        return nil
    }
    
    // 其他错误，允许重试
    return err
}
```

### 5. 日志记录

```go
func HandleTask(ctx context.Context, t *asynq.Task) error {
    logger := reqctx.GetLogger(ctx).With().
        Str("task_id", t.ID()).
        Str("task_type", t.Type()).
        Int("max_retry", t.MaxRetry()).
        Logger()
    
    logger.Info().Msg("任务开始执行")
    
    start := time.Now()
    err := doWork(ctx)
    duration := time.Since(start)
    
    if err != nil {
        logger.Error().
            Err(err).
            Dur("duration", duration).
            Msg("任务执行失败")
        return err
    }
    
    logger.Info().
        Dur("duration", duration).
        Msg("任务执行完成")
    
    return nil
}
```

## 常见问题

### Q: 任务没有执行？

检查：
1. Worker 是否已启动
2. 任务是否正确入队
3. Redis 连接是否正常
4. 队列名称是否正确

### Q: 任务执行失败？

```go
// 查看失败任务
inspector := asynq.NewInspector(...)
failed, _ := inspector.ListFailedTasks("default")

for _, task := range failed {
    fmt.Printf("Failed task: %s, Error: %s\n", task.ID, task.Error)
}
```

### Q: 如何清空队列？

```go
inspector := asynq.NewInspector(...)

// 清空特定队列
err := inspector.DeleteAllScheduledTasks("default")
err = inspector.DeleteAllRetryTasks("default")

// 清空所有队列
queues, _ := inspector.Queues()
for _, q := range queues {
    inspector.DeleteAllScheduledTasks(q)
    inspector.DeleteAllRetryTasks(q)
}
```

### Q: 任务堆积怎么办？

1. 增加 Worker 数量
2. 优化任务处理速度
3. 使用更高权重的队列
4. 考虑任务拆分

```go
// 增加并发
job:
  concurrency: 50  // 增加并发数

// 增加队列权重
queues:
  critical: 10
  default: 3
  low: 1
```
