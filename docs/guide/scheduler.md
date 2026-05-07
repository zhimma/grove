# 计划任务

本文档说明 Grove 中计划任务组件的使用方式。

## 适用范围

`pkg/scheduler` 适用于单进程定时任务，例如：

- 定时清理
- 定时报表
- 定时同步

## 最短路径

### 启用配置

```yaml
scheduler:
  enabled: true
  timezone: Asia/Shanghai
```

### 注册任务

```go
err := p.Scheduler.EveryMinute("sync_stats", scheduler.JobFunc(func(ctx context.Context) error {
	return syncStats(ctx)
}))
if err != nil {
	return err
}
```

### 启动任务调度

在服务启动阶段完成任务注册并启动调度器。

## 使用约定

- 任务名必须唯一。
- 定时任务应短小、幂等。
- 需要防止同进程重入时，应使用组件提供的互斥能力。

## 边界

- 多实例部署下的全局互斥不由调度器隐式保证。
- 跨实例去重或分布式锁应由 Redis、数据库或外部协调系统负责。

## 相关文档

- [队列任务](./queue.md)
- [pkg 基础组件](./pkg-components.md)
