package handler

import (
	"github.com/gin-gonic/gin"

	consoleservice "github.com/zhimma/grove/app/console/internal/service"
	"github.com/zhimma/grove/pkg/database"
	"github.com/zhimma/grove/pkg/pagination"
	"github.com/zhimma/grove/pkg/response"
	"github.com/zhimma/grove/pkg/route"
	"github.com/zhimma/grove/pkg/secretbox"
	"github.com/zhimma/grove/pkg/validation"
)

type SystemConfigHandler struct {
	service *consoleservice.SystemConfigService
}

type ListSystemConfigsRequest struct {
	ListQuery
	ConfigGroup string `form:"config_group" label:"配置分组"`
	IsEditable  *bool  `form:"is_editable" label:"是否可编辑"`
}

type ListSystemConfigsResponse struct {
	List []SystemConfigItem `json:"list"`
	Meta pagination.Meta    `json:"meta"`
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
	IsSecret     bool   `json:"is_secret" label:"是否敏感配置"`
	SortOrder    int    `json:"sort_order" label:"排序"`
}

type UpdateSystemConfigRequest struct {
	Value      string `json:"value" label:"配置值"`
	KeepSecret bool   `json:"keep_secret" label:"保持敏感值"`
}

type SystemConfigPathRequest struct {
	ID string `uri:"id" binding:"required" label:"配置ID"`
}

type SystemConfigGroupPathRequest struct {
	Group string `uri:"group" binding:"required" label:"配置分组"`
}

func RegisterSystemConfigRoutes(protected *gin.RouterGroup, dbs database.Connections, secrets *secretbox.Box, pages pagination.Policy, catalog *route.Catalog) {
	h := &SystemConfigHandler{
		service: consoleservice.NewSystemConfigService(dbs, secrets, pages),
	}

	group := wrapRoute(protected.Group("/system-configs"), catalog)
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
		Request:     req.Request,
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
		items = append(items, newSystemConfigItem(item))
	}
	response.Success(c, ListSystemConfigsResponse{
		List: items,
		Meta: result.Meta,
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
		items = append(items, newSystemConfigItem(item))
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
		IsSecret:     req.IsSecret,
		SortOrder:    req.SortOrder,
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	setAuditMeta(c, "system_config", result.ID, map[string]any{
		"config_key": result.ConfigKey,
		"changed":    true,
		"is_secret":  result.IsSecret,
	})
	response.Success(c, newSystemConfigItem(*result))
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
		ID:         pathReq.ID,
		Value:      req.Value,
		KeepSecret: req.KeepSecret,
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	setAuditMeta(c, "system_config", result.ID, map[string]any{
		"config_key": result.ConfigKey,
		"changed":    true,
		"is_secret":  result.IsSecret,
	})
	response.Success(c, newSystemConfigItem(*result))
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
