package service

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/zhimma/grove/internal/model"
	"github.com/zhimma/grove/internal/provider"
	pkgErr "github.com/zhimma/grove/pkg/errors"
)

// LoginLogService 登录日志服务
type LoginLogService struct {
	provider *provider.Provider
}

// NewLoginLogService 创建登录日志服务
func NewLoginLogService(p *provider.Provider) *LoginLogService {
	return &LoginLogService{provider: p}
}

// ListInput 列表输入参数
type LoginLogListInput struct {
	AdminID   string // 管理员ID
	IP        string // IP地址
	Status    int    // 状态：1成功 2失败
	StartTime string // 开始时间
	EndTime   string // 结束时间
	Page      int
	PageSize  int
}

// ListOutput 列表输出
type LoginLogListOutput struct {
	List  []model.ConsoleLoginLog
	Total int64
}

// List 获取登录日志列表
func (s *LoginLogService) List(ctx context.Context, in *LoginLogListInput) (*LoginLogListOutput, error) {
	db := s.provider.DB.Default().WithContext(ctx).Model(&model.ConsoleLoginLog{})

	// 筛选条件
	if in.AdminID != "" {
		db = db.Where("admin_id = ?", in.AdminID)
	}
	if in.IP != "" {
		db = db.Where("ip LIKE ?", "%"+in.IP+"%")
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
	var logs []model.ConsoleLoginLog
	offset := (in.Page - 1) * in.PageSize
	if err := db.Order("created_at DESC").Offset(offset).Limit(in.PageSize).Find(&logs).Error; err != nil {
		return nil, pkgErr.Internal().WithCause(err)
	}

	return &LoginLogListOutput{
		List:  logs,
		Total: total,
	}, nil
}

// GetInput 获取详情输入
type LoginLogGetInput struct {
	ID string
}

// Get 获取登录日志详情
func (s *LoginLogService) Get(ctx context.Context, in *LoginLogGetInput) (*model.ConsoleLoginLog, error) {
	var log model.ConsoleLoginLog
	if err := s.provider.DB.Default().WithContext(ctx).First(&log, "id = ?", in.ID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, pkgErr.NotFound().WithMessage("登录日志不存在")
		}
		return nil, pkgErr.Internal().WithCause(err)
	}
	return &log, nil
}

// DeleteInput 删除输入
type LoginLogDeleteInput struct {
	ID string
}

// Delete 删除登录日志
func (s *LoginLogService) Delete(ctx context.Context, in *LoginLogDeleteInput) error {
	if err := s.provider.DB.Default().WithContext(ctx).Delete(&model.ConsoleLoginLog{}, "id = ?", in.ID).Error; err != nil {
		return pkgErr.Internal().WithCause(err)
	}
	return nil
}

// ClearInput 清空输入
type LoginLogClearInput struct {
	Days int // 保留天数，0表示全部删除
}

// Clear 清空登录日志
func (s *LoginLogService) Clear(ctx context.Context, in *LoginLogClearInput) error {
	db := s.provider.DB.Default().WithContext(ctx).Model(&model.ConsoleLoginLog{})

	if in.Days > 0 {
		// 删除指定天数前的日志
		db = db.Where("created_at < DATE_SUB(NOW(), INTERVAL ? DAY)", in.Days)
	}

	if err := db.Delete(&model.ConsoleLoginLog{}).Error; err != nil {
		return pkgErr.Internal().WithCause(err)
	}
	return nil
}
