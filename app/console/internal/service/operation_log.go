package service

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/zhimma/grove/internal/model"
	"github.com/zhimma/grove/pkg/database"
	"github.com/zhimma/grove/pkg/errx"
)

// OperationLogService 操作日志服务
type OperationLogService struct {
	dbRepo database.Repo
}

// NewOperationLogService 创建操作日志服务
func NewOperationLogService(dbRepo database.Repo) *OperationLogService {
	return &OperationLogService{dbRepo: dbRepo}
}

// ListInput 列表输入参数
type OperationLogListInput struct {
	AdminID   string // 管理员ID
	Action    string // 操作动作
	Method    string // 请求方法
	Path      string // 请求路径
	Status    int    // 状态
	StartTime string // 开始时间
	EndTime   string // 结束时间
	Page      int
	PageSize  int
}

// ListOutput 列表输出
type OperationLogListOutput struct {
	List  []model.ConsoleOperationLog
	Total int64
}

// List 获取操作日志列表
func (s *OperationLogService) List(ctx context.Context, in *OperationLogListInput) (*OperationLogListOutput, error) {
	db, err := s.defaultDB(ctx)
	if err != nil {
		return nil, err
	}
	query := db.Model(&model.ConsoleOperationLog{})

	// 筛选条件
	if in.AdminID != "" {
		query = query.Where("admin_id = ?", in.AdminID)
	}
	if in.Action != "" {
		query = query.Where("action LIKE ?", "%"+in.Action+"%")
	}
	if in.Method != "" {
		query = query.Where("method = ?", in.Method)
	}
	if in.Path != "" {
		query = query.Where("path LIKE ?", "%"+in.Path+"%")
	}
	if in.Status > 0 {
		query = query.Where("status = ?", in.Status)
	}
	if in.StartTime != "" {
		query = query.Where("created_at >= ?", in.StartTime)
	}
	if in.EndTime != "" {
		query = query.Where("created_at <= ?", in.EndTime)
	}

	// 统计总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, errx.Internal().WithCause(err)
	}

	// 分页查询
	var logs []model.ConsoleOperationLog
	offset := (in.Page - 1) * in.PageSize
	if err := query.Order("created_at DESC").Offset(offset).Limit(in.PageSize).Find(&logs).Error; err != nil {
		return nil, errx.Internal().WithCause(err)
	}

	return &OperationLogListOutput{
		List:  logs,
		Total: total,
	}, nil
}

// GetInput 获取详情输入
type OperationLogGetInput struct {
	ID string
}

// Get 获取操作日志详情
func (s *OperationLogService) Get(ctx context.Context, in *OperationLogGetInput) (*model.ConsoleOperationLog, error) {
	db, dbErr := s.defaultDB(ctx)
	if dbErr != nil {
		return nil, dbErr
	}
	var log model.ConsoleOperationLog
	if err := db.First(&log, "id = ?", in.ID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errx.NotFound().WithMessage("操作日志不存在")
		}
		return nil, errx.Internal().WithCause(err)
	}
	return &log, nil
}

// DeleteInput 删除输入
type OperationLogDeleteInput struct {
	ID string
}

// Delete 删除操作日志
func (s *OperationLogService) Delete(ctx context.Context, in *OperationLogDeleteInput) error {
	db, err := s.defaultDB(ctx)
	if err != nil {
		return err
	}
	if err := db.Delete(&model.ConsoleOperationLog{}, "id = ?", in.ID).Error; err != nil {
		return errx.Internal().WithCause(err)
	}
	return nil
}

// ClearInput 清空输入
type OperationLogClearInput struct {
	Days int // 保留天数，0表示全部删除
}

// Clear 清空操作日志
func (s *OperationLogService) Clear(ctx context.Context, in *OperationLogClearInput) error {
	db, err := s.defaultDB(ctx)
	if err != nil {
		return err
	}
	query := db.Model(&model.ConsoleOperationLog{})

	if in.Days > 0 {
		// 删除指定天数前的日志
		query = query.Where("created_at < DATE_SUB(NOW(), INTERVAL ? DAY)", in.Days)
	}

	if err := query.Delete(&model.ConsoleOperationLog{}).Error; err != nil {
		return errx.Internal().WithCause(err)
	}
	return nil
}

func (s *OperationLogService) defaultDB(ctx context.Context) (*gorm.DB, error) {
	if s.dbRepo == nil || s.dbRepo.Default() == nil {
		return nil, errx.ServiceUnavailable().WithMessage("默认数据库未配置")
	}
	return s.dbRepo.Default().WithContext(ctx), nil
}
