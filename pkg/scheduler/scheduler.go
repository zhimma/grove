package scheduler

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/zhimma/grove/pkg/logger"
)

const (
	defaultStopTimeout         = 30 * time.Second
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
	ErrTaskRunning      = errors.New("scheduler task is already running")
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

type Config struct {
	Location    string
	StopTimeout time.Duration
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
	}
	s.cron = cron.New(
		cron.WithLocation(location),
		cron.WithSeconds(),
		cron.WithLogger(cron.VerbosePrintfLogger(&cronLogger{})),
	)
	return s, nil
}

func NewDefault() (*Scheduler, error) {
	return New(DefaultConfig())
}

type cronLogger struct{}

func (*cronLogger) Printf(format string, values ...interface{}) {
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

func (s *Scheduler) executeTask(record *scheduledTask) error {
	if record.task.Mutex && !record.state.locked.CompareAndSwap(false, true) {
		logger.Warn().Str("task", record.task.Name).Msg("任务正在运行，跳过本次执行")
		return ErrTaskRunning
	}
	if record.task.Mutex {
		defer record.state.locked.Store(false)
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
	err := record.task.Job.Run(ctx)
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
		return fmt.Errorf("task %q not found", name)
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
		return fmt.Errorf("task %q not found", name)
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
