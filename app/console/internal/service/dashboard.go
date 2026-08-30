package service

import (
	"context"

	"github.com/zhimma/grove/pkg/database"
	"github.com/zhimma/grove/pkg/errx"
)

type DashboardService struct {
	dbs database.Connections
}

type SummaryOutput struct {
	AdminCount     int64  `json:"admin_count"`
	UserCount      int64  `json:"user_count"`
	RoleCount      int64  `json:"role_count"`
	OperationCount int64  `json:"operation_count"`
	LoginCount     int64  `json:"login_count"`
	Message        string `json:"message"`
}

func NewDashboardService(dbs database.Connections) *DashboardService {
	return &DashboardService{dbs: dbs}
}

func (s *DashboardService) Summary(ctx context.Context) (SummaryOutput, error) {
	if s.dbs == nil || s.dbs.Default() == nil {
		return SummaryOutput{}, errx.ServiceUnavailable().WithMessage("默认数据库未配置")
	}

	var (
		adminCount     int64
		userCount      int64
		roleCount      int64
		operationCount int64
		loginCount     int64
	)
	db := s.dbs.Default().WithContext(ctx)
	if err := db.Table("console_admins").Count(&adminCount).Error; err != nil {
		return SummaryOutput{}, errx.Internal().WithCause(err)
	}
	if err := db.Table("users").Count(&userCount).Error; err != nil {
		return SummaryOutput{}, errx.Internal().WithCause(err)
	}
	if err := db.Table("console_roles").Count(&roleCount).Error; err != nil {
		return SummaryOutput{}, errx.Internal().WithCause(err)
	}
	if err := db.Table("console_operation_logs").Count(&operationCount).Error; err != nil {
		return SummaryOutput{}, errx.Internal().WithCause(err)
	}
	if err := db.Table("console_login_logs").Count(&loginCount).Error; err != nil {
		return SummaryOutput{}, errx.Internal().WithCause(err)
	}

	return SummaryOutput{
		AdminCount:     adminCount,
		UserCount:      userCount,
		RoleCount:      roleCount,
		OperationCount: operationCount,
		LoginCount:     loginCount,
		Message:        "console permissions are route-driven",
	}, nil
}
