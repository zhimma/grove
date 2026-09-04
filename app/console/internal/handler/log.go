package handler

import (
	"github.com/gin-gonic/gin"

	consoleservice "github.com/zhimma/grove/app/console/internal/service"
	"github.com/zhimma/grove/pkg/database"
	"github.com/zhimma/grove/pkg/response"
	"github.com/zhimma/grove/pkg/route"
	"github.com/zhimma/grove/pkg/validation"
)

type LogHandler struct {
	logSvc *consoleservice.LogService
}

type OperationLogPathRequest struct {
	ID string `uri:"id" binding:"required" label:"操作日志ID"`
}

type ListOperationLogsRequest struct {
	ListQuery
	Method  string `form:"method" label:"请求方法"`
	Module  string `form:"module" label:"模块"`
	Success *bool  `form:"success" label:"是否成功"`
	AdminID string `form:"admin_id" label:"管理员ID"`
}

type ListOperationLogsResponse struct {
	List []OperationLogItem `json:"list"`
	Meta ListMeta           `json:"meta"`
}

type ListLoginLogsRequest struct {
	ListQuery
	Success *bool  `form:"success" label:"是否成功"`
	AdminID string `form:"admin_id" label:"管理员ID"`
}

type ListLoginLogsResponse struct {
	List []LoginLogItem `json:"list"`
	Meta ListMeta       `json:"meta"`
}

func RegisterLogRoutes(protected *gin.RouterGroup, dbs database.Connections, policies []consoleservice.PagePolicy, catalog *route.Catalog) {
	h := &LogHandler{
		logSvc: consoleservice.NewLogService(dbs, policies...),
	}
	group := wrapRoute(protected.Group("/logs"), catalog)
	group.GET("/operations", h.OperationLogs).Name("系统日志.操作日志列表")
	group.GET("/operations/:id", h.OperationLogDetail).Name("系统日志.操作日志详情")
	group.GET("/logins", h.LoginLogs).Name("系统日志.登录日志列表")
}

func (h *LogHandler) OperationLogs(c *gin.Context) {
	var req ListOperationLogsRequest
	if err := validation.BindQuery(c, &req); err != nil {
		response.Fail(c, err)
		return
	}
	result, err := h.logSvc.ListOperationLogs(c.Request.Context(), consoleservice.ListOperationLogsInput{
		Page:        req.Page,
		PageSize:    req.PageSize,
		Offset:      req.Offset,
		Limit:       req.Limit,
		ListAll:     req.ListAll,
		Keyword:     req.Keyword,
		OrderBy:     req.OrderBy,
		Method:      req.Method,
		Module:      req.Module,
		Success:     req.Success,
		AdminID:     req.AdminID,
		CreatedFrom: req.CreatedFrom,
		CreatedTo:   req.CreatedTo,
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	items := make([]OperationLogItem, 0, len(result.List))
	for _, item := range result.List {
		items = append(items, toOperationLogItem(item))
	}
	response.Success(c, ListOperationLogsResponse{
		List: items,
		Meta: ListMeta(result.Meta),
	})
}

func (h *LogHandler) LoginLogs(c *gin.Context) {
	var req ListLoginLogsRequest
	if err := validation.BindQuery(c, &req); err != nil {
		response.Fail(c, err)
		return
	}
	result, err := h.logSvc.ListLoginLogs(c.Request.Context(), consoleservice.ListLoginLogsInput{
		Page:        req.Page,
		PageSize:    req.PageSize,
		Offset:      req.Offset,
		Limit:       req.Limit,
		ListAll:     req.ListAll,
		Keyword:     req.Keyword,
		OrderBy:     req.OrderBy,
		Success:     req.Success,
		AdminID:     req.AdminID,
		CreatedFrom: req.CreatedFrom,
		CreatedTo:   req.CreatedTo,
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	items := make([]LoginLogItem, 0, len(result.List))
	for _, item := range result.List {
		items = append(items, toLoginLogItem(item))
	}
	response.Success(c, ListLoginLogsResponse{
		List: items,
		Meta: ListMeta(result.Meta),
	})
}

func (h *LogHandler) OperationLogDetail(c *gin.Context) {
	var req OperationLogPathRequest
	if err := validation.BindURI(c, &req); err != nil {
		response.Fail(c, err)
		return
	}
	result, err := h.logSvc.GetOperationLogDetail(c.Request.Context(), consoleservice.GetOperationLogDetailInput{
		LogID: req.ID,
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, OperationLogDetailResponse{
		Log:    toOperationLogItem(*result.Log),
		Detail: result.Detail,
	})
}
