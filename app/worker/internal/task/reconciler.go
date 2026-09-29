package task

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/zhimma/grove/internal/model"
	"github.com/zhimma/grove/pkg/database"
	"github.com/zhimma/grove/pkg/logger"
	"github.com/zhimma/grove/pkg/scheduler"
)

// DefaultReconcileInterval bounds how long a console edit waits before the
// worker picks it up. Same trade as the casbin policy reload: a short poll
// beats a redeploy, and a schedule change is not latency sensitive.
const DefaultReconcileInterval = 30 * time.Second

// lastErrorLimit matches console_scheduled_tasks.last_error so a long failure
// message is truncated here rather than rejected by the database.
const lastErrorLimit = 1000

// Reconciler keeps the running scheduler in step with console_scheduled_tasks.
// The database owns when a task runs; this package owns what it does. A row
// naming a task this binary does not define has no handler, so it is reported
// and otherwise ignored.
type Reconciler struct {
	dbs         *database.Connections
	scheduler   *scheduler.Scheduler
	definitions map[string]Definition
	interval    time.Duration
	now         func() time.Time

	// applied records what was last pushed into the scheduler, because the
	// scheduler exposes task names but not the schedule behind them.
	applied map[string]appliedSchedule
}

type appliedSchedule struct {
	schedule string
	mutex    bool
	timeout  time.Duration
}

func NewReconciler(dbs *database.Connections, sched *scheduler.Scheduler, definitions map[string]Definition, interval time.Duration) (*Reconciler, error) {
	if dbs == nil {
		return nil, fmt.Errorf("task reconciler requires database connections")
	}
	if sched == nil {
		return nil, fmt.Errorf("task reconciler requires a scheduler")
	}
	if interval <= 0 {
		interval = DefaultReconcileInterval
	}
	return &Reconciler{
		dbs:         dbs,
		scheduler:   sched,
		definitions: definitions,
		interval:    interval,
		now:         time.Now,
		applied:     map[string]appliedSchedule{},
	}, nil
}

// Run reconciles once immediately, then on every tick until ctx is done. A
// failed pass is logged and retried rather than fatal: a database blip should
// not take the worker down or silently freeze the schedule.
func (r *Reconciler) Run(ctx context.Context) {
	if err := r.Reconcile(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error().Err(err).Msg("计划任务首次对账失败")
	}

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := r.Reconcile(ctx); err != nil && !errors.Is(err, context.Canceled) {
				logger.Error().Err(err).Msg("计划任务对账失败")
			}
		}
	}
}

// Reconcile runs one pass: seed rows for newly defined tasks, then make the
// scheduler match the rows.
func (r *Reconciler) Reconcile(ctx context.Context) error {
	db := r.dbs.Default()
	if db == nil {
		return fmt.Errorf("task reconciler requires a default database")
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	if err := r.seedMissingRows(ctx, db); err != nil {
		return err
	}

	var rows []model.ConsoleScheduledTask
	if err := db.WithContext(ctx).Find(&rows).Error; err != nil {
		return fmt.Errorf("load scheduled tasks: %w", err)
	}

	for _, row := range rows {
		definition, defined := r.definitions[row.Name]
		switch {
		case !defined:
			// A row left behind by an older binary, or one rolled back. It can
			// never run, so make sure it is not still scheduled from a previous
			// pass and say so once per pass.
			logger.Warn().Str("task", row.Name).Msg("计划任务在代码中不存在，已跳过")
			r.unschedule(row.Name)
			r.declineRunRequest(ctx, db, row, "任务在代码中不存在，无法执行")
		case !row.Enabled:
			r.unschedule(row.Name)
			r.declineRunRequest(ctx, db, row, "任务已停用，未执行")
		default:
			r.schedule(definition, row)
			if row.HasPendingRunRequest() {
				r.runOnRequest(ctx, db, row.Name)
			}
		}
	}
	return nil
}

// declineRunRequest answers a manual trigger that cannot run. Leaving the flag
// set would show as "pending" in the console forever with no explanation.
func (r *Reconciler) declineRunRequest(ctx context.Context, db *gorm.DB, row model.ConsoleScheduledTask, reason string) {
	if !row.HasPendingRunRequest() {
		return
	}
	if !r.claimRunRequest(ctx, db, row.Name) {
		return
	}
	r.recordSkip(row.Name, reason)
}

// claimRunRequest clears the pending flag and reports whether this worker was
// the one that cleared it. Several replicas reconcile the same table at once,
// so this conditional update is what makes a manual trigger run exactly once
// instead of once per replica.
func (r *Reconciler) claimRunRequest(ctx context.Context, db *gorm.DB, name string) bool {
	claim := db.WithContext(ctx).
		Model(&model.ConsoleScheduledTask{}).
		Where("name = ? AND run_requested_at IS NOT NULL", name).
		Update("run_requested_at", nil)
	if claim.Error != nil {
		logger.Error().Err(claim.Error).Str("task", name).Msg("认领手动执行请求失败")
		return false
	}
	return claim.RowsAffected > 0
}

// runOnRequest executes a task the console asked to run off schedule.
//
// ponytail: the request is picked up on the next reconcile, so a trigger waits
// up to one interval. Dispatch through asynq (Console would need WithJob) if
// operators need it to feel immediate.
func (r *Reconciler) runOnRequest(ctx context.Context, db *gorm.DB, name string) {
	if !r.claimRunRequest(ctx, db, name) {
		return
	}

	logger.Info().Str("task", name).Msg("手动执行计划任务")
	go func() {
		// Scheduler.Run applies the task's mutex, cluster lock and timeout, so
		// a manual run cannot double up with the scheduled one.
		err := r.scheduler.Run(name)
		switch {
		case err == nil:
		case errors.Is(err, scheduler.ErrTaskNotFound):
			// Disabled between the console click and this pass.
			r.recordSkip(name, "任务未在调度中，已跳过手动执行")
		case errors.Is(err, scheduler.ErrTaskRunning):
			r.recordSkip(name, "任务正在运行，已跳过手动执行")
		default:
			logger.Error().Err(err).Str("task", name).Msg("手动执行计划任务失败")
		}
	}()
}

// recordSkip marks a run that never started, so the console shows why instead
// of leaving the previous result in place.
func (r *Reconciler) recordSkip(name, reason string) {
	db := r.dbs.Default()
	if db == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.WithContext(ctx).
		Model(&model.ConsoleScheduledTask{}).
		Where("name = ?", name).
		Updates(map[string]any{
			"last_run_at": r.now(),
			"last_status": model.ScheduledTaskStatusSkipped,
			"last_error":  truncate(reason, lastErrorLimit),
		}).Error; err != nil {
		logger.Error().Err(err).Str("task", name).Msg("回写计划任务跳过原因失败")
	}
}

// seedMissingRows inserts a row for every task the code defines but the table
// does not have yet. Existing rows are left untouched: once an operator has
// retuned a schedule, a redeploy must not silently reset it.
func (r *Reconciler) seedMissingRows(ctx context.Context, db *gorm.DB) error {
	if len(r.definitions) == 0 {
		return nil
	}

	var existing []string
	if err := db.WithContext(ctx).
		Model(&model.ConsoleScheduledTask{}).
		Pluck("name", &existing).Error; err != nil {
		return fmt.Errorf("load scheduled task names: %w", err)
	}
	known := make(map[string]struct{}, len(existing))
	for _, name := range existing {
		known[name] = struct{}{}
	}

	pending := make([]model.ConsoleScheduledTask, 0, len(r.definitions))
	for name, definition := range r.definitions {
		if _, ok := known[name]; ok {
			continue
		}
		pending = append(pending, model.ConsoleScheduledTask{
			Name:           definition.Name,
			DisplayName:    definition.DisplayName,
			Schedule:       definition.Schedule,
			Enabled:        true,
			Mutex:          definition.Mutex,
			TimeoutSeconds: int(definition.Timeout / time.Second),
		})
	}
	if len(pending) == 0 {
		return nil
	}
	if err := db.WithContext(ctx).Create(&pending).Error; err != nil {
		return fmt.Errorf("seed scheduled tasks: %w", err)
	}
	for _, task := range pending {
		logger.Info().Str("task", task.Name).Str("schedule", task.Schedule).Msg("已登记新的计划任务")
	}
	return nil
}

// schedule registers the task, or re-registers it when the row no longer
// matches what is running. Nothing happens when the row is unchanged.
func (r *Reconciler) schedule(definition Definition, row model.ConsoleScheduledTask) {
	wanted := appliedSchedule{
		schedule: row.Schedule,
		mutex:    row.Mutex,
		timeout:  row.Timeout(),
	}
	if current, ok := r.applied[row.Name]; ok {
		if current == wanted {
			return
		}
		r.unschedule(row.Name)
	}

	// A schedule the parser rejects would abort registration for this task
	// only; the rest of the table still reconciles.
	if err := scheduler.ValidateSchedule(wanted.schedule); err != nil {
		logger.Error().Err(err).Str("task", row.Name).Msg("计划任务表达式无效，已跳过")
		return
	}

	err := r.scheduler.Register(&scheduler.Task{
		Name:     definition.Name,
		Schedule: wanted.schedule,
		Job:      r.instrument(definition),
		Mutex:    wanted.mutex,
		Timeout:  wanted.timeout,
	})
	if err != nil {
		logger.Error().Err(err).Str("task", row.Name).Msg("注册计划任务失败")
		return
	}
	r.applied[row.Name] = wanted
	logger.Info().Str("task", row.Name).Str("schedule", wanted.schedule).Msg("计划任务已应用")
}

func (r *Reconciler) unschedule(name string) {
	if _, ok := r.applied[name]; !ok {
		return
	}
	if err := r.scheduler.Remove(name); err != nil {
		logger.Error().Err(err).Str("task", name).Msg("移除计划任务失败")
		return
	}
	delete(r.applied, name)
	logger.Info().Str("task", name).Msg("计划任务已停用")
}

// instrument wraps the job so every run leaves its outcome on the row. The
// console list is the only place an operator can see whether a task worked.
func (r *Reconciler) instrument(definition Definition) scheduler.JobFunc {
	return func(ctx context.Context) error {
		startedAt := r.now()
		err := definition.Job.Run(ctx)
		r.recordRun(definition.Name, startedAt, err)
		return err
	}
}

func (r *Reconciler) recordRun(name string, startedAt time.Time, runErr error) {
	db := r.dbs.Default()
	if db == nil {
		return
	}

	status := model.ScheduledTaskStatusSuccess
	message := ""
	if runErr != nil {
		status = model.ScheduledTaskStatusFailed
		message = truncate(runErr.Error(), lastErrorLimit)
	}

	// Deliberately not the task's context: a task cancelled by timeout still
	// needs its failure recorded, and that context is already done.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	updates := map[string]any{
		"last_run_at":      startedAt,
		"last_status":      status,
		"last_error":       message,
		"last_duration_ms": r.now().Sub(startedAt).Milliseconds(),
	}
	if err := db.WithContext(ctx).
		Model(&model.ConsoleScheduledTask{}).
		Where("name = ?", name).
		Updates(updates).Error; err != nil {
		logger.Error().Err(err).Str("task", name).Msg("回写计划任务执行结果失败")
	}
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}
