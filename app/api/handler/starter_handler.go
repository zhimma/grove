package handler

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/zhimma/grove/app/api/service"
	"github.com/zhimma/grove/internal/provider"
	"github.com/zhimma/grove/pkg/reqctx"
	"github.com/zhimma/grove/pkg/response"
	"github.com/zhimma/grove/pkg/validation"
)

type StarterHandler struct {
	starterSvc *service.StarterService
}

type PingRequest struct {
	Name string `form:"name" label:"名称"`
}

type PingResponse struct {
	Message   string `json:"message"`
	Service   string `json:"service"`
	RequestID string `json:"request_id"`
}

type ProfileResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	RequestID string `json:"request_id"`
}

type DispatchEchoJobRequest struct {
	Message string `json:"message" binding:"required" label:"消息内容"`
}

type DispatchEchoJobResponse struct {
	TaskID string `json:"task_id"`
}

func RegisterStarterRoutes(public *gin.RouterGroup, protected *gin.RouterGroup, p *provider.Provider) {
	h := newStarterHandler(p)
	public.GET("/ping", h.Ping)
	protected.GET("/profile", h.Profile)
	protected.POST("/jobs/echo", h.DispatchEchoJob)
}

func newStarterHandler(p *provider.Provider) *StarterHandler {
	return &StarterHandler{
		starterSvc: service.NewStarterService(defaultDB(p), p.JobClient),
	}
}

func (h *StarterHandler) Ping(c *gin.Context) {
	var req PingRequest
	if err := validation.BindQuery(c, &req); err != nil {
		response.Fail(c, err)
		return
	}

	out, err := h.starterSvc.Ping(c.Request.Context(), service.PingInput{
		Name: req.Name,
	})
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, PingResponse{
		Message:   out.Message,
		Service:   out.Service,
		RequestID: out.RequestID,
	})
}

func (h *StarterHandler) Profile(c *gin.Context) {
	out, err := h.starterSvc.Profile(c.Request.Context(), service.ProfileInput{
		UserID: reqctx.GetUserID(c),
	})
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, ProfileResponse{
		ID:        out.ID,
		Name:      out.Name,
		Email:     out.Email,
		RequestID: out.RequestID,
	})
}

func (h *StarterHandler) DispatchEchoJob(c *gin.Context) {
	var req DispatchEchoJobRequest
	if err := validation.BindJSON(c, &req); err != nil {
		response.Fail(c, err)
		return
	}

	out, err := h.starterSvc.DispatchEchoJob(c.Request.Context(), service.DispatchEchoJobInput{
		UserID:    reqctx.GetUserID(c),
		Message:   req.Message,
		RequestID: reqctx.GetRequestID(c),
	})
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, DispatchEchoJobResponse{
		TaskID: out.TaskID,
	})
}

func defaultDB(p *provider.Provider) *gorm.DB {
	if p == nil || p.DB == nil {
		return nil
	}
	return p.DB.Default()
}
