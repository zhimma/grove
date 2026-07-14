package event

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"runtime/debug"
	"strings"
	"sync"

	"github.com/zhimma/grove/pkg/logger"
)

var (
	ErrClosed    = errors.New("event dispatcher is closed")
	ErrQueueFull = errors.New("event dispatcher queue is full")
)

type Event interface {
	EventName() string
}

type Listener interface {
	Handle(ctx context.Context, event Event) error
}

type ListenerFunc func(ctx context.Context, event Event) error

func (f ListenerFunc) Handle(ctx context.Context, event Event) error {
	return f(ctx, event)
}

type ErrorHandler func(ctx context.Context, event Event, err error)

type Config struct {
	QueueSize    int
	WorkerNum    int
	ErrorHandler ErrorHandler
}

type ListenerPanicError struct {
	EventName string
	Recovered any
	Stack     []byte
}

func (e *ListenerPanicError) Error() string {
	if e == nil {
		return "event listener panic"
	}
	return fmt.Sprintf("event %q listener panic: %v", e.EventName, e.Recovered)
}

type Dispatcher struct {
	listeners    map[string][]Listener
	mu           sync.RWMutex
	queue        chan *eventJob
	workWG       sync.WaitGroup
	workerWG     sync.WaitGroup
	closeOnce    sync.Once
	closed       bool
	errorHandler ErrorHandler
}

type eventJob struct {
	ctx       context.Context
	event     Event
	eventName string
	listeners []Listener
}

func DefaultConfig() Config {
	return Config{
		QueueSize: 1000,
		WorkerNum: 10,
	}
}

func NewDispatcher(config Config) *Dispatcher {
	defaults := DefaultConfig()
	if config.QueueSize <= 0 {
		config.QueueSize = defaults.QueueSize
	}
	if config.WorkerNum <= 0 {
		config.WorkerNum = defaults.WorkerNum
	}
	if config.ErrorHandler == nil {
		config.ErrorHandler = defaultErrorHandler
	}
	d := &Dispatcher{
		listeners:    make(map[string][]Listener),
		queue:        make(chan *eventJob, config.QueueSize),
		errorHandler: config.ErrorHandler,
	}
	d.workerWG.Add(config.WorkerNum)
	for range config.WorkerNum {
		go d.worker()
	}
	return d
}

func New() *Dispatcher {
	return NewDispatcher(DefaultConfig())
}

func NewAsync(queueSize, workerNum int) *Dispatcher {
	return NewDispatcher(Config{QueueSize: queueSize, WorkerNum: workerNum})
}

func (d *Dispatcher) Close() error {
	if d == nil {
		return nil
	}
	d.closeOnce.Do(func() {
		d.mu.Lock()
		d.closed = true
		close(d.queue)
		d.mu.Unlock()
		d.workWG.Wait()
		d.workerWG.Wait()
	})
	return nil
}

func (d *Dispatcher) worker() {
	defer d.workerWG.Done()
	for job := range d.queue {
		if job == nil {
			continue
		}
		err := dispatchListeners(job.ctx, job.event, job.eventName, job.listeners)
		if err != nil {
			d.reportAsyncError(job.ctx, job.event, err)
		}
		d.workWG.Done()
	}
}

func (d *Dispatcher) Listen(eventName string, listener Listener) error {
	if d == nil {
		return fmt.Errorf("event dispatcher is nil")
	}
	eventName = strings.TrimSpace(eventName)
	if eventName == "" {
		return fmt.Errorf("event name is required")
	}
	if isNilValue(listener) {
		return fmt.Errorf("event listener is required")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return ErrClosed
	}
	d.listeners[eventName] = append(d.listeners[eventName], listener)
	logger.Debug().Str("event", eventName).Msg("事件监听器已注册")
	return nil
}

func (d *Dispatcher) ListenFunc(eventName string, handler ListenerFunc) error {
	return d.Listen(eventName, handler)
}

func (d *Dispatcher) Dispatch(ctx context.Context, event Event) error {
	if d == nil {
		return fmt.Errorf("event dispatcher is nil")
	}
	eventName, err := eventNameOf(event)
	if err != nil {
		return err
	}
	ctx = normalizeContext(ctx)
	d.mu.RLock()
	if d.closed {
		d.mu.RUnlock()
		return ErrClosed
	}
	listeners := copyListeners(d.listeners[eventName])
	if len(listeners) == 0 {
		d.mu.RUnlock()
		logger.Debug().Str("event", eventName).Msg("事件没有监听器")
		return nil
	}
	d.workWG.Add(1)
	d.mu.RUnlock()
	defer d.workWG.Done()
	return dispatchListeners(ctx, event, eventName, listeners)
}

func (d *Dispatcher) DispatchAsync(ctx context.Context, event Event) error {
	return d.enqueue(ctx, event, true)
}

func (d *Dispatcher) TryDispatchAsync(ctx context.Context, event Event) error {
	return d.enqueue(ctx, event, false)
}

func (d *Dispatcher) enqueue(ctx context.Context, event Event, wait bool) error {
	if d == nil {
		return fmt.Errorf("event dispatcher is nil")
	}
	eventName, err := eventNameOf(event)
	if err != nil {
		return err
	}
	ctx = normalizeContext(ctx)
	if err := contextError(ctx); err != nil {
		return err
	}
	jobCtx := context.WithoutCancel(ctx)
	d.mu.RLock()
	if d.closed {
		d.mu.RUnlock()
		return ErrClosed
	}
	listeners := copyListeners(d.listeners[eventName])
	if len(listeners) == 0 {
		d.mu.RUnlock()
		return nil
	}
	d.workWG.Add(1)
	job := &eventJob{ctx: jobCtx, event: event, eventName: eventName, listeners: listeners}
	if wait {
		select {
		case d.queue <- job:
			d.mu.RUnlock()
			return nil
		case <-ctx.Done():
			d.workWG.Done()
			d.mu.RUnlock()
			return ctx.Err()
		}
	}
	select {
	case d.queue <- job:
		d.mu.RUnlock()
		return nil
	default:
		d.workWG.Done()
		d.mu.RUnlock()
		return ErrQueueFull
	}
}

func dispatchListeners(ctx context.Context, event Event, eventName string, listeners []Listener) error {
	errs := make([]error, 0)
	for index, listener := range listeners {
		if err := executeListener(ctx, event, eventName, listener); err != nil {
			errs = append(errs, fmt.Errorf("event %q listener %d: %w", eventName, index, err))
		}
	}
	return errors.Join(errs...)
}

func executeListener(ctx context.Context, event Event, eventName string, listener Listener) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = &ListenerPanicError{
				EventName: eventName,
				Recovered: recovered,
				Stack:     debug.Stack(),
			}
		}
	}()
	if err := listener.Handle(ctx, event); err != nil {
		return fmt.Errorf("listener handle: %w", err)
	}
	return nil
}

func (d *Dispatcher) reportAsyncError(ctx context.Context, event Event, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			logger.Error().Interface("recover", recovered).Msg("事件错误处理器 panic 已恢复")
		}
	}()
	d.errorHandler(ctx, event, err)
}

func defaultErrorHandler(_ context.Context, event Event, err error) {
	eventName, nameErr := eventNameOf(event)
	if nameErr != nil {
		eventName = "unknown"
	}
	logger.Error().Err(err).Str("event", eventName).Msg("异步事件监听器执行失败")
}

func eventNameOf(event Event) (name string, err error) {
	if isNilValue(event) {
		return "", fmt.Errorf("event is required")
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			name = ""
			err = fmt.Errorf("get event name panic: %v", recovered)
		}
	}()
	name = strings.TrimSpace(event.EventName())
	if name == "" {
		return "", fmt.Errorf("event name is required")
	}
	return name, nil
}

func isNilValue(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}

func normalizeContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func contextError(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func copyListeners(listeners []Listener) []Listener {
	if len(listeners) == 0 {
		return nil
	}
	return append([]Listener(nil), listeners...)
}

func (d *Dispatcher) getListeners(eventName string) []Listener {
	if d == nil {
		return nil
	}
	d.mu.RLock()
	listeners := copyListeners(d.listeners[strings.TrimSpace(eventName)])
	d.mu.RUnlock()
	return listeners
}

func (d *Dispatcher) HasListeners(eventName string) bool {
	if d == nil {
		return false
	}
	d.mu.RLock()
	hasListeners := len(d.listeners[strings.TrimSpace(eventName)]) > 0
	d.mu.RUnlock()
	return hasListeners
}

func (d *Dispatcher) Forget(eventName string) {
	if d == nil {
		return
	}
	d.mu.Lock()
	delete(d.listeners, strings.TrimSpace(eventName))
	d.mu.Unlock()
}

func (d *Dispatcher) Flush() {
	if d == nil {
		return
	}
	d.mu.Lock()
	d.listeners = make(map[string][]Listener)
	d.mu.Unlock()
}

func (d *Dispatcher) Listeners(eventName string) []Listener {
	return d.getListeners(eventName)
}

func Subscribe[T Event](dispatcher *Dispatcher, handler func(ctx context.Context, event T) error) error {
	if dispatcher == nil {
		return fmt.Errorf("event dispatcher is required")
	}
	if handler == nil {
		return fmt.Errorf("event subscription handler is required")
	}
	sample, err := eventSample[T]()
	if err != nil {
		return err
	}
	eventName, err := eventNameOf(sample)
	if err != nil {
		return fmt.Errorf("infer event name: %w", err)
	}
	return dispatcher.Listen(eventName, ListenerFunc(func(ctx context.Context, event Event) error {
		typedEvent, ok := event.(T)
		if !ok {
			return fmt.Errorf("event type mismatch: expected %T, got %T", sample, event)
		}
		return handler(ctx, typedEvent)
	}))
}

func eventSample[T Event]() (T, error) {
	var zero T
	eventType := reflect.TypeOf((*T)(nil)).Elem()
	if eventType.Kind() != reflect.Pointer {
		return zero, nil
	}
	sample, ok := reflect.New(eventType.Elem()).Interface().(T)
	if !ok {
		return zero, fmt.Errorf("create event sample for %s", eventType)
	}
	return sample, nil
}
