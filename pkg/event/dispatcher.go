package event

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	"github.com/zhimma/grove/pkg/logger"
)

// Event 事件接口
type Event interface {
	// EventName 返回事件名称
	EventName() string
}

// Listener 监听器接口
type Listener interface {
	// Handle 处理事件
	Handle(ctx context.Context, event Event) error
}

// ListenerFunc 监听器函数类型
type ListenerFunc func(ctx context.Context, event Event) error

// Handle 实现Listener接口
func (f ListenerFunc) Handle(ctx context.Context, event Event) error {
	return f(ctx, event)
}

// Dispatcher 事件调度器
type Dispatcher struct {
	listeners map[string][]Listener
	mutex     sync.RWMutex
	async     bool
	queue     chan *eventJob
	stopCh    chan struct{}
	wg        sync.WaitGroup
	closeOnce sync.Once
}

// eventJob 异步事件任务
type eventJob struct {
	ctx      context.Context
	event    Event
	listener Listener
}

// Config 调度器配置
type Config struct {
	Async     bool // 是否启用异步处理
	QueueSize int  // 异步队列大小
	WorkerNum int  // 工作协程数
}

// DefaultConfig 默认配置
func DefaultConfig() Config {
	return Config{
		Async:     false,
		QueueSize: 1000,
		WorkerNum: 10,
	}
}

// NewDispatcher 创建事件调度器
func NewDispatcher(config Config) *Dispatcher {
	d := &Dispatcher{
		listeners: make(map[string][]Listener),
		async:     config.Async,
		stopCh:    make(chan struct{}),
	}

	if config.Async {
		d.queue = make(chan *eventJob, config.QueueSize)
		for i := 0; i < config.WorkerNum; i++ {
			go d.worker()
		}
	}

	return d
}

// New 创建默认调度器（同步模式）
func New() *Dispatcher {
	return NewDispatcher(DefaultConfig())
}

// NewAsync 创建异步调度器
func NewAsync(queueSize, workerNum int) *Dispatcher {
	return NewDispatcher(Config{
		Async:     true,
		QueueSize: queueSize,
		WorkerNum: workerNum,
	})
}

// Close 关闭调度器
func (d *Dispatcher) Close() {
	if d.async {
		d.closeOnce.Do(func() {
			close(d.stopCh)
			d.wg.Wait()
		})
	}
}

// worker 异步工作协程
func (d *Dispatcher) worker() {
	for {
		select {
		case job := <-d.queue:
			if job == nil {
				return
			}
			d.executeListener(job.ctx, job.event, job.listener)
			d.wg.Done()
		case <-d.stopCh:
			return
		}
	}
}

// Listen 注册监听器
func (d *Dispatcher) Listen(eventName string, listener Listener) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.listeners[eventName] = append(d.listeners[eventName], listener)
	logger.Debug().Str("event", eventName).Msg("事件监听器已注册")
}

// ListenFunc 使用函数注册监听器
func (d *Dispatcher) ListenFunc(eventName string, handler ListenerFunc) {
	d.Listen(eventName, handler)
}

// Dispatch 分发事件（同步）
func (d *Dispatcher) Dispatch(ctx context.Context, event Event) error {
	listeners := d.getListeners(event.EventName())
	if len(listeners) == 0 {
		logger.Debug().Str("event", event.EventName()).Msg("事件没有监听器")
		return nil
	}

	for _, listener := range listeners {
		if err := d.executeListener(ctx, event, listener); err != nil {
			logger.Error().
				Err(err).
				Str("event", event.EventName()).
				Msg("事件监听器执行失败")
			// 继续执行其他监听器
		}
	}

	return nil
}

// DispatchAsync 异步分发事件
func (d *Dispatcher) DispatchAsync(ctx context.Context, event Event) error {
	if !d.async {
		return d.Dispatch(ctx, event)
	}

	listeners := d.getListeners(event.EventName())
	if len(listeners) == 0 {
		return nil
	}

	for _, listener := range listeners {
		d.wg.Add(1)
		select {
		case d.queue <- &eventJob{ctx: ctx, event: event, listener: listener}:
			// 成功入队
		default:
			d.wg.Done()
			logger.Warn().
				Str("event", event.EventName()).
				Msg("事件队列已满，事件已丢弃")
		}
	}

	return nil
}

// executeListener 执行监听器
func (d *Dispatcher) executeListener(ctx context.Context, event Event, listener Listener) error {
	defer func() {
		if r := recover(); r != nil {
			logger.Error().
				Interface("recover", r).
				Str("event", event.EventName()).
				Msg("事件监听器 panic 已恢复")
		}
	}()

	if err := listener.Handle(ctx, event); err != nil {
		return fmt.Errorf("listener handle: %w", err)
	}
	return nil
}

// getListeners 获取事件监听器（副本）
func (d *Dispatcher) getListeners(eventName string) []Listener {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	listeners := d.listeners[eventName]
	if len(listeners) == 0 {
		return nil
	}

	// 返回副本避免并发修改
	result := make([]Listener, len(listeners))
	copy(result, listeners)
	return result
}

// HasListeners 检查是否有监听器
func (d *Dispatcher) HasListeners(eventName string) bool {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	return len(d.listeners[eventName]) > 0
}

// Forget 移除所有监听器
func (d *Dispatcher) Forget(eventName string) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	delete(d.listeners, eventName)
}

// Flush 清空所有监听器
func (d *Dispatcher) Flush() {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.listeners = make(map[string][]Listener)
}

// Listeners 获取事件的所有监听器
func (d *Dispatcher) Listeners(eventName string) []Listener {
	return d.getListeners(eventName)
}

// ==================== 全局实例 ====================

var defaultDispatcher *Dispatcher

// Init 初始化全局调度器
func Init(dispatcher *Dispatcher) {
	defaultDispatcher = dispatcher
}

// Listen 全局注册监听器
func Listen(eventName string, listener Listener) {
	if defaultDispatcher == nil {
		defaultDispatcher = New()
	}
	defaultDispatcher.Listen(eventName, listener)
}

// ListenFunc 全局函数注册
func ListenFunc(eventName string, handler ListenerFunc) {
	if defaultDispatcher == nil {
		defaultDispatcher = New()
	}
	defaultDispatcher.ListenFunc(eventName, handler)
}

// Dispatch 全局分发
func Dispatch(ctx context.Context, event Event) error {
	if defaultDispatcher == nil {
		defaultDispatcher = New()
	}
	return defaultDispatcher.Dispatch(ctx, event)
}

// DispatchAsync 全局异步分发
func DispatchAsync(ctx context.Context, event Event) error {
	if defaultDispatcher == nil {
		defaultDispatcher = New()
	}
	return defaultDispatcher.DispatchAsync(ctx, event)
}

// HasListeners 全局检查
func HasListeners(eventName string) bool {
	if defaultDispatcher == nil {
		return false
	}
	return defaultDispatcher.HasListeners(eventName)
}

// ==================== 辅助函数 ====================

// eventNameFromType 从类型获取事件名
func eventNameFromType(t reflect.Type) string {
	return t.String()
}

// Subscribe 订阅事件类型（使用类型推断）
func Subscribe[T Event](dispatcher *Dispatcher, handler func(ctx context.Context, event T) error) {
	// 创建类型实例获取事件名
	var event T
	eventName := event.EventName()

	// 包装为通用监听器
	listener := ListenerFunc(func(ctx context.Context, e Event) error {
		// 类型断言
		if typedEvent, ok := e.(T); ok {
			return handler(ctx, typedEvent)
		}
		return fmt.Errorf("event type mismatch: expected %T, got %T", event, e)
	})

	dispatcher.Listen(eventName, listener)
}
