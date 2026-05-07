package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/zhimma/grove/pkg/logger"
)

// Job 任务接口
type Job interface {
	// Run 执行任务
	Run(ctx context.Context) error
}

// JobFunc 任务函数类型
type JobFunc func(ctx context.Context) error

// Run 实现Job接口
func (f JobFunc) Run(ctx context.Context) error {
	return f(ctx)
}

// Task 计划任务
type Task struct {
	Name     string
	Schedule string
	Job      Job
	Mutex    bool // 是否启用互斥锁（防止重叠执行）
}

// Scheduler 任务调度器
type Scheduler struct {
	cron     *cron.Cron
	tasks    map[string]*Task
	entries  map[string]cron.EntryID
	mutex    sync.RWMutex
	running  map[string]*sync.Mutex // 任务级互斥锁
	stopCh   chan struct{}
	location *time.Location
}

// Config 调度器配置
type Config struct {
	Location string // 时区，默认Local
}

// DefaultConfig 默认配置
func DefaultConfig() Config {
	return Config{
		Location: "Local",
	}
}

// New 创建调度器
func New(config Config) (*Scheduler, error) {
	location := time.Local
	if config.Location != "" && config.Location != "Local" {
		loc, err := time.LoadLocation(config.Location)
		if err != nil {
			return nil, fmt.Errorf("load location: %w", err)
		}
		location = loc
	}

	s := &Scheduler{
		tasks:    make(map[string]*Task),
		entries:  make(map[string]cron.EntryID),
		running:  make(map[string]*sync.Mutex),
		stopCh:   make(chan struct{}),
		location: location,
	}

	// 创建cron调度器（启用秒字段）
	s.cron = cron.New(
		cron.WithLocation(location),
		cron.WithSeconds(),
		cron.WithLogger(cron.VerbosePrintfLogger(&cronLogger{})),
	)

	return s, nil
}

// NewDefault 创建默认调度器
func NewDefault() (*Scheduler, error) {
	return New(DefaultConfig())
}

// cronLogger 适配cron的日志接口
type cronLogger struct{}

func (l *cronLogger) Printf(format string, v ...interface{}) {
	logger.Debug().Msgf(format, v...)
}

// Register 注册任务
func (s *Scheduler) Register(task *Task) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, exists := s.tasks[task.Name]; exists {
		return fmt.Errorf("task %s already exists", task.Name)
	}

	// 包装任务执行
	wrapper := func() {
		s.executeTask(task)
	}

	// 添加到cron
	entryID, err := s.cron.AddFunc(task.Schedule, wrapper)
	if err != nil {
		return fmt.Errorf("add cron job: %w", err)
	}

	s.tasks[task.Name] = task
	s.entries[task.Name] = entryID
	if task.Mutex {
		s.running[task.Name] = &sync.Mutex{}
	}

	logger.Info().
		Str("task", task.Name).
		Str("schedule", task.Schedule).
		Msg("任务已注册")

	return nil
}

// RegisterFunc 使用函数注册任务
func (s *Scheduler) RegisterFunc(name, schedule string, fn JobFunc) error {
	return s.Register(&Task{
		Name:     name,
		Schedule: schedule,
		Job:      fn,
	})
}

// executeTask 执行任务
func (s *Scheduler) executeTask(task *Task) {
	// 互斥锁检查
	if task.Mutex {
		mutex, exists := s.running[task.Name]
		if exists {
			if !mutex.TryLock() {
				logger.Warn().
					Str("task", task.Name).
					Msg("任务正在运行，跳过本次执行")
				return
			}
			defer mutex.Unlock()
		}
	}

	start := time.Now()
	logger.Info().
		Str("task", task.Name).
		Msg("任务开始执行")

	ctx := context.Background()
	if err := task.Job.Run(ctx); err != nil {
		logger.Error().
			Err(err).
			Str("task", task.Name).
			Dur("duration", time.Since(start)).
			Msg("任务执行失败")
	} else {
		logger.Info().
			Str("task", task.Name).
			Dur("duration", time.Since(start)).
			Msg("任务执行完成")
	}
}

// Start 启动调度器
func (s *Scheduler) Start() {
	s.cron.Start()
	logger.Info().Msg("调度器已启动")
}

// Stop 停止调度器
func (s *Scheduler) Stop() {
	ctx := s.cron.Stop()
	<-ctx.Done()
	logger.Info().Msg("调度器已停止")
}

// Run 立即运行一次任务（阻塞）
func (s *Scheduler) Run(name string) error {
	s.mutex.RLock()
	task, exists := s.tasks[name]
	s.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("task %s not found", name)
	}

	s.executeTask(task)
	return nil
}

// Tasks 获取所有任务名称
func (s *Scheduler) Tasks() []string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	names := make([]string, 0, len(s.tasks))
	for name := range s.tasks {
		names = append(names, name)
	}
	return names
}

// Remove 移除任务（注意：cron库不支持动态移除，需要重建）
func (s *Scheduler) Remove(name string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, exists := s.tasks[name]; !exists {
		return fmt.Errorf("task %s not found", name)
	}

	delete(s.tasks, name)
	if entryID, exists := s.entries[name]; exists {
		s.cron.Remove(entryID)
	}
	delete(s.entries, name)
	delete(s.running, name)

	logger.Info().Str("task", name).Msg("任务已移除")
	return nil
}

// IsRunning 检查任务是否正在执行
func (s *Scheduler) IsRunning(name string) bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	mutex, exists := s.running[name]
	if !exists {
		return false
	}

	// 尝试获取锁，如果失败说明正在运行
	if mutex.TryLock() {
		mutex.Unlock()
		return false
	}
	return true
}

// ==================== 便捷方法 ====================

// EverySecond 每秒执行
func (s *Scheduler) EverySecond(name string, job Job) error {
	return s.Register(&Task{Name: name, Schedule: "* * * * * *", Job: job})
}

// EveryMinute 每分钟执行
func (s *Scheduler) EveryMinute(name string, job Job) error {
	return s.Register(&Task{Name: name, Schedule: "0 * * * * *", Job: job})
}

// EveryFiveMinutes 每5分钟执行
func (s *Scheduler) EveryFiveMinutes(name string, job Job) error {
	return s.Register(&Task{Name: name, Schedule: "0 */5 * * * *", Job: job})
}

// EveryTenMinutes 每10分钟执行
func (s *Scheduler) EveryTenMinutes(name string, job Job) error {
	return s.Register(&Task{Name: name, Schedule: "0 */10 * * * *", Job: job})
}

// EveryThirtyMinutes 每30分钟执行
func (s *Scheduler) EveryThirtyMinutes(name string, job Job) error {
	return s.Register(&Task{Name: name, Schedule: "0 */30 * * * *", Job: job})
}

// Hourly 每小时执行
func (s *Scheduler) Hourly(name string, job Job) error {
	return s.Register(&Task{Name: name, Schedule: "0 0 * * * *", Job: job})
}

// Daily 每天执行（午夜）
func (s *Scheduler) Daily(name string, job Job) error {
	return s.Register(&Task{Name: name, Schedule: "0 0 0 * * *", Job: job})
}

// DailyAt 指定时间每天执行
func (s *Scheduler) DailyAt(name string, hour, minute int, job Job) error {
	schedule := fmt.Sprintf("0 %d %d * * *", minute, hour)
	return s.Register(&Task{Name: name, Schedule: schedule, Job: job})
}

// Weekly 每周执行（周日午夜）
func (s *Scheduler) Weekly(name string, job Job) error {
	return s.Register(&Task{Name: name, Schedule: "0 0 0 * * 0", Job: job})
}

// Monthly 每月执行（1号午夜）
func (s *Scheduler) Monthly(name string, job Job) error {
	return s.Register(&Task{Name: name, Schedule: "0 0 0 1 * *", Job: job})
}

// ==================== 全局实例 ====================

var defaultScheduler *Scheduler

// Init 初始化全局调度器
func Init(scheduler *Scheduler) {
	defaultScheduler = scheduler
}

// Register 全局注册
func Register(task *Task) error {
	if defaultScheduler == nil {
		return fmt.Errorf("scheduler not initialized")
	}
	return defaultScheduler.Register(task)
}

// RegisterFunc 全局函数注册
func RegisterFunc(name, schedule string, fn JobFunc) error {
	if defaultScheduler == nil {
		return fmt.Errorf("scheduler not initialized")
	}
	return defaultScheduler.RegisterFunc(name, schedule, fn)
}

// Start 全局启动
func Start() {
	if defaultScheduler != nil {
		defaultScheduler.Start()
	}
}

// Stop 全局停止
func Stop() {
	if defaultScheduler != nil {
		defaultScheduler.Stop()
	}
}

// Run 全局立即运行
func Run(name string) error {
	if defaultScheduler == nil {
		return fmt.Errorf("scheduler not initialized")
	}
	return defaultScheduler.Run(name)
}

// ==================== Cron 表达式帮助 ====================

// CronExpression 预定义的Cron表达式
var CronExpression = struct {
	EverySecond        string
	EveryMinute        string
	EveryFiveMinutes   string
	EveryTenMinutes    string
	EveryThirtyMinutes string
	Hourly             string
	Daily              string
	Weekly             string
	Monthly            string
}{
	EverySecond:        "* * * * * *",
	EveryMinute:        "0 * * * * *",
	EveryFiveMinutes:   "0 */5 * * * *",
	EveryTenMinutes:    "0 */10 * * * *",
	EveryThirtyMinutes: "0 */30 * * * *",
	Hourly:             "0 0 * * * *",
	Daily:              "0 0 0 * * *",
	Weekly:             "0 0 0 * * 0",
	Monthly:            "0 0 0 1 * *",
}
