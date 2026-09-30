package scheduler

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/zhimma/grove/pkg/logger"
)

const (
	defaultStopTimeout = 30 * time.Second
	// defaultLockTTL bounds how long a crashed worker keeps a task's cluster
	// lock. Long enough for a slow job, short enough that a crash does not
	// silently stop the schedule for hours.
	defaultLockTTL             = 15 * time.Minute
	lockOpTimeout              = 3 * time.Second
	EverySecondSchedule        = "* * * * * *"
	EveryMinuteSchedule        = "0 * * * * *"
	EveryFiveMinutesSchedule   = "0 */5 * * * *"
	EveryTenMinutesSchedule    = "0 */10 * * * *"
	EveryThirtyMinutesSchedule = "0 */30 * * * *"
	HourlySchedule             = "0 0 * * * *"
	DailySchedule              = "0 0 0 * * *"
	WeeklySchedule             = "0 0 0 * * 0"
	MonthlySchedule            = "0 0 0 1 * *"
)

var (
	ErrTaskRunning = errors.New("scheduler task is already running")
	// ErrTaskNotFound separates "this task is not registered" from a failure
	// inside the task, so a caller can tell a disabled task from a broken one.
	ErrTaskNotFound     = errors.New("scheduler task not found")
	ErrSchedulerStopped = errors.New("scheduler is stopped")
	ErrStopTimeout      = errors.New("scheduler stop timed out")
)

type Job interface {
	Run(ctx context.Context) error
}

type JobFunc func(ctx context.Context) error

func (f JobFunc) Run(ctx context.Context) error {
	return f(ctx)
}

type Task struct {
	Name     string
	Schedule string
	Job      Job
	Mutex    bool
	Timeout  time.Duration
}

// LockStore 提供调度互斥所需的原子操作；释放锁时必须同时比较所有权。
type LockStore interface {
	Add(ctx context.Context, key string, value []byte, ttl time.Duration) (bool, error)
	CompareAndDelete(ctx context.Context, key string, expected []byte) (bool, error)
}

type Config struct {
	Location    string
	StopTimeout time.Duration
	// Lock 在锁租期内协调跨实例互斥；未设置时仅提供进程内互斥。
	Lock LockStore
	// LockTTL bounds how long a crashed worker can keep a task's cluster lock.
	LockTTL time.Duration
}

type Scheduler struct {
	cron        *cron.Cron
	tasks       map[string]*scheduledTask
	entries     map[string]cron.EntryID
	running     map[string]*taskState
	mu          sync.RWMutex
	wg          sync.WaitGroup
	rootCtx     context.Context
	cancel      context.CancelFunc
	started     bool
	stopped     bool
	stopOnce    sync.Once
	stoppedCh   chan struct{}
	stopTimeout time.Duration
	location    *time.Location
	lock        LockStore
	lockTTL     time.Duration
}

type scheduledTask struct {
	task  Task
	state *taskState
}

type taskState struct {
	active atomic.Int64
	locked atomic.Bool
}

func DefaultConfig() Config {
	return Config{
		Location:    "Local",
		StopTimeout: defaultStopTimeout,
	}
}

func New(config Config) (*Scheduler, error) {
	locationName := strings.TrimSpace(config.Location)
	if locationName == "" {
		locationName = "Local"
	}
	location := time.Local
	if locationName != "Local" {
		loaded, err := time.LoadLocation(locationName)
		if err != nil {
			return nil, fmt.Errorf("load scheduler location %q: %w", locationName, err)
		}
		location = loaded
	}
	if config.StopTimeout <= 0 {
		config.StopTimeout = defaultStopTimeout
	}
	if config.LockTTL <= 0 {
		config.LockTTL = defaultLockTTL
	}
	rootCtx, cancel := context.WithCancel(context.Background())
	s := &Scheduler{
		tasks:       make(map[string]*scheduledTask),
		entries:     make(map[string]cron.EntryID),
		running:     make(map[string]*taskState),
		rootCtx:     rootCtx,
		cancel:      cancel,
		stoppedCh:   make(chan struct{}),
		stopTimeout: config.StopTimeout,
		location:    location,
		lock:        config.Lock,
		lockTTL:     config.LockTTL,
	}
	s.cron = cron.New(
		cron.WithLocation(location),
		cron.WithParser(scheduleParser),
		cron.WithLogger(cron.VerbosePrintfLogger(&cronLogger{})),
		// robfig/cron's default chain is empty in v3.0.1. Keep an explicit
		// recovery wrapper so a panic in a scheduled job cannot kill the worker.
		cron.WithChain(cron.Recover(cron.VerbosePrintfLogger(&cronLogger{}))),
	)
	return s, nil
}

// scheduleParser is the single definition of what a Grove schedule looks like:
// six fields, seconds first, plus @every / @daily style descriptors. Both the
// running cron and ValidateSchedule use it, so a spec Console accepts is
// exactly a spec the Worker can run.
var scheduleParser = cron.NewParser(
	cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
)

// ValidateSchedule reports whether spec is a schedule this scheduler can run.
// Callers that persist a schedule should use it before writing, so an invalid
// expression is rejected at the edit rather than silently dropping a task at
// the next reconcile.
func ValidateSchedule(spec string) error {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return fmt.Errorf("schedule is required")
	}
	if _, err := scheduleParser.Parse(spec); err != nil {
		return fmt.Errorf("invalid schedule %q: %w", spec, err)
	}
	return nil
}

func NewDefault() (*Scheduler, error) {
	return New(DefaultConfig())
}

type cronLogger struct{}

func (*cronLogger) Printf(format string, values ...any) {
	logger.Debug().Msgf(format, values...)
}

func (s *Scheduler) Register(task *Task) error {
	normalized, err := normalizeTask(task)
	if err != nil {
		return err
	}
	record := &scheduledTask{task: normalized, state: &taskState{}}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return ErrSchedulerStopped
	}
	if _, exists := s.tasks[normalized.Name]; exists {
		return fmt.Errorf("task %q already exists", normalized.Name)
	}
	entryID, err := s.cron.AddFunc(normalized.Schedule, func() {
		_ = s.executeTask(record)
	})
	if err != nil {
		return fmt.Errorf("add cron task %q: %w", normalized.Name, err)
	}
	s.tasks[normalized.Name] = record
	s.entries[normalized.Name] = entryID
	s.running[normalized.Name] = record.state
	logger.Info().Str("task", normalized.Name).Str("schedule", normalized.Schedule).Msg("任务已注册")
	return nil
}

func normalizeTask(task *Task) (Task, error) {
	if task == nil {
		return Task{}, fmt.Errorf("scheduler task is required")
	}
	normalized := Task{
		Name:     strings.TrimSpace(task.Name),
		Schedule: strings.TrimSpace(task.Schedule),
		Job:      task.Job,
		Mutex:    task.Mutex,
		Timeout:  task.Timeout,
	}
	if normalized.Name == "" {
		return Task{}, fmt.Errorf("scheduler task name is required")
	}
	if normalized.Schedule == "" {
		return Task{}, fmt.Errorf("scheduler task schedule is required")
	}
	if isNilJob(normalized.Job) {
		return Task{}, fmt.Errorf("scheduler task job is required")
	}
	if normalized.Timeout < 0 {
		return Task{}, fmt.Errorf("scheduler task timeout must not be negative")
	}
	return normalized, nil
}

func isNilJob(job Job) bool {
	if job == nil {
		return true
	}
	value := reflect.ValueOf(job)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func (s *Scheduler) RegisterFunc(name, schedule string, fn JobFunc) error {
	return s.Register(&Task{Name: name, Schedule: schedule, Job: fn})
}

func (s *Scheduler) executeTask(record *scheduledTask) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("scheduler task %q panicked: %v", record.task.Name, recovered)
			logger.Error().Interface("panic", recovered).Str("task", record.task.Name).Msg("任务 panic 已隔离")
		}
	}()
	if record.task.Mutex && !record.state.locked.CompareAndSwap(false, true) {
		logger.Warn().Str("task", record.task.Name).Msg("任务正在运行，跳过本次执行")
		return ErrTaskRunning
	}
	if record.task.Mutex {
		defer record.state.locked.Store(false)

		release, acquired, lockErr := s.acquireClusterLock(record.task.Name)
		if lockErr != nil {
			return lockErr
		}
		if !acquired {
			logger.Warn().Str("task", record.task.Name).Msg("任务已被其他实例持有，跳过本次执行")
			return ErrTaskRunning
		}
		defer release()
	}

	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return ErrSchedulerStopped
	}
	s.wg.Add(1)
	record.state.active.Add(1)
	s.mu.Unlock()
	defer s.wg.Done()
	defer record.state.active.Add(-1)

	ctx := s.rootCtx
	cancel := func() {}
	if record.task.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, record.task.Timeout)
	}
	defer cancel()

	start := time.Now()
	logger.Info().Str("task", record.task.Name).Msg("任务开始执行")
	err = record.task.Job.Run(ctx)
	event := logger.Info()
	message := "任务执行完成"
	if err != nil {
		event = logger.Error().Err(err)
		message = "任务执行失败"
	}
	event.Str("task", record.task.Name).Dur("duration", time.Since(start)).Msg(message)
	return err
}

func (s *Scheduler) Start() error {
	if s == nil {
		return fmt.Errorf("scheduler is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return ErrSchedulerStopped
	}
	if s.started {
		return nil
	}
	s.cron.Start()
	s.started = true
	logger.Info().Msg("调度器已启动")
	return nil
}

func (s *Scheduler) Stop() error {
	if s == nil {
		return nil
	}
	s.stopOnce.Do(func() {
		s.mu.Lock()
		s.stopped = true
		s.cancel()
		s.mu.Unlock()
		cronDone := s.cron.Stop()
		go func() {
			<-cronDone.Done()
			s.wg.Wait()
			logger.Info().Msg("调度器已停止")
			close(s.stoppedCh)
		}()
	})

	timer := time.NewTimer(s.stopTimeout)
	defer timer.Stop()
	select {
	case <-s.stoppedCh:
		return nil
	case <-timer.C:
		return fmt.Errorf("%w after %s", ErrStopTimeout, s.stopTimeout)
	}
}

func (s *Scheduler) Run(name string) error {
	if s == nil {
		return fmt.Errorf("scheduler is nil")
	}
	name = strings.TrimSpace(name)
	s.mu.RLock()
	if s.stopped {
		s.mu.RUnlock()
		return ErrSchedulerStopped
	}
	record, exists := s.tasks[name]
	s.mu.RUnlock()
	if !exists {
		return fmt.Errorf("%w: %s", ErrTaskNotFound, name)
	}
	return s.executeTask(record)
}

func (s *Scheduler) Tasks() []string {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	names := make([]string, 0, len(s.tasks))
	for name := range s.tasks {
		names = append(names, name)
	}
	s.mu.RUnlock()
	sort.Strings(names)
	return names
}

func (s *Scheduler) Remove(name string) error {
	if s == nil {
		return fmt.Errorf("scheduler is nil")
	}
	name = strings.TrimSpace(name)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return ErrSchedulerStopped
	}
	if _, exists := s.tasks[name]; !exists {
		return fmt.Errorf("%w: %s", ErrTaskNotFound, name)
	}
	if entryID, exists := s.entries[name]; exists {
		s.cron.Remove(entryID)
	}
	delete(s.tasks, name)
	delete(s.entries, name)
	delete(s.running, name)
	logger.Info().Str("task", name).Msg("任务已移除")
	return nil
}

func (s *Scheduler) IsRunning(name string) bool {
	if s == nil {
		return false
	}
	s.mu.RLock()
	state := s.running[strings.TrimSpace(name)]
	s.mu.RUnlock()
	return state != nil && state.active.Load() > 0
}

func (s *Scheduler) EverySecond(name string, job Job) error {
	return s.Register(&Task{Name: name, Schedule: EverySecondSchedule, Job: job})
}

func (s *Scheduler) EveryMinute(name string, job Job) error {
	return s.Register(&Task{Name: name, Schedule: EveryMinuteSchedule, Job: job})
}

func (s *Scheduler) EveryFiveMinutes(name string, job Job) error {
	return s.Register(&Task{Name: name, Schedule: EveryFiveMinutesSchedule, Job: job})
}

func (s *Scheduler) EveryTenMinutes(name string, job Job) error {
	return s.Register(&Task{Name: name, Schedule: EveryTenMinutesSchedule, Job: job})
}

func (s *Scheduler) EveryThirtyMinutes(name string, job Job) error {
	return s.Register(&Task{Name: name, Schedule: EveryThirtyMinutesSchedule, Job: job})
}

func (s *Scheduler) Hourly(name string, job Job) error {
	return s.Register(&Task{Name: name, Schedule: HourlySchedule, Job: job})
}

func (s *Scheduler) Daily(name string, job Job) error {
	return s.Register(&Task{Name: name, Schedule: DailySchedule, Job: job})
}

func (s *Scheduler) DailyAt(name string, hour, minute int, job Job) error {
	return s.Register(&Task{Name: name, Schedule: fmt.Sprintf("0 %d %d * * *", minute, hour), Job: job})
}

func (s *Scheduler) Weekly(name string, job Job) error {
	return s.Register(&Task{Name: name, Schedule: WeeklySchedule, Job: job})
}

func (s *Scheduler) Monthly(name string, job Job) error {
	return s.Register(&Task{Name: name, Schedule: MonthlySchedule, Job: job})
}

// acquireClusterLock 获取有固定租期的共享锁；释放时原子校验所有权。
// 当前不自动续租，任务仍需限制执行时间并保证幂等。
func (s *Scheduler) acquireClusterLock(taskName string) (release func(), acquired bool, err error) {
	if s.lock == nil {
		return func() {}, true, nil
	}
	key := clusterLockKey(taskName)
	token := []byte(strconv.FormatInt(time.Now().UnixNano(), 36) + "-" + strconv.Itoa(os.Getpid()))

	ctx, cancel := context.WithTimeout(s.rootCtx, lockOpTimeout)
	defer cancel()

	stored, err := s.lock.Add(ctx, key, token, s.lockTTL)
	if err != nil {
		logger.Error().Err(err).Str("task", taskName).Msg("获取任务集群锁失败")
		return nil, false, fmt.Errorf("acquire cluster lock for task %q: %w", taskName, err)
	}
	if !stored {
		return nil, false, nil
	}

	return func() {
		releaseCtx, releaseCancel := context.WithTimeout(context.Background(), lockOpTimeout)
		defer releaseCancel()

		deleted, delErr := s.lock.CompareAndDelete(releaseCtx, key, token)
		if delErr != nil {
			logger.Error().Err(delErr).Str("task", taskName).Msg("释放任务集群锁失败")
		} else if !deleted {
			logger.Warn().Str("task", taskName).Msg("任务集群锁已失效或被接管，跳过释放")
		}
	}, true, nil
}

func clusterLockKey(taskName string) string {
	return "scheduler:lock:" + strings.TrimSpace(taskName)
}
