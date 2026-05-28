package service

import (
	"context"
	"encoding/json"
	"strings"

	"gorm.io/gorm"

	"github.com/zhimma/grove/internal/model"
	"github.com/zhimma/grove/pkg/database"
	"github.com/zhimma/grove/pkg/errx"
)

type SystemConfigService struct {
	dbRepo database.Repo
}

type ListSystemConfigsInput struct {
	Page        int
	PageSize    int
	Offset      int
	Limit       int
	ListAll     bool
	Keyword     string
	OrderBy     []string
	ConfigGroup string
	IsEditable  *bool
	CreatedFrom string
	CreatedTo   string
}

type ListSystemConfigsOutput struct {
	List []model.SystemConfig
	Meta ListMeta
}

type CreateSystemConfigInput struct {
	ConfigGroup  string
	ConfigKey    string
	Name         string
	Description  string
	ValueType    string
	Value        string
	DefaultValue string
	IsEditable   bool
	IsSystem     bool
	SortOrder    int
}

type UpdateSystemConfigByIDInput struct {
	ID    string
	Value string
}

type GetGroupConfigsInput struct {
	Group string
}

func NewSystemConfigService(dbRepo database.Repo) *SystemConfigService {
	return &SystemConfigService{dbRepo: dbRepo}
}

func (s *SystemConfigService) ListConfigs(ctx context.Context, in ListSystemConfigsInput) (*ListSystemConfigsOutput, error) {
	db, err := s.defaultDB(ctx)
	if err != nil {
		return nil, err
	}

	query := db.Model(&model.SystemConfig{})
	if group := strings.TrimSpace(in.ConfigGroup); group != "" {
		query = query.Where("config_group = ?", group)
	}
	if keyword := strings.TrimSpace(in.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("config_group LIKE ? OR config_key LIKE ? OR name LIKE ? OR description LIKE ?", like, like, like, like)
	}
	if in.IsEditable != nil {
		query = query.Where("is_editable = ?", *in.IsEditable)
	}

	query, err = applyTimeRange(query, "created_at", in.CreatedFrom, in.CreatedTo)
	if err != nil {
		return nil, errx.InvalidParams().WithMessage("时间范围格式不正确")
	}

	page, pageSize := resolvePage(ListRequest{
		Page:     in.Page,
		PageSize: in.PageSize,
		Offset:   in.Offset,
		Limit:    in.Limit,
		ListAll:  in.ListAll,
	})

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, errx.Internal().WithCause(err)
	}

	if len(in.OrderBy) == 0 {
		query = query.Order("config_group ASC").Order("sort_order ASC").Order("created_at DESC")
	} else {
		for _, item := range in.OrderBy {
			field, direction := parseOrderBy(item)
			switch field {
			case "config_group", "sort_order", "created_at", "updated_at", "name":
				query = query.Order(field + " " + direction)
			}
		}
	}

	if !in.ListAll {
		offset := in.Offset
		if offset <= 0 {
			offset = (page - 1) * pageSize
		}
		query = query.Offset(offset).Limit(pageSize)
	}

	var list []model.SystemConfig
	if err := query.Find(&list).Error; err != nil {
		return nil, errx.Internal().WithCause(err)
	}
	return &ListSystemConfigsOutput{
		List: list,
		Meta: NewListMeta(total, page, pageSize),
	}, nil
}

func (s *SystemConfigService) GetGroupConfigs(ctx context.Context, in GetGroupConfigsInput) ([]model.SystemConfig, error) {
	db, err := s.defaultDB(ctx)
	if err != nil {
		return nil, err
	}

	var items []model.SystemConfig
	if err := db.
		Where("config_group = ?", strings.TrimSpace(in.Group)).
		Order("sort_order ASC, created_at ASC").
		Find(&items).Error; err != nil {
		return nil, errx.Internal().WithCause(err)
	}
	return items, nil
}

func (s *SystemConfigService) CreateConfig(ctx context.Context, in CreateSystemConfigInput) (*model.SystemConfig, error) {
	db, dbErr := s.defaultDB(ctx)
	if dbErr != nil {
		return nil, dbErr
	}

	record := model.SystemConfig{
		ConfigGroup:  strings.TrimSpace(in.ConfigGroup),
		ConfigKey:    strings.TrimSpace(in.ConfigKey),
		Name:         strings.TrimSpace(in.Name),
		Description:  strings.TrimSpace(in.Description),
		ValueType:    normalizeConfigValueType(in.ValueType),
		Value:        strings.TrimSpace(in.Value),
		DefaultValue: strings.TrimSpace(in.DefaultValue),
		IsEditable:   in.IsEditable,
		IsSystem:     in.IsSystem,
		SortOrder:    in.SortOrder,
	}
	if record.ConfigGroup == "" || record.ConfigKey == "" {
		return nil, errx.InvalidParams().WithHTTPStatus(422).WithMessage("配置分组和配置键不能为空")
	}
	if record.Name == "" {
		record.Name = record.ConfigKey
	}
	if err := validateSystemConfigValue(record.ValueType, record.Value); err != nil {
		return nil, err
	}
	if err := validateSystemConfigValue(record.ValueType, record.DefaultValue); err != nil {
		return nil, err
	}

	var count int64
	if err := db.Model(&model.SystemConfig{}).
		Where("config_group = ? AND config_key = ?", record.ConfigGroup, record.ConfigKey).
		Count(&count).Error; err != nil {
		return nil, errx.Internal().WithCause(err)
	}
	if count > 0 {
		return nil, errx.Conflict().WithCode("system_config_exists").WithMessage("系统配置已存在")
	}

	if err := db.Create(&record).Error; err != nil {
		return nil, errx.Internal().WithCause(err)
	}
	return &record, nil
}

func (s *SystemConfigService) UpdateConfigByID(ctx context.Context, in UpdateSystemConfigByIDInput) (*model.SystemConfig, error) {
	record, err := s.getByID(ctx, in.ID)
	if err != nil {
		return nil, err
	}
	if !record.IsEditable {
		return nil, errx.Forbidden().WithMessage("系统配置为只读，不允许修改")
	}
	value := strings.TrimSpace(in.Value)
	if err := validateSystemConfigValue(record.ValueType, value); err != nil {
		return nil, err
	}
	db, dbErr := s.defaultDB(ctx)
	if dbErr != nil {
		return nil, dbErr
	}
	if err := db.Model(&model.SystemConfig{}).
		Where("id = ?", record.ID).
		Update("value", value).Error; err != nil {
		return nil, errx.Internal().WithCause(err)
	}
	record.Value = value
	return record, nil
}

func (s *SystemConfigService) DeleteConfig(ctx context.Context, id string) error {
	record, err := s.getByID(ctx, id)
	if err != nil {
		return err
	}
	if record.IsSystem {
		return errx.Forbidden().WithMessage("系统配置不允许删除")
	}
	db, dbErr := s.defaultDB(ctx)
	if dbErr != nil {
		return dbErr
	}
	if err := db.Delete(&model.SystemConfig{}, "id = ?", record.ID).Error; err != nil {
		return errx.Internal().WithCause(err)
	}
	return nil
}

func (s *SystemConfigService) getByID(ctx context.Context, id string) (*model.SystemConfig, error) {
	db, err := s.defaultDB(ctx)
	if err != nil {
		return nil, err
	}
	var record model.SystemConfig
	if err := db.First(&record, "id = ?", strings.TrimSpace(id)).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errx.NotFound().WithMessage("系统配置不存在")
		}
		return nil, errx.Internal().WithCause(err)
	}
	return &record, nil
}

func (s *SystemConfigService) defaultDB(ctx context.Context) (*gorm.DB, error) {
	if s.dbRepo == nil || s.dbRepo.Default() == nil {
		return nil, errx.ServiceUnavailable().WithMessage("默认数据库未配置")
	}
	return s.dbRepo.Default().WithContext(ctx), nil
}

func normalizeConfigValueType(valueType string) string {
	switch strings.TrimSpace(strings.ToLower(valueType)) {
	case "array", "int", "bool", "json":
		return strings.TrimSpace(strings.ToLower(valueType))
	default:
		return "string"
	}
}

func validateSystemConfigValue(valueType, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	switch normalizeConfigValueType(valueType) {
	case "array":
		var target []any
		if err := json.Unmarshal([]byte(value), &target); err != nil {
			return errx.InvalidParams().WithHTTPStatus(422).WithMessage("配置值必须是合法的 JSON 数组")
		}
	case "int":
		_, err := (&model.SystemConfig{Value: value, ValueType: "int"}).IntValue()
		if err != nil {
			return errx.InvalidParams().WithHTTPStatus(422).WithMessage("配置值必须是整数")
		}
	case "bool":
		_, err := (&model.SystemConfig{Value: value, ValueType: "bool"}).BoolValue()
		if err != nil {
			return errx.InvalidParams().WithHTTPStatus(422).WithMessage("配置值必须是布尔值")
		}
	case "json":
		var target any
		if err := json.Unmarshal([]byte(value), &target); err != nil {
			return errx.InvalidParams().WithHTTPStatus(422).WithMessage("配置值必须是合法的 JSON")
		}
	}
	return nil
}
