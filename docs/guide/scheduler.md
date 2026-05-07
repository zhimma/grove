# 计划任务

本文档介绍 grove 框架的计划任务系统，基于 robfig/cron 封装。

## 特性

- ⏰ **Cron 表达式** - 标准 Unix cron 格式
- 🔄 **多种频率** - 支持秒级、分钟级、小时级等多种频率
- 📝 **任务管理** - 支持添加、删除、暂停任务
- 🎯 **上下文支持** - 任务接收 context.Context
- 📊 **执行日志** - 自动记录任务执行
- 🛑 **优雅关闭** - 支持等待任务完成

## 配置

```yaml
scheduler:
  enabled: true                    # 是否启用
  timezone: Asia/Shanghai          # 时区
```

## 快速开始

### 注册任务

```go
// 在 Provider 初始化后注册任务
func RegisterSchedulers(p *provider.Provider) {
    // 每分钟执行
    p.Scheduler.EveryMinute("cleanup", scheduler.JobFunc(func(ctx context.Context) error {
        return cleanupExpiredData(ctx)
    }))
    
    // 每天凌晨执行
    p.Scheduler.Daily("report", scheduler.JobFunc(func(ctx context.Context) error {
        return generateDailyReport(ctx)
    }))
}
```

### 启动调度器

```go
// 在 main.go 中启动
func main() {
    p, cleanup, err := server.NewCoreServer(cfg, "console", port, opts...)
    if err != nil {
        log.Fatal(err)
    }
    defer cleanup()
    
    // 注册计划任务
    RegisterSchedulers(p)
    
    // 启动调度器
    if cfg.Scheduler.Enabled {
        p.Scheduler.Start()
    }
    
    // ... 其他代码
}
```

## 详细用法

### 频率方法

```go
// 每秒执行
p.Scheduler.EverySecond("task", job)

// 每 N 秒执行
p.Scheduler.EveryNSeconds("task", 30, job)  // 每 30 秒

// 每分钟执行
p.Scheduler.EveryMinute("task", job)

// 每 N 分钟执行
p.Scheduler.EveryNMinutes("task", 5, job)  // 每 5 分钟

// 每小时执行
p.Scheduler.Hourly("task", job)

// 每 N 小时执行
p.Scheduler.EveryNHours("task", 2, job)  // 每 2 小时

// 每天执行（凌晨 0:00）
p.Scheduler.Daily("task", job)

// 每天指定时间执行
p.Scheduler.DailyAt("task", "14:30", job)  // 每天 14:30

// 每周执行（周日 0:00）
p.Scheduler.Weekly("task", job)

// 每月执行（1 号 0:00）
p.Scheduler.Monthly("task", job)
```

### Cron 表达式

```go
// 使用标准 cron 表达式
// 格式：秒 分 时 日 月 周

// 每分钟执行
p.Scheduler.Cron("task", "0 * * * * *", job)

// 每天凌晨 2:30 执行
p.Scheduler.Cron("task", "0 30 2 * * *", job)

// 每周一早上 9:00 执行
p.Scheduler.Cron("task", "0 0 9 * * 1", job)

// 每月 1 号 0:00 执行
p.Scheduler.Cron("task", "0 0 0 1 * *", job)

// 每 5 分钟执行
p.Scheduler.Cron("task", "0 */5 * * * *", job)

// 工作日每小时的第 0 和第 30 分钟执行
p.Scheduler.Cron("task", "0 0,30 * * * 1-5", job)
```

Cron 表达式格式说明：

| 字段 | 含义 | 范围 |
|------|------|------|
| 秒 | Seconds | 0-59 |
| 分 | Minutes | 0-59 |
| 时 | Hours | 0-23 |
| 日 | Day of month | 1-31 |
| 月 | Month | 1-12 |
| 周 | Day of week | 0-6 (0=周日) |

特殊字符：
- `*` - 任意值
- `,` - 列表（如 `1,3,5`）
- `-` - 范围（如 `1-5`）
- `/` - 步长（如 `*/5`）

### 一次性任务

```go
// 延迟执行
p.Scheduler.ScheduleOnce("delayed_task", time.Now().Add(5*time.Minute), scheduler.JobFunc(func(ctx context.Context) error {
    // 5 分钟后执行
    return doSomething(ctx)
}))

// 指定时间执行
nextRun := time.Date(2024, 1, 1, 0, 0, 0, 0, time.Local)
p.Scheduler.ScheduleOnce("new_year", nextRun, scheduler.JobFunc(func(ctx context.Context) error {
    return sendNewYearGreetings(ctx)
}))
```

### 任务定义

```go
// 使用 JobFunc 包装函数
type MyJob struct {
    provider *provider.Provider
}

func (j *MyJob) Run(ctx context.Context) error {
    // 获取日志记录器
    logger := reqctx.GetLogger(ctx)
    logger.Info().Msg("任务开始执行")
    
    // 执行业务逻辑
    result, err := j.processData(ctx)
    if err != nil {
        logger.Error().Err(err).Msg("任务执行失败")
        return err
    }
    
    logger.Info().Interface("result", result).Msg("任务执行完成")
    return nil
}

// 注册
p.Scheduler.EveryMinute("my_job", &MyJob{provider: p})
```

## 任务管理

### 停止任务

```go
// 停止单个任务
p.Scheduler.StopJob("cleanup")

// 停止所有任务
p.Scheduler.Stop()
```

### 任务状态

```go
// 获取任务列表
tasks := p.Scheduler.Tasks()
for _, task := range tasks {
    fmt.Printf("Task: %s, Next: %s\n", task.Name, task.NextRun)
}

// 检查任务是否存在
if p.Scheduler.Has("cleanup") {
    // ...
}
```

## 在 Service 中使用

### 定期清理

```go
type CleanupService struct {
    provider *provider.Provider
}

func (s *CleanupService) Register(scheduler *scheduler.Scheduler) {
    // 每天凌晨清理过期数据
    scheduler.Daily("cleanup_expired", scheduler.JobFunc(func(ctx context.Context) error {
        return s.cleanupExpiredData(ctx)
    }))
    
    // 每小时清理临时文件
    scheduler.EveryNHours("cleanup_temp", 1, scheduler.JobFunc(func(ctx context.Context) error {
        return s.cleanupTempFiles(ctx)
    }))
}

func (s *CleanupService) cleanupExpiredData(ctx context.Context) error {
    // 清理过期日志
    result := s.provider.DB.Default().
        Where("created_at < ?", time.Now().AddDate(0, -3, 0)).
        Delete(&model.OperationLog{})
    
    if result.Error != nil {
        return result.Error
    }
    
    logger.Info().Int64("rows", result.RowsAffected).Msg("过期日志已清理")
    return nil
}
```

### 定期统计

```go
func (s *StatsService) Register(scheduler *scheduler.Scheduler) {
    // 每小时更新统计
    scheduler.EveryNMinutes("update_stats", 10, scheduler.JobFunc(func(ctx context.Context) error {
        return s.updateRealtimeStats(ctx)
    }))
    
    // 每天生成报表
    scheduler.DailyAt("daily_report", "01:00", scheduler.JobFunc(func(ctx context.Context) error {
        return s.generateDailyReport(ctx)
    }))
    
    // 每周生成周报
    scheduler.Cron("weekly_report", "0 0 9 * * 1", scheduler.JobFunc(func(ctx context.Context) error {
        return s.generateWeeklyReport(ctx)
    }))
}
```

### 数据同步

```go
func (s *SyncService) Register(scheduler *scheduler.Scheduler) {
    // 每 5 分钟同步订单状态
    scheduler.EveryNMinutes("sync_orders", 5, scheduler.JobFunc(func(ctx context.Context) error {
        return s.syncOrderStatus(ctx)
    }))
    
    // 每小时同步库存
    scheduler.EveryNMinutes("sync_inventory", 60, scheduler.JobFunc(func(ctx context.Context) error {
        return s.syncInventory(ctx)
    }))
}
```

## 常见任务模式

### 分布式锁（防止重复执行）

```go
func (s *Service) Register(scheduler *scheduler.Scheduler) {
    scheduler.Daily("distributed_task", scheduler.JobFunc(func(ctx context.Context) error {
        // 获取分布式锁
        lockKey := "scheduler:distributed_task"
        locked, err := s.provider.RedisClient.SetNX(ctx, lockKey, "1", 5*time.Minute).Result()
        if err != nil || !locked {
            logger.Info().Msg("任务已在其他实例运行")
            return nil
        }
        
        // 确保释放锁
        defer s.provider.RedisClient.Del(ctx, lockKey)
        
        // 执行任务
        return s.doWork(ctx)
    }))
}
```

### 任务重试

```go
func (s *Service) Register(scheduler *scheduler.Scheduler) {
    scheduler.EveryNMinutes("retry_task", 5, scheduler.JobFunc(func(ctx context.Context) error {
        // 获取失败的任务
        var failedTasks []model.FailedTask
        s.provider.DB.Default().
            Where("retry_count < ?", 3).
            Where("next_retry_at < ?", time.Now()).
            Find(&failedTasks)
        
        for _, task := range failedTasks {
            err := s.retryTask(ctx, task)
            if err != nil {
                task.RetryCount++
                task.NextRetryAt = time.Now().Add(time.Minute * time.Duration(task.RetryCount*5))
                s.provider.DB.Default().Save(&task)
            } else {
                s.provider.DB.Default().Delete(&task)
            }
        }
        
        return nil
    }))
}
```

### 任务依赖

```go
func (s *Service) Register(scheduler *scheduler.Scheduler) {
    // 任务 A
    scheduler.Daily("task_a", scheduler.JobFunc(func(ctx context.Context) error {
        err := s.doTaskA(ctx)
        if err != nil {
            return err
        }
        
        // 任务 A 成功后立即触发任务 B
        go s.provider.Scheduler.ScheduleOnce("task_b_trigger", time.Now(), scheduler.JobFunc(func(ctx context.Context) error {
            return s.doTaskB(ctx)
        }))
        
        return nil
    }))
}
```

## 监控与调试

### 任务日志

```go
func (s *Service) Register(scheduler *scheduler.Scheduler) {
    scheduler.EveryMinute("logged_task", scheduler.JobFunc(func(ctx context.Context) error {
        logger := reqctx.GetLogger(ctx)
        
        logger.Info().Msg("任务开始执行")
        
        start := time.Now()
        err := s.doWork(ctx)
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
    }))
}
```

### 执行统计

```go
// 记录任务执行历史
type TaskExecution struct {
    TaskName  string
    StartAt   time.Time
    EndAt     time.Time
    Duration  time.Duration
    Error     string
    Success   bool
}

func (s *Service) Register(scheduler *scheduler.Scheduler) {
    scheduler.EveryMinute("monitored_task", scheduler.JobFunc(func(ctx context.Context) error {
        exec := &TaskExecution{
            TaskName: "monitored_task",
            StartAt:  time.Now(),
        }
        
        err := s.doWork(ctx)
        
        exec.EndAt = time.Now()
        exec.Duration = exec.EndAt.Sub(exec.StartAt)
        exec.Success = err == nil
        if err != nil {
            exec.Error = err.Error()
        }
        
        // 保存执行记录
        s.provider.DB.Default().Create(exec)
        
        return err
    }))
}
```

## 最佳实践

### 1. 任务命名

```go
// ✅ 使用描述性名称
p.Scheduler.Daily("cleanup_expired_logs", job)
p.Scheduler.EveryNMinutes("sync_order_status", 5, job)

// ❌ 避免模糊名称
p.Scheduler.Daily("task1", job)
p.Scheduler.EveryNMinutes("job", 5, job)
```

### 2. 错误处理

```go
// ✅ 记录错误但不中断调度
scheduler.JobFunc(func(ctx context.Context) error {
    if err := doWork(ctx); err != nil {
        logger.Error().Err(err).Msg("任务执行失败，将在下次重试")
        return nil  // 返回 nil 不中断调度
    }
    return nil
})

// ✅ 或者返回错误让框架记录
scheduler.JobFunc(func(ctx context.Context) error {
    return doWork(ctx)  // 返回错误会被记录
})
```

### 3. 超时控制

```go
// ✅ 使用 context 控制超时
scheduler.JobFunc(func(ctx context.Context) error {
    // 创建带超时的子上下文
    ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
    defer cancel()
    
    return doWork(ctx)
})
```

### 4. 避免重叠执行

```go
// ✅ 使用互斥锁防止重叠
var mu sync.Mutex

scheduler.EveryMinute("non_overlapping", scheduler.JobFunc(func(ctx context.Context) error {
    if !mu.TryLock() {
        logger.Warn().Msg("上一次任务仍在运行，跳过本次执行")
        return nil
    }
    defer mu.Unlock()
    
    return doWork(ctx)
}))
```

### 5. 优雅关闭

```go
func main() {
    // ...
    
    // 等待中断信号
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit
    
    logger.Info().Msg("正在关闭")
    
    // 停止调度器，等待正在执行的任务完成
    p.Scheduler.Stop()
    
    // 关闭其他资源
    cleanup()
}
```

## 常见问题

### Q: 任务没有执行？

检查：
1. Scheduler 是否已启动 (`p.Scheduler.Start()`)
2. 配置中 `scheduler.enabled` 是否为 `true`
3. 时区设置是否正确
4. Cron 表达式是否正确

### Q: 如何查看任务列表？

```go
tasks := p.Scheduler.Tasks()
for _, task := range tasks {
    fmt.Printf("%s: %s\n", task.Name, task.Schedule)
}
```

### Q: 任务执行时间过长？

- 使用 `context.WithTimeout` 设置超时
- 考虑将任务拆分为多个小任务
- 使用分布式锁防止重复执行

### Q: 集群环境下如何避免重复执行？

使用分布式锁：

```go
scheduler.JobFunc(func(ctx context.Context) error {
    lock := "scheduler:" + taskName
    if !acquireLock(ctx, lock, 5*time.Minute) {
        return nil  // 其他实例正在执行
    }
    defer releaseLock(ctx, lock)
    
    return doWork(ctx)
})
```
