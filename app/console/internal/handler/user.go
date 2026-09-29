package handler

import (
	"github.com/gin-gonic/gin"

	consoleservice "github.com/zhimma/grove/app/console/internal/service"
	"github.com/zhimma/grove/pkg/database"
	"github.com/zhimma/grove/pkg/pagination"
	"github.com/zhimma/grove/pkg/request"
	"github.com/zhimma/grove/pkg/response"
	"github.com/zhimma/grove/pkg/route"
	"github.com/zhimma/grove/pkg/validation"
)

type UserHandler struct {
	userSvc *consoleservice.UserService
}

type ListUsersRequest struct {
	ListQuery
	Status *int `form:"status" label:"状态"`
}

type ListUsersResponse struct {
	List []UserResponse  `json:"list"`
	Meta pagination.Meta `json:"meta"`
}

type CreateUserRequest struct {
	Name   string `json:"name" binding:"required,max=120" label:"用户名称"`
	Email  string `json:"email" binding:"required,email,max=160" label:"邮箱"`
	Phone  string `json:"phone" binding:"omitempty,max=32" label:"手机号"`
	Avatar string `json:"avatar" binding:"omitempty,max=255" label:"头像"`
	Remark string `json:"remark" binding:"omitempty,max=500" label:"备注"`
	Status *int   `json:"status" binding:"omitempty,oneof=0 1" label:"状态"`
}

type UpdateUserRequest struct {
	Name   *string `json:"name" binding:"omitempty,max=120" label:"用户名称"`
	Email  *string `json:"email" binding:"omitempty,email,max=160" label:"邮箱"`
	Phone  *string `json:"phone" binding:"omitempty,max=32" label:"手机号"`
	Avatar *string `json:"avatar" binding:"omitempty,max=255" label:"头像"`
	Remark *string `json:"remark" binding:"omitempty,max=500" label:"备注"`
	Status *int    `json:"status" binding:"omitempty,oneof=0 1" label:"状态"`
}

type UpdateUserStatusRequest struct {
	Status *int `json:"status" binding:"required,oneof=0 1" label:"状态"`
}

type UserPathRequest struct {
	ID string `uri:"id" binding:"required" label:"用户ID"`
}

func RegisterUserRoutes(protected *gin.RouterGroup, dbs database.Connections, pages pagination.Policy, catalog *route.Catalog) {
	h := &UserHandler{
		userSvc: consoleservice.NewUserService(dbs, pages),
	}
	users := wrapRoute(protected.Group("/users"), catalog)
	users.GET("", h.List).Name("用户管理.用户列表")
	users.GET("/:id", h.Detail).Name("用户管理.用户详情")
	users.POST("", h.Create).Name("用户管理.创建用户")
	users.PUT("/:id", h.Update).Name("用户管理.更新用户")
	users.PUT("/:id/status", h.UpdateStatus).Name("用户管理.更新用户状态")
	users.DELETE("/:id", h.Delete).Name("用户管理.删除用户")
}

func (h *UserHandler) List(c *gin.Context) {
	var req ListUsersRequest
	if err := validation.BindQuery(c, &req); err != nil {
		response.Fail(c, err)
		return
	}
	result, err := h.userSvc.ListUsers(c.Request.Context(), consoleservice.ListUsersInput{
		Request:     req.Request,
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
	items := make([]UserResponse, 0, len(result.List))
	for i := range result.List {
		items = append(items, newUserResponse(&result.List[i]))
	}
	response.Success(c, ListUsersResponse{List: items, Meta: result.Meta})
}

func (h *UserHandler) Detail(c *gin.Context) {
	var req UserPathRequest
	if err := validation.BindURI(c, &req); err != nil {
		response.Fail(c, err)
		return
	}
	user, err := h.userSvc.GetUser(c.Request.Context(), consoleservice.GetUserInput{UserID: req.ID})
	if err != nil {
		response.Fail(c, err)
		return
	}
	setAuditMeta(c, "user", user.ID, map[string]any{
		"name":   user.Name,
		"email":  user.Email,
		"status": user.Status,
	})
	response.Success(c, newUserResponse(user))
}

func (h *UserHandler) Create(c *gin.Context) {
	var req CreateUserRequest
	if err := validation.BindJSON(c, &req); err != nil {
		response.Fail(c, err)
		return
	}
	user, err := h.userSvc.CreateUser(c.Request.Context(), consoleservice.CreateUserInput{
		Name:   req.Name,
		Email:  req.Email,
		Phone:  req.Phone,
		Avatar: req.Avatar,
		Remark: req.Remark,
		Status: req.Status,
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	setAuditMeta(c, "user", user.ID, map[string]any{
		"name":   user.Name,
		"email":  user.Email,
		"status": user.Status,
	})
	response.Success(c, newUserResponse(user))
}

func (h *UserHandler) Update(c *gin.Context) {
	var pathReq UserPathRequest
	if err := validation.BindURI(c, &pathReq); err != nil {
		response.Fail(c, err)
		return
	}
	var req UpdateUserRequest
	if err := validation.BindJSON(c, &req); err != nil {
		response.Fail(c, err)
		return
	}
	user, err := h.userSvc.UpdateUser(c.Request.Context(), consoleservice.UpdateUserInput{
		UserID: pathReq.ID,
		Name:   req.Name,
		Email:  req.Email,
		Phone:  req.Phone,
		Avatar: req.Avatar,
		Remark: req.Remark,
		Status: req.Status,
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	setAuditMeta(c, "user", user.ID, map[string]any{
		"name":   user.Name,
		"email":  user.Email,
		"status": user.Status,
	})
	response.Success(c, newUserResponse(user))
}

func (h *UserHandler) UpdateStatus(c *gin.Context) {
	var pathReq UserPathRequest
	if err := validation.BindURI(c, &pathReq); err != nil {
		response.Fail(c, err)
		return
	}
	var req UpdateUserStatusRequest
	if err := validation.BindJSON(c, &req); err != nil {
		response.Fail(c, err)
		return
	}
	user, err := h.userSvc.UpdateUserStatus(c.Request.Context(), consoleservice.UpdateUserStatusInput{
		UserID: pathReq.ID,
		Status: *req.Status,
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	setAuditMeta(c, "user", user.ID, map[string]any{"status": user.Status})
	response.Success(c, newUserResponse(user))
}

func (h *UserHandler) Delete(c *gin.Context) {
	var req UserPathRequest
	if err := validation.BindURI(c, &req); err != nil {
		response.Fail(c, err)
		return
	}
	if err := h.userSvc.DeleteUser(c.Request.Context(), consoleservice.DeleteUserInput{UserID: req.ID}); err != nil {
		response.Fail(c, err)
		return
	}
	setAuditMeta(c, "user", req.ID, map[string]any{"deleted": true, "operator_id": request.AdminID(c)})
	response.Success(c, nil)
}
