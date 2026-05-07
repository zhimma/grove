package service

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/zhimma/grove/internal/model"
	"github.com/zhimma/grove/internal/provider"
	pkgErr "github.com/zhimma/grove/pkg/errors"
)

// OperationLogService 操作日志服务
type OperationLogService struct {
	provider *provider.Provider
}

// NewOperationLogService 创建操作日志服务
func NewOperationLogService(p *provider.Provider) *OperationLogService {
	return &OperationLogService{provider: p}
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
	db := s.provider.DB.Default().WithContext(ctx).Model(&model.ConsoleOperationLog{})

	// 筛选条件
	if in.AdminID != "" {
		db = db.Where("admin_id = ?", in.AdminID)
	}
	if in.Action != "" {
		db = db.Where("action LIKE ?", "%"+in.Action+"%")
	}
	if in.Method != "" {
		db = db.Where("method = ?", in.Method)
	}
	if in.Path != "" {
		db = db.Where("path LIKE ?", "%"+in.Path+"%")
	}
	if in.Status > 0 {
		db = db.Where("status = ?", in.Status)
	}
	if in.StartTime != "" {
		db = db.Where("created_at >= ?", in.StartTime)
	}
	if in.EndTime != "" {
		db = db.Where("created_at <= ?", in.EndTime)
	}

	// 统计总数
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, pkgErr.Internal().WithCause(err)
	}

	// 分页查询
	var logs []model.ConsoleOperationLog
	offset := (in.Page - 1) * in.PageSize
	if err := db.Order("created_at DESC").Offset(offset).Limit(in.PageSize).Find(&logs).Error; err != nil {
		return nil, pkgErr.Internal().WithCause(err)
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
	var log model.ConsoleOperationLog
	if err := s.provider.DB.Default().WithContext(ctx).First(&log, "id = ?", in.ID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, pkgErr.NotFound().WithMessage("操作日志不存在")
		}
		return nil, pkgErr.Internal().WithCause(err)
	}
	return &log, nil
}

// DeleteInput 删除输入
type OperationLogDeleteInput struct {
	ID string
}

// Delete 删除操作日志
func (s *OperationLogService) Delete(ctx context.Context, in *OperationLogDeleteInput) error {
	if err := s.provider.DB.Default().WithContext(ctx).Delete(&model.ConsoleOperationLog{}, "id = ?", in.ID).Error; err != nil {
		return pkgErr.Internal().WithCause(err)
	}
	return nil
}

// ClearInput 清空输入
type OperationLogClearInput struct {
	Days int // 保留天数，0表示全部删除
}

// Clear 清空操作日志
func (s *OperationLogService) Clear(ctx context.Context, in *OperationLogClearInput) error {
	db := s.provider.DB.Default().WithContext(ctx).Model(&model.ConsoleOperationLog{})

	if in.Days > 0 {
		// 删除指定天数前的日志
		db = db.Where("created_at < DATE_SUB(NOW(), INTERVAL ? DAY)", in.Days)
	}

	if err := db.Delete(&model.ConsoleOperationLog{}).Error; err != nil {
		return pkgErr.Internal().WithCause(err)
	}
	return nil
}
