package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/zhimma/grove/internal/model"
	"github.com/zhimma/grove/pkg/database"
	"github.com/zhimma/grove/pkg/errx"
	"github.com/zhimma/grove/pkg/pagination"
	"github.com/zhimma/grove/pkg/scheduler"
)

// ScheduledTaskService edits the schedule of tasks the Worker already defines.
// It deliberately has no create or delete: rows mirror the Worker's code
// registry, so a row Console invented would name a task with no handler.
type ScheduledTaskService struct {
	dbs   *database.Connections
	pages pagination.Policy
	now   func() time.Time
}

func NewScheduledTaskService(dbs *database.Connections, pages pagination.Policy) *ScheduledTaskService {
	return &ScheduledTaskService{
		dbs:   dbs,
		pages: pages,
		now:   time.Now,
	}
}

type ListScheduledTasksInput struct {
	pagination.Request
	Keyword string
	Enabled *bool
}

type ListScheduledTasksOutput struct {
	List []model.ConsoleScheduledTask
	Meta pagination.Meta
}

type UpdateScheduledTaskInput struct {
	TaskID         string
	Schedule       string
	Mutex          *bool
	TimeoutSeconds *int
}

type SetScheduledTaskStatusInput struct {
	TaskID  string
	Enabled bool
}

type RequestScheduledTaskRunInput struct {
	TaskID string
}

func (s *ScheduledTaskService) List(ctx context.Context, in ListScheduledTasksInput) (*ListScheduledTasksOutput, error) {
	db, err := s.defaultDB(ctx)
	if err != nil {
		return nil, err
	}

	query := db.Model(&model.ConsoleScheduledTask{})
	if keyword := strings.TrimSpace(in.Keyword); keyword != "" {
		pattern := "%" + keyword + "%"
		query = query.Where("name LIKE ? OR display_name LIKE ?", pattern, pattern)
	}
	if in.Enabled != nil {
		query = query.Where("enabled = ?", *in.Enabled)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, errx.Internal().WithCause(err)
	}

	page := s.pages.Resolve(in.Request)
	var tasks []model.ConsoleScheduledTask
	if err := page.Apply(query.Order("name ASC")).Find(&tasks).Error; err != nil {
		return nil, errx.Internal().WithCause(err)
	}

	return &ListScheduledTasksOutput{
		List: tasks,
		Meta: pagination.NewMeta(total, page),
	}, nil
}

// Update changes when a task runs. The schedule is parsed with the same parser
// the Worker's cron uses, so an expression Console accepts is one the Worker
// can actually register.
func (s *ScheduledTaskService) Update(ctx context.Context, in UpdateScheduledTaskInput) (*model.ConsoleScheduledTask, error) {
	db, err := s.defaultDB(ctx)
	if err != nil {
		return nil, err
	}
	task, err := s.find(db, in.TaskID)
	if err != nil {
		return nil, err
	}

	updates := map[string]any{}
	if schedule := strings.TrimSpace(in.Schedule); schedule != "" && schedule != task.Schedule {
		if err := scheduler.ValidateSchedule(schedule); err != nil {
			return nil, errx.BadRequest().WithMessage("调度表达式无效，需为 6 段（秒 分 时 日 月 周）或 @every 形式")
		}
		updates["schedule"] = schedule
	}
	if in.Mutex != nil && *in.Mutex != task.Mutex {
		updates["mutex"] = *in.Mutex
	}
	if in.TimeoutSeconds != nil && *in.TimeoutSeconds != task.TimeoutSeconds {
		if *in.TimeoutSeconds < 0 {
			return nil, errx.BadRequest().WithMessage("超时时间不能为负数")
		}
		updates["timeout_seconds"] = *in.TimeoutSeconds
	}
	if len(updates) == 0 {
		return task, nil
	}

	if err := db.Model(&model.ConsoleScheduledTask{}).
		Where("id = ?", task.ID).
		Updates(updates).Error; err != nil {
		return nil, errx.Internal().WithCause(err)
	}
	return s.find(db, task.ID)
}

func (s *ScheduledTaskService) SetStatus(ctx context.Context, in SetScheduledTaskStatusInput) (*model.ConsoleScheduledTask, error) {
	db, err := s.defaultDB(ctx)
	if err != nil {
		return nil, err
	}
	task, err := s.find(db, in.TaskID)
	if err != nil {
		return nil, err
	}
	if task.Enabled == in.Enabled {
		return task, nil
	}
	if err := db.Model(&model.ConsoleScheduledTask{}).
		Where("id = ?", task.ID).
		Update("enabled", in.Enabled).Error; err != nil {
		return nil, errx.Internal().WithCause(err)
	}
	return s.find(db, task.ID)
}

// RequestRun asks the Worker to run the task once outside its schedule. It
// only records the request; the Worker claims and executes it on its next
// reconcile, so the response means "queued", not "finished".
func (s *ScheduledTaskService) RequestRun(ctx context.Context, in RequestScheduledTaskRunInput) (*model.ConsoleScheduledTask, error) {
	db, err := s.defaultDB(ctx)
	if err != nil {
		return nil, err
	}
	task, err := s.find(db, in.TaskID)
	if err != nil {
		return nil, err
	}
	if !task.Enabled {
		return nil, errx.BadRequest().WithMessage("任务已停用，请先启用后再执行")
	}
	if task.HasPendingRunRequest() {
		return nil, errx.BadRequest().WithMessage("已有一次待执行的手动请求，请等待其完成")
	}

	if err := db.Model(&model.ConsoleScheduledTask{}).
		Where("id = ? AND run_requested_at IS NULL", task.ID).
		Update("run_requested_at", s.now()).Error; err != nil {
		return nil, errx.Internal().WithCause(err)
	}
	return s.find(db, task.ID)
}

func (s *ScheduledTaskService) find(db *gorm.DB, taskID string) (*model.ConsoleScheduledTask, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, errx.BadRequest().WithMessage("任务ID不能为空")
	}
	var task model.ConsoleScheduledTask
	if err := db.Where("id = ?", taskID).First(&task).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errx.NotFound().WithMessage("计划任务不存在")
		}
		return nil, errx.Internal().WithCause(err)
	}
	return &task, nil
}

func (s *ScheduledTaskService) defaultDB(ctx context.Context) (*gorm.DB, error) {
	if s.dbs == nil || s.dbs.Default() == nil {
		return nil, errx.ServiceUnavailable().WithMessage("默认数据库未配置")
	}
	return s.dbs.Default().WithContext(ctx), nil
}
