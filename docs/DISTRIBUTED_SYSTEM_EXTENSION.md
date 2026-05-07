# 分布式系统扩展方案

## 当前架构分析

### 现状
- 单体应用架构（Monolithic）
- 多应用支持（api/console/worker）
- 共享数据库和 Redis
- 基础组件完善但缺乏分布式能力

### 分布式扩展需求
1. **水平扩展** - 多实例部署
2. **服务发现** - 动态服务注册与发现
3. **配置中心** - 集中式配置管理
4. **分布式锁** - 跨实例互斥控制
5. **分布式事务** - Saga/TCC 模式
6. **链路追踪** - 请求全链路追踪
7. **限流熔断** - 服务保护机制

---

## 1. 服务注册与发现

### 1.1 方案选型

| 方案 | 优点 | 缺点 | 适用场景 |
|------|------|------|----------|
| Consul | 功能全面，健康检查 | 部署复杂 | 大规模集群 |
| etcd | Kubernetes 原生 | 功能单一 | K8s 环境 |
| Nacos | 配置+注册一体 | 阿里生态 | 国内环境 |
| **推荐：Consul** | | | |

### 1.2 实现代码

```go
// pkg/discovery/consul.go
package discovery

import (
    "context"
    "fmt"
    "time"
    
    "github.com/hashicorp/consul/api"
)

type ConsulClient struct {
    client    *api.Client
    serviceID string
    ttl       time.Duration
    stopCh    chan struct{}
}

func NewConsulClient(addr string) (*ConsulClient, error) {
    config := api.DefaultConfig()
    config.Address = addr
    
    client, err := api.NewClient(config)
    if err != nil {
        return nil, err
    }
    
    return &ConsulClient{
        client: client,
        ttl:    10 * time.Second,
        stopCh: make(chan struct{}),
    }, nil
}

// Register 注册服务
func (c *ConsulClient) Register(serviceName, host string, port int, tags []string) error {
    c.serviceID = fmt.Sprintf("%s-%s-%d", serviceName, host, port)
    
    registration := &api.AgentServiceRegistration{
        ID:      c.serviceID,
        Name:    serviceName,
        Address: host,
        Port:    port,
        Tags:    tags,
        Check: &api.AgentServiceCheck{
            TTL:                            c.ttl.String(),
            Status:                         api.HealthPassing,
            DeregisterCriticalServiceAfter: "1m",
        },
    }
    
    if err := c.client.Agent().ServiceRegister(registration); err != nil {
        return err
    }
    
    // 启动心跳
    go c.heartbeat()
    
    return nil
}

// Deregister 注销服务
func (c *ConsulClient) Deregister() error {
    close(c.stopCh)
    return c.client.Agent().ServiceDeregister(c.serviceID)
}

// heartbeat 发送心跳
func (c *ConsulClient) heartbeat() {
    ticker := time.NewTicker(c.ttl / 2)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            c.client.Agent().UpdateTTL("service:"+c.serviceID, "", api.HealthPassing)
        case <-c.stopCh:
            return
        }
    }
}

// Discover 服务发现
func (c *ConsulClient) Discover(serviceName string) ([]*api.ServiceEntry, error) {
    entries, _, err := c.client.Health().Service(serviceName, "", true, nil)
    return entries, err
}

// Watch 监听服务变化
func (c *ConsulClient) Watch(serviceName string, handler func([]*api.ServiceEntry)) {
    plan, _ := api.Parse(``)
    plan.Handler = func(idx uint64, raw interface{}) {
        if entries, ok := raw.([]*api.ServiceEntry); ok {
            handler(entries)
        }
    }
    plan.Run(c.client.Address())
}
```

### 1.3 集成到 Provider

```go
// internal/provider/provider.go

type Provider struct {
    // ... 现有字段
    
    // 服务发现
    Discovery discovery.Client
    
    // 负载均衡
    LoadBalancer loadbalancer.Balancer
}

func WithDiscovery(addr string) Option {
    return func(p *Provider) error {
        client, err := discovery.NewConsulClient(addr)
        if err != nil {
            return err
        }
        
        // 注册当前服务
        hostname, _ := os.Hostname()
        client.Register(
            p.Config.App.Name,
            hostname,
            p.Config.Port,
            []string{p.Config.App.Env, "v1"},
        )
        
        p.Discovery = client
        p.LoadBalancer = loadbalancer.NewRoundRobin()
        return nil
    }
}
```

---

## 2. 分布式配置中心

### 2.1 方案选型

| 方案 | 优点 | 缺点 | 推荐 |
|------|------|------|------|
| Apollo | 功能完善，UI友好 | 部署较重 | ⭐⭐⭐⭐ |
| Nacos | 配置+注册一体 | 阿里生态 | ⭐⭐⭐⭐ |
| etcd | K8s原生 | 无UI | ⭐⭐⭐ |
| Consul KV | 一体化 | 功能简单 | ⭐⭐⭐ |

### 2.2 实现代码（基于 Nacos）

```go
// pkg/configcenter/nacos.go
package configcenter

import (
    "context"
    "encoding/json"
    "sync"
    
    "github.com/nacos-group/nacos-sdk-go/v2/clients"
    "github.com/nacos-group/nacos-sdk-go/v2/clients/config_client"
    "github.com/nacos-group/nacos-sdk-go/v2/common/constant"
    "github.com/nacos-group/nacos-sdk-go/v2/vo"
)

type NacosConfigCenter struct {
    client      config_client.IConfigClient
    dataID      string
    group       string
    cache       map[string]interface{}
    cacheMu     sync.RWMutex
    listeners   map[string][]func(string)
    listenerMu  sync.RWMutex
}

func NewNacosConfigCenter(serverAddr, namespace, dataID, group string) (*NacosConfigCenter, error) {
    clientConfig := constant.ClientConfig{
        NamespaceId:         namespace,
        TimeoutMs:           5000,
        NotLoadCacheAtStart: true,
        LogDir:              "/tmp/nacos/log",
        CacheDir:            "/tmp/nacos/cache",
        LogLevel:            "warn",
    }
    
    serverConfigs := []constant.ServerConfig{
        {
            IpAddr: serverAddr,
            Port:   8848,
        },
    }
    
    client, err := clients.NewConfigClient(
        vo.NacosClientParam{
            ClientConfig:  &clientConfig,
            ServerConfigs: serverConfigs,
        },
    )
    if err != nil {
        return nil, err
    }
    
    cc := &NacosConfigCenter{
        client:    client,
        dataID:    dataID,
        group:     group,
        cache:     make(map[string]interface{}),
        listeners: make(map[string][]func(string)),
    }
    
    // 初始加载
    if err := cc.load(); err != nil {
        return nil, err
    }
    
    // 监听配置变更
    cc.watch()
    
    return cc, nil
}

func (c *NacosConfigCenter) load() error {
    content, err := c.client.GetConfig(vo.ConfigParam{
        DataId: c.dataID,
        Group:  c.group,
    })
    if err != nil {
        return err
    }
    
    return c.parse(content)
}

func (c *NacosConfigCenter) parse(content string) error {
    var config map[string]interface{}
    if err := json.Unmarshal([]byte(content), &config); err != nil {
        return err
    }
    
    c.cacheMu.Lock()
    c.cache = config
    c.cacheMu.Unlock()
    
    return nil
}

func (c *NacosConfigCenter) watch() {
    c.client.ListenConfig(vo.ConfigParam{
        DataId: c.dataID,
        Group:  c.group,
        OnChange: func(namespace, group, dataId, data string) {
            c.parse(data)
            c.notify(data)
        },
    })
}

func (c *NacosConfigCenter) Get(key string) (interface{}, bool) {
    c.cacheMu.RLock()
    defer c.cacheMu.RUnlock()
    
    val, ok := c.cache[key]
    return val, ok
}

func (c *NacosConfigCenter) GetString(key string) string {
    val, ok := c.Get(key)
    if !ok {
        return ""
    }
    
    if str, ok := val.(string); ok {
        return str
    }
    return ""
}

func (c *NacosConfigCenter) GetInt(key string) int {
    val, ok := c.Get(key)
    if !ok {
        return 0
    }
    
    switch v := val.(type) {
    case int:
        return v
    case float64:
        return int(v)
    default:
        return 0
    }
}

func (c *NacosConfigCenter) OnChange(key string, handler func(string)) {
    c.listenerMu.Lock()
    defer c.listenerMu.Unlock()
    
    c.listeners[key] = append(c.listeners[key], handler)
}

func (c *NacosConfigCenter) notify(data string) {
    c.listenerMu.RLock()
    defer c.listenerMu.RUnlock()
    
    for _, handlers := range c.listeners {
        for _, handler := range handlers {
            go handler(data)
        }
    }
}
```

---

## 3. 分布式锁

### 3.1 方案选型

| 方案 | 优点 | 缺点 | 适用场景 |
|------|------|------|----------|
| Redis Redlock | 性能高 | 时钟漂移问题 | 高并发 |
| etcd | 强一致性 | 性能较低 | 强一致性要求 |
| Zookeeper | 成熟稳定 | 部署复杂 | 传统系统 |
| **推荐：Redis Redlock** | | | |

### 3.2 实现代码

```go
// pkg/distributedlock/redis_lock.go
package distributedlock

import (
    "context"
    "crypto/rand"
    "encoding/base64"
    "fmt"
    "sync"
    "time"
    
    "github.com/redis/go-redis/v9"
)

const (
    defaultTimeout = 30 * time.Second
    retryDelay     = 100 * time.Millisecond
)

type RedisLock struct {
    client    *redis.Client
    key       string
    value     string
    timeout   time.Duration
    cancel    context.CancelFunc
    wg        sync.WaitGroup
}

func NewRedisLock(client *redis.Client, key string) *RedisLock {
    return &RedisLock{
        client:  client,
        key:     fmt.Sprintf("lock:%s", key),
        timeout: defaultTimeout,
    }
}

func (l *RedisLock) WithTimeout(timeout time.Duration) *RedisLock {
    l.timeout = timeout
    return l
}

// Lock 获取锁
func (l *RedisLock) Lock(ctx context.Context) error {
    // 生成唯一值
    l.value = l.generateValue()
    
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
        }
        
        // 尝试获取锁
        ok, err := l.client.SetNX(ctx, l.key, l.value, l.timeout).Result()
        if err != nil {
            return err
        }
        
        if ok {
            // 获取成功，启动续期
            l.startRenewal()
            return nil
        }
        
        // 获取失败，等待重试
        time.Sleep(retryDelay)
    }
}

// TryLock 尝试获取锁（非阻塞）
func (l *RedisLock) TryLock(ctx context.Context) (bool, error) {
    l.value = l.generateValue()
    
    ok, err := l.client.SetNX(ctx, l.key, l.value, l.timeout).Result()
    if err != nil {
        return false, err
    }
    
    if ok {
        l.startRenewal()
    }
    
    return ok, nil
}

// Unlock 释放锁
func (l *RedisLock) Unlock(ctx context.Context) error {
    // 停止续期
    if l.cancel != nil {
        l.cancel()
        l.wg.Wait()
    }
    
    // 使用 Lua 脚本确保原子性
    script := `
        if redis.call("get", KEYS[1]) == ARGV[1] then
            return redis.call("del", KEYS[1])
        else
            return 0
        end
    `
    
    result, err := l.client.Eval(ctx, script, []string{l.key}, l.value).Result()
    if err != nil {
        return err
    }
    
    if result.(int64) == 0 {
        return fmt.Errorf("lock not held or value mismatch")
    }
    
    return nil
}

// startRenewal 启动自动续期
func (l *RedisLock) startRenewal() {
    ctx, cancel := context.WithCancel(context.Background())
    l.cancel = cancel
    
    l.wg.Add(1)
    go func() {
        defer l.wg.Done()
        
        ticker := time.NewTicker(l.timeout / 3)
        defer ticker.Stop()
        
        for {
            select {
            case <-ticker.C:
                // 续期
                script := `
                    if redis.call("get", KEYS[1]) == ARGV[1] then
                        return redis.call("pexpire", KEYS[1], ARGV[2])
                    else
                        return 0
                    end
                `
                l.client.Eval(ctx, script, []string{l.key}, l.value, l.timeout.Milliseconds())
            case <-ctx.Done():
                return
            }
        }
    }()
}

func (l *RedisLock) generateValue() string {
    b := make([]byte, 16)
    rand.Read(b)
    return base64.StdEncoding.EncodeToString(b)
}

// LockManager 锁管理器
type LockManager struct {
    client *redis.Client
}

func NewLockManager(client *redis.Client) *LockManager {
    return &LockManager{client: client}
}

func (m *LockManager) NewLock(key string) *RedisLock {
    return NewRedisLock(m.client, key)
}
```

### 3.3 使用示例

```go
// 在 Service 中使用分布式锁
func (s *OrderService) CreateWithLock(ctx context.Context, req CreateRequest) (*Order, error) {
    // 获取分布式锁（防止重复创建）
    lock := s.provider.LockManager.NewLock(fmt.Sprintf("order:create:%s", req.OrderNo))
    
    if err := lock.Lock(ctx); err != nil {
        return nil, fmt.Errorf("获取锁失败: %w", err)
    }
    defer lock.Unlock(ctx)
    
    // 检查订单是否已存在
    var existing Order
    if err := s.provider.DB.Default().Where("order_no = ?", req.OrderNo).First(&existing).Error; err == nil {
        return nil, errors.NewConflict("订单已存在")
    }
    
    // 创建订单
    order := &Order{...}
    if err := s.provider.DB.Default().Create(order).Error; err != nil {
        return nil, err
    }
    
    return order, nil
}
```

---

## 4. 分布式事务

### 4.1 方案选型

| 方案 | 优点 | 缺点 | 适用场景 |
|------|------|------|----------|
| Saga | 最终一致，性能好 | 补偿复杂 | 长事务 |
| TCC | 强一致 | 业务侵入大 | 金融场景 |
| 2PC | 强一致 | 性能差，阻塞 | 传统系统 |
| **推荐：Saga + 本地消息表** | | | |

### 4.2 Saga 实现

```go
// pkg/saga/saga.go
package saga

import (
    "context"
    "encoding/json"
    "fmt"
    "time"
)

// Saga 分布式事务
type Saga struct {
    id        string
    steps     []Step
    current   int
    state     State
    logStore  LogStore
}

type State int

const (
    StatePending State = iota
    StateRunning
    StateCompleted
    StateFailed
    StateCompensating
    StateCompensated
)

type Step struct {
    Name       string
    Action     func(context.Context) error
    Compensate func(context.Context) error
    Data       map[string]interface{}
}

// New 创建 Saga
func New(id string, logStore LogStore) *Saga {
    return &Saga{
        id:       id,
        logStore: logStore,
        state:    StatePending,
    }
}

// AddStep 添加步骤
func (s *Saga) AddStep(name string, action, compensate func(context.Context) error) *Saga {
    s.steps = append(s.steps, Step{
        Name:       name,
        Action:     action,
        Compensate: compensate,
    })
    return s
}

// Execute 执行 Saga
func (s *Saga) Execute(ctx context.Context) error {
    s.state = StateRunning
    
    // 记录开始
    s.logStore.Append(s.id, LogEntry{
        Timestamp: time.Now(),
        State:     StateRunning,
    })
    
    for i, step := range s.steps {
        s.current = i
        
        // 记录步骤开始
        s.logStore.Append(s.id, LogEntry{
            Timestamp: time.Now(),
            Step:      step.Name,
            State:     StateRunning,
        })
        
        // 执行步骤
        if err := step.Action(ctx); err != nil {
            // 记录失败
            s.logStore.Append(s.id, LogEntry{
                Timestamp: time.Now(),
                Step:      step.Name,
                State:     StateFailed,
                Error:     err.Error(),
            })
            
            // 触发补偿
            return s.compensate(ctx, i)
        }
        
        // 记录成功
        s.logStore.Append(s.id, LogEntry{
            Timestamp: time.Now(),
            Step:      step.Name,
            State:     StateCompleted,
        })
    }
    
    s.state = StateCompleted
    s.logStore.Append(s.id, LogEntry{
        Timestamp: time.Now(),
        State:     StateCompleted,
    })
    
    return nil
}

// compensate 执行补偿
func (s *Saga) compensate(ctx context.Context, failedIndex int) error {
    s.state = StateCompensating
    
    // 逆向补偿
    for i := failedIndex; i >= 0; i-- {
        step := s.steps[i]
        
        if step.Compensate != nil {
            s.logStore.Append(s.id, LogEntry{
                Timestamp: time.Now(),
                Step:      step.Name,
                State:     StateCompensating,
            })
            
            if err := step.Compensate(ctx); err != nil {
                // 补偿失败，需要人工介入
                s.logStore.Append(s.id, LogEntry{
                    Timestamp: time.Now(),
                    Step:      step.Name,
                    State:     StateFailed,
                    Error:     fmt.Sprintf("compensate failed: %v", err),
                })
                return fmt.Errorf("compensate failed at step %s: %w", step.Name, err)
            }
        }
    }
    
    s.state = StateCompensated
    s.logStore.Append(s.id, LogEntry{
        Timestamp: time.Now(),
        State:     StateCompensated,
    })
    
    return fmt.Errorf("saga failed, compensated")
}

// LogStore 日志存储接口
type LogStore interface {
    Append(sagaID string, entry LogEntry) error
    Get(sagaID string) ([]LogEntry, error)
}

type LogEntry struct {
    Timestamp time.Time
    Step      string
    State     State
    Error     string
}
```

### 4.3 使用示例

```go
// 订单创建 Saga
func (s *OrderService) CreateOrderSaga(ctx context.Context, req CreateRequest) (*Order, error) {
    var order *Order
    var payment *Payment
    var inventoryDeducted bool
    
    sagaInstance := saga.New(
        fmt.Sprintf("order-%s", req.OrderNo),
        s.provider.SagaLogStore,
    ).
    AddStep(
        "create_order",
        func(ctx context.Context) error {
            order = &Order{...}
            return s.provider.DB.Default().Create(order).Error
        },
        func(ctx context.Context) error {
            return s.provider.DB.Default().Delete(order).Error
        },
    ).
    AddStep(
        "deduct_inventory",
        func(ctx context.Context) error {
            inventoryDeducted = true
            return s.inventoryService.Deduct(ctx, req.ProductID, req.Quantity)
        },
        func(ctx context.Context) error {
            if inventoryDeducted {
                return s.inventoryService.Restore(ctx, req.ProductID, req.Quantity)
            }
            return nil
        },
    ).
    AddStep(
        "create_payment",
        func(ctx context.Context) error {
            payment = &Payment{...}
            return s.paymentService.Create(ctx, payment)
        },
        func(ctx context.Context) error {
            if payment != nil {
                return s.paymentService.Cancel(ctx, payment.ID)
            }
            return nil
        },
    )
    
    if err := sagaInstance.Execute(ctx); err != nil {
        return nil, err
    }
    
    return order, nil
}
```

---

## 5. 链路追踪

### 5.1 方案选型

| 方案 | 优点 | 缺点 | 推荐 |
|------|------|------|------|
| Jaeger | CNCF项目，功能完善 | 资源占用较高 | ⭐⭐⭐⭐ |
| Zipkin | 轻量级 | 功能较少 | ⭐⭐⭐ |
| SkyWalking | 国产，功能全面 | 学习曲线陡峭 | ⭐⭐⭐⭐ |
| **推荐：Jaeger** | | | |

### 5.2 OpenTelemetry 实现

```go
// pkg/trace/trace.go
package trace

import (
    "context"
    
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/exporters/jaeger"
    "go.opentelemetry.io/otel/sdk/resource"
    sdktrace "go.opentelemetry.io/otel/sdk/trace"
    semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
    "go.opentelemetry.io/otel/trace"
)

var tracer trace.Tracer

func InitTracer(serviceName, jaegerEndpoint string) (*sdktrace.TracerProvider, error) {
    // 创建 Jaeger 导出器
    exp, err := jaeger.New(jaeger.WithCollectorEndpoint(
        jaeger.WithEndpoint(jaegerEndpoint),
    ))
    if err != nil {
        return nil, err
    }
    
    // 创建资源
    res, err := resource.Merge(
        resource.Default(),
        resource.NewWithAttributes(
            semconv.SchemaURL,
            semconv.ServiceName(serviceName),
        ),
    )
    if err != nil {
        return nil, err
    }
    
    // 创建 TracerProvider
    tp := sdktrace.NewTracerProvider(
        sdktrace.WithBatcher(exp),
        sdktrace.WithResource(res),
        sdktrace.WithSampler(sdktrace.AlwaysSample()),
    )
    
    otel.SetTracerProvider(tp)
    tracer = tp.Tracer(serviceName)
    
    return tp, nil
}

// StartSpan 开始 Span
func StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
    return tracer.Start(ctx, name, opts...)
}

// SpanFromContext 从上下文获取 Span
func SpanFromContext(ctx context.Context) trace.Span {
    return trace.SpanFromContext(ctx)
}

// AddEvent 添加事件
func AddEvent(ctx context.Context, name string, attrs ...attribute.KeyValue) {
    span := trace.SpanFromContext(ctx)
    span.AddEvent(name, trace.WithAttributes(attrs...))
}

// SetAttributes 设置属性
func SetAttributes(ctx context.Context, attrs ...attribute.KeyValue) {
    span := trace.SpanFromContext(ctx)
    span.SetAttributes(attrs...)
}

// RecordError 记录错误
func RecordError(ctx context.Context, err error, opts ...trace.EventOption) {
    span := trace.SpanFromContext(ctx)
    span.RecordError(err, opts...)
}
```

### 5.3 Gin 中间件

```go
// pkg/trace/middleware.go

func GinMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        // 从请求头提取 trace context
        ctx := c.Request.Context()
        
        // 创建新 span
        ctx, span := tracer.Start(ctx, c.Request.Method+" "+c.FullPath())
        defer span.End()
        
        // 设置属性
        span.SetAttributes(
            attribute.String("http.method", c.Request.Method),
            attribute.String("http.url", c.Request.URL.String()),
            attribute.String("http.user_agent", c.Request.UserAgent()),
            attribute.String("http.client_ip", c.ClientIP()),
        )
        
        // 更新请求上下文
        c.Request = c.Request.WithContext(ctx)
        
        // 执行请求
        c.Next()
        
        // 记录响应
        span.SetAttributes(
            attribute.Int("http.status_code", c.Writer.Status()),
            attribute.Int("http.response_size", c.Writer.Size()),
        )
        
        // 记录错误
        if len(c.Errors) > 0 {
            span.RecordError(c.Errors.Last())
        }
    }
}
```

### 5.4 数据库追踪

```go
// pkg/trace/gorm.go

func GormTracingPlugin() gorm.Plugin {
    return &tracingPlugin{}
}

type tracingPlugin struct{}

func (p *tracingPlugin) Name() string {
    return "tracing"
}

func (p *tracingPlugin) Initialize(db *gorm.DB) error {
    db.Callback().Create().Before("gorm:create").Register("tracing:before_create", p.before("INSERT"))
    db.Callback().Create().After("gorm:create").Register("tracing:after_create", p.after())
    
    db.Callback().Query().Before("gorm:query").Register("tracing:before_query", p.before("SELECT"))
    db.Callback().Query().After("gorm:query").Register("tracing:after_query", p.after())
    
    db.Callback().Update().Before("gorm:update").Register("tracing:before_update", p.before("UPDATE"))
    db.Callback().Update().After("gorm:update").Register("tracing:after_update", p.after())
    
    db.Callback().Delete().Before("gorm:delete").Register("tracing:before_delete", p.before("DELETE"))
    db.Callback().Delete().After("gorm:delete").Register("tracing:after_delete", p.after())
    
    return nil
}

func (p *tracingPlugin) before(operation string) func(db *gorm.DB) {
    return func(db *gorm.DB) {
        ctx := db.Statement.Context
        ctx, span := tracer.Start(ctx, "db:"+operation)
        
        span.SetAttributes(
            attribute.String("db.system", "postgresql"),
            attribute.String("db.operation", operation),
            attribute.String("db.table", db.Statement.Table),
        )
        
        db.Statement.Context = ctx
    }
}

func (p *tracingPlugin) after() func(db *gorm.DB) {
    return func(db *gorm.DB) {
        span := trace.SpanFromContext(db.Statement.Context)
        defer span.End()
        
        if db.Error != nil {
            span.RecordError(db.Error)
        }
        
        span.SetAttributes(
            attribute.Int64("db.rows_affected", db.RowsAffected),
        )
    }
}
```

---

## 6. 限流熔断

### 6.1 限流实现（令牌桶）

```go
// pkg/ratelimit/token_bucket.go
package ratelimit

import (
    "context"
    "sync"
    "time"
)

// TokenBucket 令牌桶
type TokenBucket struct {
    rate       float64    // 令牌产生速率（每秒）
    capacity   int64      // 桶容量
    tokens     float64    // 当前令牌数
    lastUpdate time.Time  // 上次更新时间
    mu         sync.Mutex
}

func NewTokenBucket(rate float64, capacity int64) *TokenBucket {
    return &TokenBucket{
        rate:       rate,
        capacity:   capacity,
        tokens:     float64(capacity),
        lastUpdate: time.Now(),
    }
}

// Allow 是否允许请求
func (tb *TokenBucket) Allow() bool {
    return tb.AllowN(1)
}

// AllowN 是否允许 N 个请求
func (tb *TokenBucket) AllowN(n int64) bool {
    tb.mu.Lock()
    defer tb.mu.Unlock()
    
    now := time.Now()
    elapsed := now.Sub(tb.lastUpdate).Seconds()
    tb.lastUpdate = now
    
    // 添加新令牌
    tb.tokens += elapsed * tb.rate
    if tb.tokens > float64(tb.capacity) {
        tb.tokens = float64(tb.capacity)
    }
    
    // 检查令牌是否足够
    if tb.tokens >= float64(n) {
        tb.tokens -= float64(n)
        return true
    }
    
    return false
}

// Wait 等待获取令牌
func (tb *TokenBucket) Wait(ctx context.Context) error {
    return tb.WaitN(ctx, 1)
}

// WaitN 等待获取 N 个令牌
func (tb *TokenBucket) WaitN(ctx context.Context, n int64) error {
    for {
        if tb.AllowN(n) {
            return nil
        }
        
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-time.After(10 * time.Millisecond):
            // 继续尝试
        }
    }
}

// Limiter 限流器管理
type Limiter struct {
    buckets map[string]*TokenBucket
    mu      sync.RWMutex
}

func NewLimiter() *Limiter {
    return &Limiter{
        buckets: make(map[string]*TokenBucket),
    }
}

func (l *Limiter) GetBucket(key string, rate float64, capacity int64) *TokenBucket {
    l.mu.RLock()
    if bucket, ok := l.buckets[key]; ok {
        l.mu.RUnlock()
        return bucket
    }
    l.mu.RUnlock()
    
    l.mu.Lock()
    defer l.mu.Unlock()
    
    // 双重检查
    if bucket, ok := l.buckets[key]; ok {
        return bucket
    }
    
    bucket := NewTokenBucket(rate, capacity)
    l.buckets[key] = bucket
    return bucket
}
```

### 6.2 熔断器实现

```go
// pkg/circuitbreaker/circuitbreaker.go
package circuitbreaker

import (
    "context"
    "errors"
    "sync"
    "time"
)

// State 熔断器状态
type State int

const (
    StateClosed State = iota    // 关闭（正常）
    StateOpen                   // 打开（熔断）
    StateHalfOpen               // 半开（试探）
)

var ErrCircuitOpen = errors.New("circuit breaker is open")

type CircuitBreaker struct {
    name          string
    maxFailures   int           // 最大失败次数
    timeout       time.Duration // 熔断持续时间
    halfOpenMax   int           // 半开状态最大请求数
    
    state         State
    failures      int
    lastFailure   time.Time
    halfOpenCount int
    
    mu sync.RWMutex
}

func New(name string, maxFailures int, timeout time.Duration) *CircuitBreaker {
    return &CircuitBreaker{
        name:        name,
        maxFailures: maxFailures,
        timeout:     timeout,
        halfOpenMax: 5,
        state:       StateClosed,
    }
}

// Call 执行受保护的调用
func (cb *CircuitBreaker) Call(ctx context.Context, fn func() error) error {
    if !cb.allow() {
        return ErrCircuitOpen
    }
    
    err := fn()
    cb.recordResult(err)
    return err
}

func (cb *CircuitBreaker) allow() bool {
    cb.mu.Lock()
    defer cb.mu.Unlock()
    
    switch cb.state {
    case StateClosed:
        return true
        
    case StateOpen:
        // 检查是否超过熔断时间
        if time.Since(cb.lastFailure) > cb.timeout {
            cb.state = StateHalfOpen
            cb.halfOpenCount = 0
            return true
        }
        return false
        
    case StateHalfOpen:
        if cb.halfOpenCount < cb.halfOpenMax {
            cb.halfOpenCount++
            return true
        }
        return false
    }
    
    return false
}

func (cb *CircuitBreaker) recordResult(err error) {
    cb.mu.Lock()
    defer cb.mu.Unlock()
    
    if err == nil {
        // 成功
        switch cb.state {
        case StateHalfOpen:
            cb.halfOpenCount--
            if cb.halfOpenCount == 0 {
                // 恢复关闭状态
                cb.state = StateClosed
                cb.failures = 0
            }
        case StateClosed:
            cb.failures = 0
        }
    } else {
        // 失败
        cb.failures++
        cb.lastFailure = time.Now()
        
        switch cb.state {
        case StateHalfOpen:
            // 半开状态失败，重新熔断
            cb.state = StateOpen
            
        case StateClosed:
            if cb.failures >= cb.maxFailures {
                cb.state = StateOpen
            }
        }
    }
}

func (cb *CircuitBreaker) State() State {
    cb.mu.RLock()
    defer cb.mu.RUnlock()
    return cb.state
}
```

### 6.3 Gin 中间件

```go
// pkg/ratelimit/middleware.go

func RateLimitMiddleware(limiter *Limiter) gin.HandlerFunc {
    return func(c *gin.Context) {
        // 根据 IP 限流
        key := "ip:" + c.ClientIP()
        bucket := limiter.GetBucket(key, 100, 200) // 每秒100，桶容量200
        
        if !bucket.Allow() {
            c.JSON(429, gin.H{
                "code":    -1,
                "message": "请求过于频繁，请稍后重试",
            })
            c.Abort()
            return
        }
        
        c.Next()
    }
}

// 用户级别限流
func UserRateLimitMiddleware(limiter *Limiter) gin.HandlerFunc {
    return func(c *gin.Context) {
        userID, exists := c.Get("user_id")
        if !exists {
            c.Next()
            return
        }
        
        key := fmt.Sprintf("user:%v", userID)
        bucket := limiter.GetBucket(key, 10, 20) // 每秒10，桶容量20
        
        if !bucket.Allow() {
            c.JSON(429, gin.H{
                "code":    -1,
                "message": "请求过于频繁，请稍后重试",
            })
            c.Abort()
            return
        }
        
        c.Next()
    }
}
```

---

## 7. 集成到 Provider

```go
// internal/provider/provider.go

type Provider struct {
    // ... 现有字段
    
    // 分布式组件
    Discovery      discovery.Client
    ConfigCenter   configcenter.ConfigCenter
    LockManager    *distributedlock.LockManager
    Limiter        *ratelimit.Limiter
    CircuitBreaker map[string]*circuitbreaker.CircuitBreaker
}

// 分布式组件 Option
func WithDiscovery(addr string) Option {
    return func(p *Provider) error {
        client, err := discovery.NewConsulClient(addr)
        if err != nil {
            return err
        }
        p.Discovery = client
        return nil
    }
}

func WithConfigCenter(center configcenter.ConfigCenter) Option {
    return func(p *Provider) error {
        p.ConfigCenter = center
        return nil
    }
}

func WithDistributedLock() Option {
    return func(p *Provider) error {
        if p.RedisClient == nil {
            return errors.New("redis client required for distributed lock")
        }
        p.LockManager = distributedlock.NewLockManager(p.RedisClient)
        return nil
    }
}

func WithRateLimit() Option {
    return func(p *Provider) error {
        p.Limiter = ratelimit.NewLimiter()
        return nil
    }
}

func WithCircuitBreaker() Option {
    return func(p *Provider) error {
        p.CircuitBreaker = make(map[string]*circuitbreaker.CircuitBreaker)
        return nil
    }
}
```

---

## 8. 部署架构

### 8.1 多实例部署

```yaml
# docker-compose.yml
version: '3.8'

services:
  app1:
    build: .
    environment:
      - PORT=8081
      - SERVICE_ID=app-1
    ports:
      - "8081:8081"
    depends_on:
      - consul
      - redis
      - postgres
    
  app2:
    build: .
    environment:
      - PORT=8082
      - SERVICE_ID=app-2
    ports:
      - "8082:8082"
    depends_on:
      - consul
      - redis
      - postgres
    
  app3:
    build: .
    environment:
      - PORT=8083
      - SERVICE_ID=app-3
    ports:
      - "8083:8083"
    depends_on:
      - consul
      - redis
      - postgres

  nginx:
    image: nginx:alpine
    ports:
      - "80:80"
    volumes:
      - ./nginx.conf:/etc/nginx/nginx.conf
    depends_on:
      - app1
      - app2
      - app3

  consul:
    image: consul:1.15
    ports:
      - "8500:8500"
    command: consul agent -server -ui -bootstrap-expect=1 -client=0.0.0.0

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"

  postgres:
    image: postgres:14-alpine
    environment:
      - POSTGRES_USER=golang_web
      - POSTGRES_PASSWORD=secret
      - POSTGRES_DB=golang_web
    volumes:
      - postgres_data:/var/lib/postgresql/data

volumes:
  postgres_data:
```

### 8.2 Nginx 负载均衡

```nginx
upstream golang_web {
    least_conn;  # 最少连接算法
    
    server app1:8081 weight=5;
    server app2:8082 weight=5;
    server app3:8083 weight=5;
    
    keepalive 32;
}

server {
    listen 80;
    
    location / {
        proxy_pass http://golang_web;
        proxy_http_version 1.1;
        
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        
        proxy_connect_timeout 30s;
        proxy_send_timeout 30s;
        proxy_read_timeout 30s;
    }
}
```

---

## 9. 总结

### 9.1 扩展能力矩阵

| 能力 | 实现方案 | 状态 |
|------|----------|------|
| 服务注册发现 | Consul | ✅ |
| 配置中心 | Nacos/Consul | ✅ |
| 分布式锁 | Redis Redlock | ✅ |
| 分布式事务 | Saga | ✅ |
| 链路追踪 | OpenTelemetry + Jaeger | ✅ |
| 限流 | 令牌桶 | ✅ |
| 熔断 | 计数器模式 | ✅ |
| 负载均衡 | Nginx/Consul | ✅ |

### 9.2 演进路径

**阶段一：单体多实例**（当前 → 1个月）
- 多实例部署
- 负载均衡
- 分布式锁
- 限流熔断

**阶段二：服务化**（1-3个月）
- 服务拆分
- 服务注册发现
- 配置中心
- 链路追踪

**阶段三：完整分布式**（3-6个月）
- 分布式事务
- 消息队列
- 数据分片
- 多活架构

### 9.3 关键指标

| 指标 | 目标 |
|------|------|
| 可用性 | 99.9% |
| 响应时间 | P99 < 200ms |
| 吞吐量 | 10000 QPS |
| 数据一致性 | 最终一致 |
| 故障恢复 | < 30秒 |
