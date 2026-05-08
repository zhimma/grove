package handler

import (
	"github.com/gin-gonic/gin"

	consoleservice "github.com/zhimma/grove/app/console/service"
	"github.com/zhimma/grove/internal/model"
	"github.com/zhimma/grove/internal/provider"
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
	Page        int      `form:"page" binding:"omitempty,min=1" label:"页码"`
	PageSize    int      `form:"page_size" binding:"omitempty,min=1,max=100" label:"每页条数"`
	Offset      int      `form:"offset" label:"偏移量"`
	Limit       int      `form:"limit" label:"限制条数"`
	ListAll     bool     `form:"list_all" label:"是否返回全部"`
	Keyword     string   `form:"keyword" label:"关键词"`
	OrderBy     []string `form:"order_by" label:"排序字段"`
	Method      string   `form:"method" label:"请求方法"`
	Module      string   `form:"module" label:"模块"`
	Success     *bool    `form:"success" label:"是否成功"`
	AdminID     string   `form:"admin_id" label:"管理员ID"`
	CreatedFrom string   `form:"created_from" label:"创建开始时间"`
	CreatedTo   string   `form:"created_to" label:"创建结束时间"`
}

type ListOperationLogsResponse struct {
	List []OperationLogItem `json:"list"`
	Meta ListMeta           `json:"meta"`
}

type OperationLogItem struct {
	ID           string `json:"id"`
	AdminID      string `json:"admin_id"`
	AdminAccount string `json:"admin_account"`
	AdminName    string `json:"admin_name"`
	Method       string `json:"method"`
	Path         string `json:"path"`
	Route        string `json:"route"`
	Module       string `json:"module"`
	Action       string `json:"action"`
	TargetType   string `json:"target_type"`
	TargetID     string `json:"target_id"`
	RequestID    string `json:"request_id"`
	StatusCode   int    `json:"status_code"`
	Success      bool   `json:"success"`
	ErrorMessage string `json:"error_message"`
	DurationMS   int64  `json:"duration_ms"`
	ClientIP     string `json:"client_ip"`
	UserAgent    string `json:"user_agent"`
	RequestQuery string `json:"request_query"`
	CreatedAt    string `json:"created_at"`
}

type OperationLogDetailResponse struct {
	Log    OperationLogItem `json:"log"`
	Detail map[string]any   `json:"detail"`
}

type ListLoginLogsRequest struct {
	Page        int      `form:"page" binding:"omitempty,min=1" label:"页码"`
	PageSize    int      `form:"page_size" binding:"omitempty,min=1,max=100" label:"每页条数"`
	Offset      int      `form:"offset" label:"偏移量"`
	Limit       int      `form:"limit" label:"限制条数"`
	ListAll     bool     `form:"list_all" label:"是否返回全部"`
	Keyword     string   `form:"keyword" label:"关键词"`
	OrderBy     []string `form:"order_by" label:"排序字段"`
	Success     *bool    `form:"success" label:"是否成功"`
	AdminID     string   `form:"admin_id" label:"管理员ID"`
	CreatedFrom string   `form:"created_from" label:"创建开始时间"`
	CreatedTo   string   `form:"created_to" label:"创建结束时间"`
}

type ListLoginLogsResponse struct {
	List []LoginLogItem `json:"list"`
	Meta ListMeta       `json:"meta"`
}

type LoginLogItem struct {
	ID            string `json:"id"`
	AdminID       string `json:"admin_id"`
	AdminAccount  string `json:"admin_account"`
	AdminName     string `json:"admin_name"`
	Account       string `json:"account"`
	Success       bool   `json:"success"`
	FailureReason string `json:"failure_reason"`
	RequestID     string `json:"request_id"`
	ClientIP      string `json:"client_ip"`
	UserAgent     string `json:"user_agent"`
	CreatedAt     string `json:"created_at"`
}

func RegisterLogRoutes(protected *gin.RouterGroup, p *provider.Provider) {
	h := &LogHandler{
		logSvc: consoleservice.NewLogService(p.DB),
	}
	group := route.Wrap(protected.Group("/logs"))
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

func toOperationLogItem(item model.ConsoleOperationLog) OperationLogItem {
	result := OperationLogItem{
		ID:           item.ID,
		AdminID:      item.AdminID,
		Method:       item.Method,
		Path:         item.Path,
		Route:        item.Route,
		Module:       item.Module,
		Action:       item.Action,
		TargetType:   item.TargetType,
		TargetID:     item.TargetID,
		RequestID:    item.RequestID,
		StatusCode:   item.StatusCode,
		Success:      item.Success,
		ErrorMessage: item.ErrorMessage,
		DurationMS:   item.DurationMS,
		ClientIP:     item.ClientIP,
		UserAgent:    item.UserAgent,
		RequestQuery: item.RequestQuery,
		CreatedAt:    item.CreatedAt.Format("2006-01-02 15:04:05"),
	}
	if item.Operator != nil {
		result.AdminAccount = item.Operator.Account
		result.AdminName = item.Operator.GetDisplayName()
	}
	return result
}

func toLoginLogItem(item model.ConsoleLoginLog) LoginLogItem {
	result := LoginLogItem{
		ID:            item.ID,
		AdminID:       item.AdminID,
		Account:       item.Account,
		Success:       item.Success,
		FailureReason: item.FailureReason,
		RequestID:     item.RequestID,
		ClientIP:      item.ClientIP,
		UserAgent:     item.UserAgent,
		CreatedAt:     item.CreatedAt.Format("2006-01-02 15:04:05"),
	}
	if item.Operator != nil {
		result.AdminAccount = item.Operator.Account
		result.AdminName = item.Operator.GetDisplayName()
	}
	return result
}
