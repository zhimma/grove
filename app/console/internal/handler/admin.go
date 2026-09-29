package handler

import (
	"github.com/gin-gonic/gin"

	consoleservice "github.com/zhimma/grove/app/console/internal/service"
	"github.com/zhimma/grove/pkg/database"
	"github.com/zhimma/grove/pkg/pagination"
	"github.com/zhimma/grove/pkg/rbac"
	"github.com/zhimma/grove/pkg/request"
	"github.com/zhimma/grove/pkg/response"
	"github.com/zhimma/grove/pkg/route"
	"github.com/zhimma/grove/pkg/validation"
)

type AdminHandler struct {
	adminSvc *consoleservice.AdminService
}

type MessageResponse struct {
	Message string `json:"message"`
}

type ListAdminsRequest struct {
	ListQuery
	RoleID string `form:"role_id" label:"角色ID"`
	Status *int   `form:"status" label:"状态"`
}

type ListAdminsResponse struct {
	List []AdminResponse `json:"list"`
	Meta pagination.Meta `json:"meta"`
}

type CreateAdminRequest struct {
	Account     string `json:"account" binding:"required,min=3,max=50" label:"账号"`
	Username    string `json:"username" binding:"omitempty,max=50" label:"用户名"`
	Email       string `json:"email" binding:"omitempty,email,max=255" label:"邮箱"`
	Phone       string `json:"phone" binding:"omitempty,max=32" label:"手机号"`
	Password    string `json:"password" binding:"required,min=6,max=64" label:"密码"`
	RealName    string `json:"real_name" binding:"omitempty,max=50" label:"真实姓名"`
	DisplayName string `json:"display_name" binding:"omitempty,max=100" label:"显示名称"`
	Avatar      string `json:"avatar" binding:"omitempty,max=500" label:"头像"`
	RoleID      string `json:"role_id" binding:"required" label:"角色"`
	Status      int    `json:"status" label:"状态"`
	Remark      string `json:"remark" binding:"omitempty,max=500" label:"备注"`
}

type UpdateAdminRequest struct {
	Account     *string `json:"account" binding:"omitempty,min=3,max=50" label:"账号"`
	Username    *string `json:"username" binding:"omitempty,max=50" label:"用户名"`
	Email       *string `json:"email" binding:"omitempty,email,max=255" label:"邮箱"`
	Phone       *string `json:"phone" binding:"omitempty,max=32" label:"手机号"`
	Password    *string `json:"password" binding:"omitempty,min=6,max=64" label:"密码"`
	RealName    *string `json:"real_name" binding:"omitempty,max=50" label:"真实姓名"`
	DisplayName *string `json:"display_name" binding:"omitempty,max=100" label:"显示名称"`
	Avatar      *string `json:"avatar" binding:"omitempty,max=500" label:"头像"`
	RoleID      *string `json:"role_id" label:"角色"`
	Status      *int    `json:"status" label:"状态"`
	Remark      *string `json:"remark" binding:"omitempty,max=500" label:"备注"`
}

type UpdateAdminStatusRequest struct {
	Status *int `json:"status" binding:"required" label:"状态"`
}

type ResetAdminPasswordRequest struct {
	Password string `json:"password" binding:"required,min=6,max=64" label:"密码"`
}

type AdminPathRequest struct {
	ID string `uri:"id" binding:"required" label:"管理员ID"`
}

func RegisterAdminRoutes(protected *gin.RouterGroup, dbs *database.Connections, enforcer *rbac.Enforcer, pages pagination.Policy, catalog *route.Catalog) {
	h := &AdminHandler{
		adminSvc: consoleservice.NewAdminService(dbs, enforcer, pages),
	}

	admins := wrapRoute(protected.Group("/admins"), catalog)
	admins.GET("", h.List).Name("系统管理.管理员列表")
	admins.GET("/:id", h.Detail).Name("系统管理.管理员详情")
	admins.POST("", h.Create).Name("系统管理.创建管理员")
	admins.PUT("/:id", h.Update).Name("系统管理.更新管理员")
	admins.PUT("/:id/status", h.UpdateStatus).Name("系统管理.更新管理员状态")
	admins.PUT("/:id/reset-password", h.ResetPassword).Name("系统管理.重置管理员密码")
	admins.DELETE("/:id", h.Delete).Name("系统管理.删除管理员")
}

func (h *AdminHandler) List(c *gin.Context) {
	var req ListAdminsRequest
	if err := validation.BindQuery(c, &req); err != nil {
		response.Fail(c, err)
		return
	}

	result, err := h.adminSvc.ListAdmins(c.Request.Context(), consoleservice.ListAdminsInput{
		Request:     req.Request,
		Keyword:     req.Keyword,
		OrderBy:     req.OrderBy,
		RoleID:      req.RoleID,
		Status:      req.Status,
		CreatedFrom: req.CreatedFrom,
		CreatedTo:   req.CreatedTo,
	})
	if err != nil {
		response.Fail(c, err)
		return
	}

	items := make([]AdminResponse, 0, len(result.List))
	for i := range result.List {
		items = append(items, newAdminResponse(&result.List[i]))
	}

	response.Success(c, ListAdminsResponse{
		List: items,
		Meta: result.Meta,
	})
}

func (h *AdminHandler) Detail(c *gin.Context) {
	var req AdminPathRequest
	if err := validation.BindURI(c, &req); err != nil {
		response.Fail(c, err)
		return
	}

	admin, err := h.adminSvc.GetAdmin(c.Request.Context(), consoleservice.GetAdminInput{AdminID: req.ID})
	if err != nil {
		response.Fail(c, err)
		return
	}
	setAuditMeta(c, "console_admin", admin.ID, map[string]any{
		"account":      admin.Account,
		"display_name": admin.GetDisplayName(),
		"role_id":      admin.RoleID,
		"status":       admin.Status,
	})
	response.Success(c, newAdminResponse(admin))
}

func (h *AdminHandler) Create(c *gin.Context) {
	var req CreateAdminRequest
	if err := validation.BindJSON(c, &req); err != nil {
		response.Fail(c, err)
		return
	}

	admin, err := h.adminSvc.CreateAdmin(c.Request.Context(), consoleservice.CreateAdminInput{
		Account:     req.Account,
		Username:    req.Username,
		Email:       req.Email,
		Phone:       req.Phone,
		Password:    req.Password,
		RealName:    req.RealName,
		DisplayName: req.DisplayName,
		Avatar:      req.Avatar,
		RoleID:      req.RoleID,
		Status:      req.Status,
		Remark:      req.Remark,
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	setAuditMeta(c, "console_admin", admin.ID, map[string]any{
		"account":      admin.Account,
		"display_name": admin.GetDisplayName(),
		"role_id":      admin.RoleID,
		"status":       admin.Status,
	})
	response.Success(c, newAdminResponse(admin))
}

func (h *AdminHandler) Update(c *gin.Context) {
	var pathReq AdminPathRequest
	if err := validation.BindURI(c, &pathReq); err != nil {
		response.Fail(c, err)
		return
	}

	var req UpdateAdminRequest
	if err := validation.BindJSON(c, &req); err != nil {
		response.Fail(c, err)
		return
	}

	admin, err := h.adminSvc.UpdateAdmin(c.Request.Context(), consoleservice.UpdateAdminInput{
		AdminID:     pathReq.ID,
		Account:     req.Account,
		Username:    req.Username,
		Email:       req.Email,
		Phone:       req.Phone,
		Password:    req.Password,
		RealName:    req.RealName,
		DisplayName: req.DisplayName,
		Avatar:      req.Avatar,
		RoleID:      req.RoleID,
		Status:      req.Status,
		Remark:      req.Remark,
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	setAuditMeta(c, "console_admin", admin.ID, map[string]any{
		"status": admin.Status,
	})
	response.Success(c, newAdminResponse(admin))
}

func (h *AdminHandler) UpdateStatus(c *gin.Context) {
	var pathReq AdminPathRequest
	if err := validation.BindURI(c, &pathReq); err != nil {
		response.Fail(c, err)
		return
	}

	var req UpdateAdminStatusRequest
	if err := validation.BindJSON(c, &req); err != nil {
		response.Fail(c, err)
		return
	}

	admin, err := h.adminSvc.UpdateAdminStatus(c.Request.Context(), consoleservice.UpdateAdminStatusInput{
		AdminID: pathReq.ID,
		Status:  *req.Status,
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, newAdminResponse(admin))
}

func (h *AdminHandler) Delete(c *gin.Context) {
	var req AdminPathRequest
	if err := validation.BindURI(c, &req); err != nil {
		response.Fail(c, err)
		return
	}

	if err := h.adminSvc.DeleteAdmin(c.Request.Context(), consoleservice.DeleteAdminInput{
		AdminID:    req.ID,
		OperatorID: request.AdminID(c),
	}); err != nil {
		response.Fail(c, err)
		return
	}
	setAuditMeta(c, "console_admin", req.ID, map[string]any{
		"deleted": true,
	})
	response.Success(c, nil)
}

func (h *AdminHandler) ResetPassword(c *gin.Context) {
	var pathReq AdminPathRequest
	if err := validation.BindURI(c, &pathReq); err != nil {
		response.Fail(c, err)
		return
	}

	var req ResetAdminPasswordRequest
	if err := validation.BindJSON(c, &req); err != nil {
		response.Fail(c, err)
		return
	}

	if err := h.adminSvc.ResetPassword(c.Request.Context(), consoleservice.ResetAdminPasswordInput{
		AdminID:  pathReq.ID,
		Password: req.Password,
	}); err != nil {
		response.Fail(c, err)
		return
	}
	setAuditMeta(c, "console_admin", pathReq.ID, map[string]any{
		"password_reset": true,
	})
	response.Success(c, MessageResponse{Message: "password reset successfully"})
}
