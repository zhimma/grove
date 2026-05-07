package handler

import (
	"github.com/gin-gonic/gin"

	consoleservice "github.com/zhimma/grove/app/console/service"
	"github.com/zhimma/grove/internal/provider"
	"github.com/zhimma/grove/pkg/response"
	"github.com/zhimma/grove/pkg/route"
	"github.com/zhimma/grove/pkg/validation"
)

type SystemConfigHandler struct {
	service *consoleservice.SystemConfigService
}

type ListSystemConfigsRequest struct {
	Page        int      `form:"page" binding:"omitempty,min=1" label:"页码"`
	PageSize    int      `form:"page_size" binding:"omitempty,min=1,max=100" label:"每页条数"`
	Offset      int      `form:"offset"`
	Limit       int      `form:"limit"`
	ListAll     bool     `form:"list_all"`
	Keyword     string   `form:"keyword" label:"关键词"`
	OrderBy     []string `form:"order_by"`
	ConfigGroup string   `form:"config_group" label:"配置分组"`
	IsEditable  *bool    `form:"is_editable" label:"是否可编辑"`
	CreatedFrom string   `form:"created_from"`
	CreatedTo   string   `form:"created_to"`
}

type ListSystemConfigsResponse struct {
	List []SystemConfigItem `json:"list"`
	Meta ListMeta           `json:"meta"`
}

type SystemConfigItem struct {
	ID           string `json:"id"`
	ConfigGroup  string `json:"config_group"`
	ConfigKey    string `json:"config_key"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	ValueType    string `json:"value_type"`
	Value        string `json:"value"`
	DefaultValue string `json:"default_value"`
	IsEditable   bool   `json:"is_editable"`
	IsSystem     bool   `json:"is_system"`
	SortOrder    int    `json:"sort_order"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

type CreateSystemConfigRequest struct {
	ConfigGroup  string `json:"config_group" binding:"required" label:"配置分组"`
	ConfigKey    string `json:"config_key" binding:"required" label:"配置键"`
	Name         string `json:"name" label:"配置名称"`
	Description  string `json:"description" label:"配置描述"`
	ValueType    string `json:"value_type" label:"值类型"`
	Value        string `json:"value" label:"配置值"`
	DefaultValue string `json:"default_value" label:"默认值"`
	IsEditable   bool   `json:"is_editable" label:"是否可编辑"`
	IsSystem     bool   `json:"is_system" label:"是否系统配置"`
	SortOrder    int    `json:"sort_order" label:"排序"`
}

type UpdateSystemConfigRequest struct {
	Value string `json:"value" label:"配置值"`
}

type SystemConfigPathRequest struct {
	ID string `uri:"id" binding:"required" label:"配置ID"`
}

type SystemConfigGroupPathRequest struct {
	Group string `uri:"group" binding:"required" label:"配置分组"`
}

func RegisterSystemConfigRoutes(protected *gin.RouterGroup, p *provider.Provider) {
	h := &SystemConfigHandler{
		service: consoleservice.NewSystemConfigService(p.DB.Default()),
	}

	group := route.Wrap(protected.Group("/system-configs"))
	group.GET("", h.List).Name("配置管理.系统配置列表")
	group.GET("/groups/:group", h.ListGroup).Name("配置管理.系统配置分组")
	group.POST("", h.Create).Name("配置管理.创建系统配置")
	group.PUT("/:id", h.Update).Name("配置管理.更新系统配置")
	group.DELETE("/:id", h.Delete).Name("配置管理.删除系统配置")
}

func (h *SystemConfigHandler) List(c *gin.Context) {
	var req ListSystemConfigsRequest
	if err := validation.BindQuery(c, &req); err != nil {
		response.Fail(c, err)
		return
	}
	result, err := h.service.ListConfigs(c.Request.Context(), consoleservice.ListSystemConfigsInput{
		Page:        req.Page,
		PageSize:    req.PageSize,
		Offset:      req.Offset,
		Limit:       req.Limit,
		ListAll:     req.ListAll,
		Keyword:     req.Keyword,
		OrderBy:     req.OrderBy,
		ConfigGroup: req.ConfigGroup,
		IsEditable:  req.IsEditable,
		CreatedFrom: req.CreatedFrom,
		CreatedTo:   req.CreatedTo,
	})
	if err != nil {
		response.Fail(c, err)
		return
	}

	items := make([]SystemConfigItem, 0, len(result.List))
	for _, item := range result.List {
		items = append(items, SystemConfigItem{
			ID:           item.ID,
			ConfigGroup:  item.ConfigGroup,
			ConfigKey:    item.ConfigKey,
			Name:         item.Name,
			Description:  item.Description,
			ValueType:    item.ValueType,
			Value:        item.Value,
			DefaultValue: item.DefaultValue,
			IsEditable:   item.IsEditable,
			IsSystem:     item.IsSystem,
			SortOrder:    item.SortOrder,
			CreatedAt:    item.CreatedAt.Format("2006-01-02 15:04:05"),
			UpdatedAt:    item.UpdatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	response.Success(c, ListSystemConfigsResponse{
		List: items,
		Meta: ListMeta(result.Meta),
	})
}

func (h *SystemConfigHandler) ListGroup(c *gin.Context) {
	var req SystemConfigGroupPathRequest
	if err := validation.BindURI(c, &req); err != nil {
		response.Fail(c, err)
		return
	}
	result, err := h.service.GetGroupConfigs(c.Request.Context(), consoleservice.GetGroupConfigsInput{
		Group: req.Group,
	})
	if err != nil {
		response.Fail(c, err)
		return
	}

	items := make([]SystemConfigItem, 0, len(result))
	for _, item := range result {
		items = append(items, SystemConfigItem{
			ID:           item.ID,
			ConfigGroup:  item.ConfigGroup,
			ConfigKey:    item.ConfigKey,
			Name:         item.Name,
			Description:  item.Description,
			ValueType:    item.ValueType,
			Value:        item.Value,
			DefaultValue: item.DefaultValue,
			IsEditable:   item.IsEditable,
			IsSystem:     item.IsSystem,
			SortOrder:    item.SortOrder,
			CreatedAt:    item.CreatedAt.Format("2006-01-02 15:04:05"),
			UpdatedAt:    item.UpdatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	response.Success(c, items)
}

func (h *SystemConfigHandler) Create(c *gin.Context) {
	var req CreateSystemConfigRequest
	if err := validation.BindJSON(c, &req); err != nil {
		response.Fail(c, err)
		return
	}
	result, err := h.service.CreateConfig(c.Request.Context(), consoleservice.CreateSystemConfigInput{
		ConfigGroup:  req.ConfigGroup,
		ConfigKey:    req.ConfigKey,
		Name:         req.Name,
		Description:  req.Description,
		ValueType:    req.ValueType,
		Value:        req.Value,
		DefaultValue: req.DefaultValue,
		IsEditable:   req.IsEditable,
		IsSystem:     req.IsSystem,
		SortOrder:    req.SortOrder,
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	setAuditMeta(c, "system_config", result.ID, map[string]any{
		"config_group": result.ConfigGroup,
		"config_key":   result.ConfigKey,
		"value_type":   result.ValueType,
		"value":        result.Value,
	})
	response.Success(c, SystemConfigItem{
		ID:           result.ID,
		ConfigGroup:  result.ConfigGroup,
		ConfigKey:    result.ConfigKey,
		Name:         result.Name,
		Description:  result.Description,
		ValueType:    result.ValueType,
		Value:        result.Value,
		DefaultValue: result.DefaultValue,
		IsEditable:   result.IsEditable,
		IsSystem:     result.IsSystem,
		SortOrder:    result.SortOrder,
		CreatedAt:    result.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:    result.UpdatedAt.Format("2006-01-02 15:04:05"),
	})
}

func (h *SystemConfigHandler) Update(c *gin.Context) {
	var pathReq SystemConfigPathRequest
	if err := validation.BindURI(c, &pathReq); err != nil {
		response.Fail(c, err)
		return
	}
	var req UpdateSystemConfigRequest
	if err := validation.BindJSON(c, &req); err != nil {
		response.Fail(c, err)
		return
	}
	result, err := h.service.UpdateConfigByID(c.Request.Context(), consoleservice.UpdateSystemConfigByIDInput{
		ID:    pathReq.ID,
		Value: req.Value,
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	setAuditMeta(c, "system_config", result.ID, map[string]any{
		"config_group": result.ConfigGroup,
		"config_key":   result.ConfigKey,
		"value":        result.Value,
	})
	response.Success(c, SystemConfigItem{
		ID:           result.ID,
		ConfigGroup:  result.ConfigGroup,
		ConfigKey:    result.ConfigKey,
		Name:         result.Name,
		Description:  result.Description,
		ValueType:    result.ValueType,
		Value:        result.Value,
		DefaultValue: result.DefaultValue,
		IsEditable:   result.IsEditable,
		IsSystem:     result.IsSystem,
		SortOrder:    result.SortOrder,
		CreatedAt:    result.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:    result.UpdatedAt.Format("2006-01-02 15:04:05"),
	})
}

func (h *SystemConfigHandler) Delete(c *gin.Context) {
	var req SystemConfigPathRequest
	if err := validation.BindURI(c, &req); err != nil {
		response.Fail(c, err)
		return
	}
	if err := h.service.DeleteConfig(c.Request.Context(), req.ID); err != nil {
		response.Fail(c, err)
		return
	}
	setAuditMeta(c, "system_config", req.ID, map[string]any{
		"deleted": true,
	})
	response.Success(c, nil)
}
