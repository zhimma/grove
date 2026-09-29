package handler

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/zhimma/grove/app/api/internal/service"
	"github.com/zhimma/grove/pkg/job"
	"github.com/zhimma/grove/pkg/request"
	"github.com/zhimma/grove/pkg/response"
	"github.com/zhimma/grove/pkg/route"
	"github.com/zhimma/grove/pkg/validation"
)

type DemoStarterHandler struct {
	starterSvc *service.DemoStarterService
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

// RegisterDemoStarterRoutes receives only the dependencies that its service
// actually needs. The API router remains responsible for provider composition.
func RegisterDemoStarterRoutes(public *route.Group, protected *route.Group, db *gorm.DB, jobs *job.Client) {
	h := newStarterHandler(db, jobs)
	public.GET("/ping", h.Ping).Name("示例.连通性检查").Ignore()
	protected.GET("/profile", h.Profile).Name("示例.当前用户")
	protected.POST("/jobs/echo", h.DispatchEchoJob).Name("示例.投递回显任务")
}

func newStarterHandler(db *gorm.DB, jobs *job.Client) *DemoStarterHandler {
	return &DemoStarterHandler{
		starterSvc: service.NewDemoStarterService(db, jobs),
	}
}

func (h *DemoStarterHandler) Ping(c *gin.Context) {
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

func (h *DemoStarterHandler) Profile(c *gin.Context) {
	out, err := h.starterSvc.Profile(c.Request.Context(), service.ProfileInput{
		UserID: request.UserID(c),
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

func (h *DemoStarterHandler) DispatchEchoJob(c *gin.Context) {
	var req DispatchEchoJobRequest
	if err := validation.BindJSONStrict(c, &req); err != nil {
		response.Fail(c, err)
		return
	}

	out, err := h.starterSvc.DispatchEchoJob(c.Request.Context(), service.DispatchEchoJobInput{
		UserID:    request.UserID(c),
		Message:   req.Message,
		RequestID: request.RequestID(c),
	})
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, DispatchEchoJobResponse{
		TaskID: out.TaskID,
	})
}
