package handler

import (
	"github.com/gin-gonic/gin"

	consoleservice "github.com/zhimma/grove/app/console/internal/service"
	"github.com/zhimma/grove/pkg/database"
	"github.com/zhimma/grove/pkg/pagination"
	"github.com/zhimma/grove/pkg/response"
	"github.com/zhimma/grove/pkg/route"
	"github.com/zhimma/grove/pkg/validation"
)

type ScheduledTaskHandler struct {
	scheduledTaskSvc *consoleservice.ScheduledTaskService
}

type ScheduledTaskPathRequest struct {
	ID string `uri:"id" binding:"required" label:"计划任务ID"`
}

type ListScheduledTasksRequest struct {
	ListQuery
	Enabled *bool `form:"enabled" label:"是否启用"`
}

type ListScheduledTasksResponse struct {
	List []ScheduledTaskItem `json:"list"`
	Meta pagination.Meta     `json:"meta"`
}

type UpdateScheduledTaskRequest struct {
	Schedule       string `json:"schedule" label:"调度表达式"`
	Mutex          *bool  `json:"mutex" label:"是否互斥"`
	TimeoutSeconds *int   `json:"timeout_seconds" label:"超时秒数"`
}

type SetScheduledTaskStatusRequest struct {
	Enabled *bool `json:"enabled" binding:"required" label:"是否启用"`
}

// RegisterScheduledTaskRoutes exposes editing only. Rows mirror the Worker's
// code registry, so there is deliberately no create or delete: a task Console
// invented would have no handler to run.
func RegisterScheduledTaskRoutes(protected *gin.RouterGroup, dbs *database.Connections, pages pagination.Policy, catalog *route.Catalog) {
	h := &ScheduledTaskHandler{
		scheduledTaskSvc: consoleservice.NewScheduledTaskService(dbs, pages),
	}
	group := wrapRoute(protected.Group("/scheduled-tasks"), catalog)
	group.GET("", h.List).Name("计划任务.任务列表")
	group.PUT("/:id", h.Update).Name("计划任务.更新调度")
	group.PUT("/:id/status", h.SetStatus).Name("计划任务.启停任务")
	group.POST("/:id/run", h.RequestRun).Name("计划任务.手动执行")
}

func (h *ScheduledTaskHandler) List(c *gin.Context) {
	var req ListScheduledTasksRequest
	if err := validation.BindQuery(c, &req); err != nil {
		response.Fail(c, err)
		return
	}
	result, err := h.scheduledTaskSvc.List(c.Request.Context(), consoleservice.ListScheduledTasksInput{
		Request: req.Request,
		Keyword: req.Keyword,
		Enabled: req.Enabled,
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	items := make([]ScheduledTaskItem, 0, len(result.List))
	for _, item := range result.List {
		items = append(items, toScheduledTaskItem(item))
	}
	response.Success(c, ListScheduledTasksResponse{
		List: items,
		Meta: result.Meta,
	})
}

func (h *ScheduledTaskHandler) Update(c *gin.Context) {
	var path ScheduledTaskPathRequest
	if err := validation.BindURI(c, &path); err != nil {
		response.Fail(c, err)
		return
	}
	var req UpdateScheduledTaskRequest
	if err := validation.BindJSON(c, &req); err != nil {
		response.Fail(c, err)
		return
	}
	task, err := h.scheduledTaskSvc.Update(c.Request.Context(), consoleservice.UpdateScheduledTaskInput{
		TaskID:         path.ID,
		Schedule:       req.Schedule,
		Mutex:          req.Mutex,
		TimeoutSeconds: req.TimeoutSeconds,
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, toScheduledTaskItem(*task))
}

func (h *ScheduledTaskHandler) SetStatus(c *gin.Context) {
	var path ScheduledTaskPathRequest
	if err := validation.BindURI(c, &path); err != nil {
		response.Fail(c, err)
		return
	}
	var req SetScheduledTaskStatusRequest
	if err := validation.BindJSON(c, &req); err != nil {
		response.Fail(c, err)
		return
	}
	task, err := h.scheduledTaskSvc.SetStatus(c.Request.Context(), consoleservice.SetScheduledTaskStatusInput{
		TaskID:  path.ID,
		Enabled: *req.Enabled,
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, toScheduledTaskItem(*task))
}

// RequestRun records the request and returns. The Worker picks it up on its
// next reconcile, so a success here means queued, not finished.
func (h *ScheduledTaskHandler) RequestRun(c *gin.Context) {
	var path ScheduledTaskPathRequest
	if err := validation.BindURI(c, &path); err != nil {
		response.Fail(c, err)
		return
	}
	task, err := h.scheduledTaskSvc.RequestRun(c.Request.Context(), consoleservice.RequestScheduledTaskRunInput{
		TaskID: path.ID,
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, toScheduledTaskItem(*task))
}
