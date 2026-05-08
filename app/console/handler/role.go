package handler

import (
	"github.com/gin-gonic/gin"

	consoleservice "github.com/zhimma/grove/app/console/service"
	"github.com/zhimma/grove/internal/provider"
	"github.com/zhimma/grove/pkg/response"
	"github.com/zhimma/grove/pkg/route"
	"github.com/zhimma/grove/pkg/validation"
)

type RoleHandler struct {
	roleSvc *consoleservice.RoleService
}

type ListRolesRequest struct {
	Page        int      `form:"page" binding:"omitempty,min=1" label:"页码"`
	PageSize    int      `form:"page_size" binding:"omitempty,min=1,max=100" label:"每页条数"`
	Offset      int      `form:"offset" label:"偏移量"`
	Limit       int      `form:"limit" label:"限制条数"`
	ListAll     bool     `form:"list_all" label:"是否返回全部"`
	Keyword     string   `form:"keyword" label:"关键词"`
	OrderBy     []string `form:"order_by" label:"排序字段"`
	Status      *int     `form:"status" label:"状态"`
	CreatedFrom string   `form:"created_from" label:"创建开始时间"`
	CreatedTo   string   `form:"created_to" label:"创建结束时间"`
}

type ListRolesResponse struct {
	List []RoleItem `json:"list"`
	Meta ListMeta   `json:"meta"`
}

type RoleItem struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Code        string `json:"code"`
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
	Sort        int    `json:"sort"`
	Status      int    `json:"status"`
	StatusText  string `json:"status_text"`
	IsSuper     bool   `json:"is_super"`
	CreatedAt   string `json:"created_at"`
}

type RoleDetail struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Code        string `json:"code"`
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
	Sort        int    `json:"sort"`
	Status      int    `json:"status"`
	StatusText  string `json:"status_text"`
	IsSuper     bool   `json:"is_super"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type CreateRoleRequest struct {
	Name        string `json:"name" binding:"required" label:"角色名称"`
	Code        string `json:"code" binding:"required" label:"角色编码"`
	DisplayName string `json:"display_name" label:"显示名称"`
	Description string `json:"description" label:"角色描述"`
	Sort        int    `json:"sort" label:"排序"`
	Status      int    `json:"status" label:"状态"`
}

type UpdateRoleRequest struct {
	Name        *string `json:"name" binding:"omitempty" label:"角色名称"`
	Code        *string `json:"code" binding:"omitempty" label:"角色编码"`
	DisplayName *string `json:"display_name" binding:"omitempty" label:"显示名称"`
	Description *string `json:"description" binding:"omitempty" label:"角色描述"`
	Status      *int    `json:"status" binding:"omitempty" label:"状态"`
	Sort        *int    `json:"sort" binding:"omitempty" label:"排序"`
}

type AssignPermissionsRequest struct {
	APIPermissions []string `json:"api_permissions" binding:"required" label:"接口权限"`
}

type AssignMenusRequest struct {
	MenuKeys []string `json:"menu_keys" binding:"required" label:"菜单权限"`
}

type RolePathRequest struct {
	ID string `uri:"id" binding:"required" label:"角色ID"`
}

func RegisterRoleRoutes(protected *gin.RouterGroup, p *provider.Provider, runtimeCatalog *consoleservice.RuntimePermissionCatalog) {
	h := &RoleHandler{
		roleSvc: consoleservice.NewRoleService(p.DB, p.GetEnforcer("console"), runtimeCatalog).WithTransaction(p.TxManager),
	}

	roles := route.Wrap(protected.Group("/roles"))
	roles.GET("", h.List).Name("角色权限.角色列表")
	roles.GET("/:id", h.Detail).Name("角色权限.角色详情")
	roles.POST("", h.Create).Name("角色权限.创建角色")
	roles.PUT("/:id", h.Update).Name("角色权限.更新角色")
	roles.DELETE("/:id", h.Delete).Name("角色权限.删除角色")
	roles.GET("/:id/permissions", h.Permissions).Name("角色权限.查看接口权限")
	roles.POST("/:id/permissions", h.AssignPermissions).Name("角色权限.配置接口权限")
	roles.GET("/:id/menus", h.Menus).Name("角色权限.查看菜单权限")
	roles.POST("/:id/menus", h.AssignMenus).Name("角色权限.配置菜单权限")
}

func (h *RoleHandler) List(c *gin.Context) {
	var req ListRolesRequest
	if err := validation.BindQuery(c, &req); err != nil {
		response.Fail(c, err)
		return
	}

	result, err := h.roleSvc.ListRoles(c.Request.Context(), consoleservice.ListRolesInput{
		Page:        req.Page,
		PageSize:    req.PageSize,
		Offset:      req.Offset,
		Limit:       req.Limit,
		ListAll:     req.ListAll,
		Keyword:     req.Keyword,
		OrderBy:     req.OrderBy,
		Status:      req.Status,
		CreatedFrom: req.CreatedFrom,
		CreatedTo:   req.CreatedTo,
	})
	if err != nil {
		response.Fail(c, err)
		return
	}

	items := make([]RoleItem, 0, len(result.List))
	for _, role := range result.List {
		items = append(items, RoleItem{
			ID:          role.ID,
			Name:        role.Name,
			Code:        role.Code,
			DisplayName: role.DisplayName,
			Description: role.Description,
			Sort:        role.Sort,
			Status:      role.Status,
			StatusText:  roleStatusToText(role.Status),
			IsSuper:     role.IsSuper,
			CreatedAt:   role.CreatedAt,
		})
	}

	response.Success(c, ListRolesResponse{
		List: items,
		Meta: ListMeta(result.Meta),
	})
}

func (h *RoleHandler) Detail(c *gin.Context) {
	var req RolePathRequest
	if err := validation.BindURI(c, &req); err != nil {
		response.Fail(c, err)
		return
	}

	role, err := h.roleSvc.GetRole(c.Request.Context(), consoleservice.GetRoleInput{RoleID: req.ID})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, RoleDetail{
		ID:          role.ID,
		Name:        role.Name,
		Code:        role.Code,
		DisplayName: role.DisplayName,
		Description: role.Description,
		Sort:        role.Sort,
		Status:      role.Status,
		StatusText:  roleStatusToText(role.Status),
		IsSuper:     role.IsSuper,
		CreatedAt:   role.CreatedAt,
		UpdatedAt:   role.UpdatedAt,
	})
}

func (h *RoleHandler) Create(c *gin.Context) {
	var req CreateRoleRequest
	if err := validation.BindJSON(c, &req); err != nil {
		response.Fail(c, err)
		return
	}

	role, err := h.roleSvc.CreateRole(c.Request.Context(), consoleservice.CreateRoleInput{
		Name:        req.Name,
		Code:        req.Code,
		DisplayName: req.DisplayName,
		Description: req.Description,
		Sort:        req.Sort,
		Status:      req.Status,
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	setAuditMeta(c, "console_role", role.ID, map[string]any{
		"name":         role.Name,
		"code":         role.Code,
		"display_name": role.DisplayName,
		"status":       role.Status,
		"sort":         role.Sort,
	})
	response.Success(c, RoleDetail{
		ID:          role.ID,
		Name:        role.Name,
		Code:        role.Code,
		DisplayName: role.DisplayName,
		Description: role.Description,
		Sort:        role.Sort,
		Status:      role.Status,
		StatusText:  roleStatusToText(role.Status),
		IsSuper:     role.IsSuper,
		CreatedAt:   role.CreatedAt,
		UpdatedAt:   role.UpdatedAt,
	})
}

func (h *RoleHandler) Update(c *gin.Context) {
	var pathReq RolePathRequest
	if err := validation.BindURI(c, &pathReq); err != nil {
		response.Fail(c, err)
		return
	}

	var req UpdateRoleRequest
	if err := validation.BindJSON(c, &req); err != nil {
		response.Fail(c, err)
		return
	}

	role, err := h.roleSvc.UpdateRole(c.Request.Context(), consoleservice.UpdateRoleInput{
		RoleID:      pathReq.ID,
		Name:        req.Name,
		Code:        req.Code,
		DisplayName: req.DisplayName,
		Description: req.Description,
		Status:      req.Status,
		Sort:        req.Sort,
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	setAuditMeta(c, "console_role", role.ID, map[string]any{
		"name":         role.Name,
		"code":         role.Code,
		"display_name": role.DisplayName,
		"status":       role.Status,
		"sort":         role.Sort,
	})
	response.Success(c, RoleDetail{
		ID:          role.ID,
		Name:        role.Name,
		Code:        role.Code,
		DisplayName: role.DisplayName,
		Description: role.Description,
		Sort:        role.Sort,
		Status:      role.Status,
		StatusText:  roleStatusToText(role.Status),
		IsSuper:     role.IsSuper,
		CreatedAt:   role.CreatedAt,
		UpdatedAt:   role.UpdatedAt,
	})
}

func (h *RoleHandler) Delete(c *gin.Context) {
	var req RolePathRequest
	if err := validation.BindURI(c, &req); err != nil {
		response.Fail(c, err)
		return
	}

	if err := h.roleSvc.DeleteRole(c.Request.Context(), consoleservice.DeleteRoleInput{RoleID: req.ID}); err != nil {
		response.Fail(c, err)
		return
	}
	setAuditMeta(c, "console_role", req.ID, map[string]any{
		"deleted": true,
	})
	response.Success(c, nil)
}

func (h *RoleHandler) Permissions(c *gin.Context) {
	var req RolePathRequest
	if err := validation.BindURI(c, &req); err != nil {
		response.Fail(c, err)
		return
	}

	keys, err := h.roleSvc.GetRolePermissions(c.Request.Context(), consoleservice.GetRolePermissionsInput{RoleID: req.ID})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, keys)
}

func (h *RoleHandler) AssignPermissions(c *gin.Context) {
	var pathReq RolePathRequest
	if err := validation.BindURI(c, &pathReq); err != nil {
		response.Fail(c, err)
		return
	}

	var req AssignPermissionsRequest
	if err := validation.BindJSON(c, &req); err != nil {
		response.Fail(c, err)
		return
	}

	if err := h.roleSvc.SetRolePermissions(c.Request.Context(), consoleservice.SetRolePermissionsInput{
		RoleID:         pathReq.ID,
		APIPermissions: req.APIPermissions,
	}); err != nil {
		response.Fail(c, err)
		return
	}
	setAuditMeta(c, "console_role", pathReq.ID, map[string]any{
		"api_permissions":  req.APIPermissions,
		"permission_count": len(req.APIPermissions),
	})
	response.Success(c, nil)
}

func (h *RoleHandler) Menus(c *gin.Context) {
	var req RolePathRequest
	if err := validation.BindURI(c, &req); err != nil {
		response.Fail(c, err)
		return
	}

	keys, err := h.roleSvc.GetRoleMenus(c.Request.Context(), consoleservice.GetRoleMenusInput{RoleID: req.ID})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, keys)
}

func (h *RoleHandler) AssignMenus(c *gin.Context) {
	var pathReq RolePathRequest
	if err := validation.BindURI(c, &pathReq); err != nil {
		response.Fail(c, err)
		return
	}

	var req AssignMenusRequest
	if err := validation.BindJSON(c, &req); err != nil {
		response.Fail(c, err)
		return
	}

	if err := h.roleSvc.SetRoleMenus(c.Request.Context(), consoleservice.SetRoleMenusInput{
		RoleID:   pathReq.ID,
		MenuKeys: req.MenuKeys,
	}); err != nil {
		response.Fail(c, err)
		return
	}
	setAuditMeta(c, "console_role", pathReq.ID, map[string]any{
		"menu_keys":      req.MenuKeys,
		"menu_key_count": len(req.MenuKeys),
	})
	response.Success(c, nil)
}

func roleStatusToText(status int) string {
	switch status {
	case 1:
		return "启用"
	case 0:
		return "禁用"
	default:
		return "未知"
	}
}
