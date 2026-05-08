package service

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/zhimma/grove/internal/model"
	"github.com/zhimma/grove/pkg/database"
	pkgErr "github.com/zhimma/grove/pkg/errors"
)

// LoginLogService 登录日志服务
type LoginLogService struct {
	dbRepo database.Repo
}

// NewLoginLogService 创建登录日志服务
func NewLoginLogService(dbRepo database.Repo) *LoginLogService {
	return &LoginLogService{dbRepo: dbRepo}
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
	db, err := s.defaultDB(ctx)
	if err != nil {
		return nil, err
	}
	query := db.Model(&model.ConsoleLoginLog{})

	// 筛选条件
	if in.AdminID != "" {
		query = query.Where("admin_id = ?", in.AdminID)
	}
	if in.IP != "" {
		query = query.Where("ip LIKE ?", "%"+in.IP+"%")
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
		return nil, pkgErr.Internal().WithCause(err)
	}

	// 分页查询
	var logs []model.ConsoleLoginLog
	offset := (in.Page - 1) * in.PageSize
	if err := query.Order("created_at DESC").Offset(offset).Limit(in.PageSize).Find(&logs).Error; err != nil {
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
	db, dbErr := s.defaultDB(ctx)
	if dbErr != nil {
		return nil, dbErr
	}
	var log model.ConsoleLoginLog
	if err := db.First(&log, "id = ?", in.ID).Error; err != nil {
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
	db, err := s.defaultDB(ctx)
	if err != nil {
		return err
	}
	if err := db.Delete(&model.ConsoleLoginLog{}, "id = ?", in.ID).Error; err != nil {
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
	db, err := s.defaultDB(ctx)
	if err != nil {
		return err
	}
	query := db.Model(&model.ConsoleLoginLog{})

	if in.Days > 0 {
		// 删除指定天数前的日志
		query = query.Where("created_at < DATE_SUB(NOW(), INTERVAL ? DAY)", in.Days)
	}

	if err := query.Delete(&model.ConsoleLoginLog{}).Error; err != nil {
		return pkgErr.Internal().WithCause(err)
	}
	return nil
}

func (s *LoginLogService) defaultDB(ctx context.Context) (*gorm.DB, error) {
	if s.dbRepo == nil || s.dbRepo.Default() == nil {
		return nil, pkgErr.ServiceUnavailable().WithMessage("默认数据库未配置")
	}
	return s.dbRepo.Default().WithContext(ctx), nil
}
