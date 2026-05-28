package handler

import (
	"github.com/gin-gonic/gin"

	consoleservice "github.com/zhimma/grove/app/console/internal/service"
	"github.com/zhimma/grove/internal/provider"
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
	Page        int      `form:"page" binding:"omitempty,min=1" label:"页码"`
	PageSize    int      `form:"page_size" binding:"omitempty,min=1,max=100" label:"每页条数"`
	Offset      int      `form:"offset" label:"偏移量"`
	Limit       int      `form:"limit" label:"限制条数"`
	ListAll     bool     `form:"list_all" label:"是否返回全部"`
	Keyword     string   `form:"keyword" label:"关键词"`
	OrderBy     []string `form:"order_by" label:"排序字段"`
	RoleID      string   `form:"role_id" label:"角色ID"`
	Status      *int     `form:"status" label:"状态"`
	CreatedFrom string   `form:"created_from" label:"创建开始时间"`
	CreatedTo   string   `form:"created_to" label:"创建结束时间"`
}

type ListAdminsResponse struct {
	List []AdminItem `json:"list"`
	Meta ListMeta    `json:"meta"`
}

type AdminItem struct {
	ID          string `json:"id"`
	Account     string `json:"account"`
	Username    string `json:"username"`
	Email       string `json:"email"`
	Phone       string `json:"phone"`
	RealName    string `json:"real_name"`
	DisplayName string `json:"display_name"`
	Avatar      string `json:"avatar"`
	RoleID      string `json:"role_id"`
	RoleName    string `json:"role_name"`
	Status      int    `json:"status"`
	IsSuper     bool   `json:"is_super"`
	CreatedAt   string `json:"created_at"`
}

type AdminDetail struct {
	ID            string    `json:"id"`
	Account       string    `json:"account"`
	Username      string    `json:"username"`
	Email         string    `json:"email"`
	Phone         string    `json:"phone"`
	RealName      string    `json:"real_name"`
	DisplayName   string    `json:"display_name"`
	Avatar        string    `json:"avatar"`
	RoleID        string    `json:"role_id"`
	Role          *RoleInfo `json:"role,omitempty"`
	Status        int       `json:"status"`
	StatusText    string    `json:"status_text"`
	EmailVerified bool      `json:"email_verified"`
	PhoneVerified bool      `json:"phone_verified"`
	IsSuper       bool      `json:"is_super"`
	Remark        string    `json:"remark"`
	CreatedAt     string    `json:"created_at"`
	UpdatedAt     string    `json:"updated_at"`
}

type RoleInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Code string `json:"code"`
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

func RegisterAdminRoutes(protected *gin.RouterGroup, p *provider.Provider) {
	h := &AdminHandler{
		adminSvc: consoleservice.NewAdminService(p.DB, p.GetEnforcer("console")).WithTransaction(p.TxManager),
	}

	admins := route.Wrap(protected.Group("/admins"))
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
		Page:        req.Page,
		PageSize:    req.PageSize,
		Offset:      req.Offset,
		Limit:       req.Limit,
		ListAll:     req.ListAll,
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

	items := make([]AdminItem, 0, len(result.List))
	for _, admin := range result.List {
		item := AdminItem{
			ID:          admin.ID,
			Account:     admin.Account,
			Username:    admin.Username,
			Email:       admin.Email,
			Phone:       admin.Phone,
			RealName:    admin.RealName,
			DisplayName: admin.GetDisplayName(),
			Avatar:      admin.Avatar,
			RoleID:      admin.RoleID,
			Status:      admin.Status,
			IsSuper:     admin.HasSuperAccess(),
			CreatedAt:   admin.CreatedAt.Format("2006-01-02 15:04:05"),
		}
		if admin.Role != nil {
			item.RoleName = admin.Role.Name
		}
		items = append(items, item)
	}

	response.Success(c, ListAdminsResponse{
		List: items,
		Meta: ListMeta(result.Meta),
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
	var role *RoleInfo
	if admin.Role != nil {
		role = &RoleInfo{ID: admin.Role.ID, Name: admin.Role.Name, Code: admin.Role.Code}
	}
	response.Success(c, AdminDetail{
		ID:            admin.ID,
		Account:       admin.Account,
		Username:      admin.Username,
		Email:         admin.Email,
		Phone:         admin.Phone,
		RealName:      admin.RealName,
		DisplayName:   admin.GetDisplayName(),
		Avatar:        admin.Avatar,
		RoleID:        admin.RoleID,
		Role:          role,
		Status:        admin.Status,
		StatusText:    adminStatusToText(admin.Status),
		EmailVerified: admin.EmailVerified,
		PhoneVerified: admin.PhoneVerified,
		IsSuper:       admin.HasSuperAccess(),
		Remark:        admin.Remark,
		CreatedAt:     admin.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:     admin.UpdatedAt.Format("2006-01-02 15:04:05"),
	})
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
	var role *RoleInfo
	if admin.Role != nil {
		role = &RoleInfo{ID: admin.Role.ID, Name: admin.Role.Name, Code: admin.Role.Code}
	}
	response.Success(c, AdminDetail{
		ID:            admin.ID,
		Account:       admin.Account,
		Username:      admin.Username,
		Email:         admin.Email,
		Phone:         admin.Phone,
		RealName:      admin.RealName,
		DisplayName:   admin.GetDisplayName(),
		Avatar:        admin.Avatar,
		RoleID:        admin.RoleID,
		Role:          role,
		Status:        admin.Status,
		StatusText:    adminStatusToText(admin.Status),
		EmailVerified: admin.EmailVerified,
		PhoneVerified: admin.PhoneVerified,
		IsSuper:       admin.HasSuperAccess(),
		Remark:        admin.Remark,
		CreatedAt:     admin.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:     admin.UpdatedAt.Format("2006-01-02 15:04:05"),
	})
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
	var role *RoleInfo
	if admin.Role != nil {
		role = &RoleInfo{ID: admin.Role.ID, Name: admin.Role.Name, Code: admin.Role.Code}
	}
	response.Success(c, AdminDetail{
		ID:            admin.ID,
		Account:       admin.Account,
		Username:      admin.Username,
		Email:         admin.Email,
		Phone:         admin.Phone,
		RealName:      admin.RealName,
		DisplayName:   admin.GetDisplayName(),
		Avatar:        admin.Avatar,
		RoleID:        admin.RoleID,
		Role:          role,
		Status:        admin.Status,
		StatusText:    adminStatusToText(admin.Status),
		EmailVerified: admin.EmailVerified,
		PhoneVerified: admin.PhoneVerified,
		IsSuper:       admin.HasSuperAccess(),
		Remark:        admin.Remark,
		CreatedAt:     admin.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:     admin.UpdatedAt.Format("2006-01-02 15:04:05"),
	})
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
	var role *RoleInfo
	if admin.Role != nil {
		role = &RoleInfo{ID: admin.Role.ID, Name: admin.Role.Name, Code: admin.Role.Code}
	}
	response.Success(c, AdminDetail{
		ID:            admin.ID,
		Account:       admin.Account,
		Username:      admin.Username,
		Email:         admin.Email,
		Phone:         admin.Phone,
		RealName:      admin.RealName,
		DisplayName:   admin.GetDisplayName(),
		Avatar:        admin.Avatar,
		RoleID:        admin.RoleID,
		Role:          role,
		Status:        admin.Status,
		StatusText:    adminStatusToText(admin.Status),
		EmailVerified: admin.EmailVerified,
		PhoneVerified: admin.PhoneVerified,
		IsSuper:       admin.HasSuperAccess(),
		Remark:        admin.Remark,
		CreatedAt:     admin.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:     admin.UpdatedAt.Format("2006-01-02 15:04:05"),
	})
}

func (h *AdminHandler) Delete(c *gin.Context) {
	var req AdminPathRequest
	if err := validation.BindURI(c, &req); err != nil {
		response.Fail(c, err)
		return
	}

	if err := h.adminSvc.DeleteAdmin(c.Request.Context(), consoleservice.DeleteAdminInput{
		AdminID:    req.ID,
		OperatorID: request.GetAdminID(c),
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

func adminStatusToText(status int) string {
	switch status {
	case 1:
		return "启用"
	case 0:
		return "禁用"
	case 2:
		return "锁定"
	default:
		return "未知"
	}
}
