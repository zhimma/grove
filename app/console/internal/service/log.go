package service

import (
	"context"
	"encoding/json"
	"strings"

	"gorm.io/gorm"

	"github.com/zhimma/grove/internal/model"
	"github.com/zhimma/grove/pkg/database"
	"github.com/zhimma/grove/pkg/errx"
	"github.com/zhimma/grove/pkg/pagination"
)

type LogService struct {
	dbs   *database.Connections
	pages pagination.Policy
}

type ListOperationLogsInput struct {
	pagination.Request
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
	Meta pagination.Meta
}

type GetOperationLogDetailInput struct {
	LogID string
}

type ListLoginLogsInput struct {
	pagination.Request
	Keyword     string
	OrderBy     []string
	Success     *bool
	AdminID     string
	CreatedFrom string
	CreatedTo   string
}

type ListLoginLogsOutput struct {
	List []model.ConsoleLoginLog
	Meta pagination.Meta
}

type OperationLogDetail struct {
	Log    *model.ConsoleOperationLog
	Detail map[string]any
}

func NewLogService(dbs *database.Connections, pages pagination.Policy) *LogService {
	return &LogService{dbs: dbs, pages: pages}
}

func (s *LogService) ListOperationLogs(ctx context.Context, in ListOperationLogsInput) (*ListOperationLogsOutput, error) {
	db, err := s.defaultDB(ctx)
	if err != nil {
		return nil, err
	}

	query := db.Model(&model.ConsoleOperationLog{}).Preload("Operator")
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

	query, err = applyTimeRange(query, "created_at", in.CreatedFrom, in.CreatedTo)
	if err != nil {
		return nil, errx.InvalidParams().WithMessage("时间范围格式不正确")
	}

	return queryConsoleLogs(query, s.pages.Resolve(in.Request), in.OrderBy, func(db *gorm.DB) *gorm.DB {
		return db.Order("created_at DESC")
	}, func(result []model.ConsoleOperationLog, meta pagination.Meta) *ListOperationLogsOutput {
		return &ListOperationLogsOutput{List: result, Meta: meta}
	})
}

func (s *LogService) ListLoginLogs(ctx context.Context, in ListLoginLogsInput) (*ListLoginLogsOutput, error) {
	db, err := s.defaultDB(ctx)
	if err != nil {
		return nil, err
	}

	query := db.Model(&model.ConsoleLoginLog{}).Preload("Operator")
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

	query, err = applyTimeRange(query, "created_at", in.CreatedFrom, in.CreatedTo)
	if err != nil {
		return nil, errx.InvalidParams().WithMessage("时间范围格式不正确")
	}

	return queryConsoleLogs(query, s.pages.Resolve(in.Request), in.OrderBy, func(db *gorm.DB) *gorm.DB {
		return db.Order("created_at DESC")
	}, func(result []model.ConsoleLoginLog, meta pagination.Meta) *ListLoginLogsOutput {
		return &ListLoginLogsOutput{List: result, Meta: meta}
	})
}

func (s *LogService) GetOperationLogDetail(ctx context.Context, in GetOperationLogDetailInput) (*OperationLogDetail, error) {
	db, dbErr := s.defaultDB(ctx)
	if dbErr != nil {
		return nil, dbErr
	}

	var item model.ConsoleOperationLog
	if err := db.Preload("Operator").First(&item, "id = ?", strings.TrimSpace(in.LogID)).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errx.NotFound().WithMessage("操作日志不存在")
		}
		return nil, errx.Internal().WithCause(err)
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
	page pagination.Page,
	orderBy []string,
	defaultOrder func(*gorm.DB) *gorm.DB,
	assemble func([]T, pagination.Meta) R,
) (R, error) {
	var zero R

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return zero, errx.Internal().WithCause(err)
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

	query = page.Apply(query)

	// Return an empty JSON array rather than null when no records match. The
	// console client treats list as a stable array in both list endpoints.
	list := make([]T, 0)
	if err := query.Find(&list).Error; err != nil {
		return zero, errx.Internal().WithCause(err)
	}
	return assemble(list, pagination.NewMeta(total, page)), nil
}

func (s *LogService) defaultDB(ctx context.Context) (*gorm.DB, error) {
	if s.dbs == nil || s.dbs.Default() == nil {
		return nil, errx.ServiceUnavailable().WithMessage("默认数据库未配置")
	}
	return s.dbs.Default().WithContext(ctx), nil
}
