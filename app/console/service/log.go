package service

import (
	"context"
	"encoding/json"
	"strings"

	"gorm.io/gorm"

	"github.com/zhimma/grove/internal/model"
	pkgerrors "github.com/zhimma/grove/pkg/errors"
)

type LogService struct {
	db *gorm.DB
}

type ListOperationLogsInput struct {
	Page        int
	PageSize    int
	Offset      int
	Limit       int
	ListAll     bool
	Keyword     string
	OrderBy     []string
	Method      string
	Module      string
	Success     *bool
	AdminID     string
	CreatedFrom string
	CreatedTo   string
}

type ListOperationLogsOutput struct {
	List []model.ConsoleOperationLog
	Meta ListMeta
}

type GetOperationLogDetailInput struct {
	LogID string
}

type ListLoginLogsInput struct {
	Page        int
	PageSize    int
	Offset      int
	Limit       int
	ListAll     bool
	Keyword     string
	OrderBy     []string
	Success     *bool
	AdminID     string
	CreatedFrom string
	CreatedTo   string
}

type ListLoginLogsOutput struct {
	List []model.ConsoleLoginLog
	Meta ListMeta
}

type OperationLogDetail struct {
	Log    *model.ConsoleOperationLog
	Detail map[string]any
}

func NewLogService(db *gorm.DB) *LogService {
	return &LogService{db: db}
}

func (s *LogService) ListOperationLogs(ctx context.Context, in ListOperationLogsInput) (*ListOperationLogsOutput, error) {
	if s.db == nil {
		return nil, pkgerrors.ServiceUnavailable().WithMessage("默认数据库未配置")
	}

	query := s.db.WithContext(ctx).Model(&model.ConsoleOperationLog{}).Preload("Operator")
	if keyword := strings.TrimSpace(in.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("path LIKE ? OR route LIKE ? OR action LIKE ? OR error_message LIKE ?", like, like, like, like)
	}
	if method := strings.TrimSpace(strings.ToUpper(in.Method)); method != "" {
		query = query.Where("method = ?", method)
	}
	if module := strings.TrimSpace(in.Module); module != "" {
		query = query.Where("module = ?", module)
	}
	if in.Success != nil {
		query = query.Where("success = ?", *in.Success)
	}
	if adminID := strings.TrimSpace(in.AdminID); adminID != "" {
		query = query.Where("admin_id = ?", adminID)
	}

	var err error
	query, err = applyTimeRange(query, "created_at", in.CreatedFrom, in.CreatedTo)
	if err != nil {
		return nil, pkgerrors.InvalidParams().WithMessage("时间范围格式不正确")
	}

	return queryConsoleLogs(query, in.Page, in.PageSize, in.Offset, in.Limit, in.ListAll, in.OrderBy, func(db *gorm.DB) *gorm.DB {
		return db.Order("created_at DESC")
	}, func(result []model.ConsoleOperationLog, meta ListMeta) *ListOperationLogsOutput {
		return &ListOperationLogsOutput{List: result, Meta: meta}
	})
}

func (s *LogService) ListLoginLogs(ctx context.Context, in ListLoginLogsInput) (*ListLoginLogsOutput, error) {
	if s.db == nil {
		return nil, pkgerrors.ServiceUnavailable().WithMessage("默认数据库未配置")
	}

	query := s.db.WithContext(ctx).Model(&model.ConsoleLoginLog{}).Preload("Operator")
	if keyword := strings.TrimSpace(in.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("account LIKE ? OR failure_reason LIKE ?", like, like)
	}
	if in.Success != nil {
		query = query.Where("success = ?", *in.Success)
	}
	if adminID := strings.TrimSpace(in.AdminID); adminID != "" {
		query = query.Where("admin_id = ?", adminID)
	}

	var err error
	query, err = applyTimeRange(query, "created_at", in.CreatedFrom, in.CreatedTo)
	if err != nil {
		return nil, pkgerrors.InvalidParams().WithMessage("时间范围格式不正确")
	}

	return queryConsoleLogs(query, in.Page, in.PageSize, in.Offset, in.Limit, in.ListAll, in.OrderBy, func(db *gorm.DB) *gorm.DB {
		return db.Order("created_at DESC")
	}, func(result []model.ConsoleLoginLog, meta ListMeta) *ListLoginLogsOutput {
		return &ListLoginLogsOutput{List: result, Meta: meta}
	})
}

func (s *LogService) GetOperationLogDetail(ctx context.Context, in GetOperationLogDetailInput) (*OperationLogDetail, error) {
	if s.db == nil {
		return nil, pkgerrors.ServiceUnavailable().WithMessage("默认数据库未配置")
	}

	var item model.ConsoleOperationLog
	if err := s.db.WithContext(ctx).Preload("Operator").First(&item, "id = ?", strings.TrimSpace(in.LogID)).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, pkgerrors.NotFound().WithMessage("操作日志不存在")
		}
		return nil, pkgerrors.Internal().WithCause(err)
	}

	detail := map[string]any{}
	if strings.TrimSpace(item.DetailJSON) != "" {
		if err := json.Unmarshal([]byte(item.DetailJSON), &detail); err != nil {
			detail = map[string]any{
				"_raw": item.DetailJSON,
			}
		}
	}
	return &OperationLogDetail{
		Log:    &item,
		Detail: detail,
	}, nil
}

func queryConsoleLogs[T any, R any](
	query *gorm.DB,
	page int,
	pageSize int,
	offset int,
	limit int,
	listAll bool,
	orderBy []string,
	defaultOrder func(*gorm.DB) *gorm.DB,
	assemble func([]T, ListMeta) R,
) (R, error) {
	var zero R

	page, pageSize = resolvePage(ListRequest{
		Page:     page,
		PageSize: pageSize,
		Offset:   offset,
		Limit:    limit,
		ListAll:  listAll,
	})

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return zero, pkgerrors.Internal().WithCause(err)
	}

	if len(orderBy) == 0 {
		query = defaultOrder(query)
	} else {
		for _, item := range orderBy {
			field, direction := parseOrderBy(item)
			switch field {
			case "created_at", "updated_at", "status_code", "duration_ms", "method", "module", "account":
				query = query.Order(field + " " + direction)
			}
		}
	}

	if !listAll {
		resolvedOffset := offset
		if resolvedOffset <= 0 {
			resolvedOffset = (page - 1) * pageSize
		}
		query = query.Offset(resolvedOffset).Limit(pageSize)
	}

	var list []T
	if err := query.Find(&list).Error; err != nil {
		return zero, pkgerrors.Internal().WithCause(err)
	}
	return assemble(list, NewListMeta(total, page, pageSize)), nil
}
