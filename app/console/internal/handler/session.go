package handler

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	consoleservice "github.com/zhimma/grove/app/console/internal/service"
	"github.com/zhimma/grove/pkg/auth"
	"github.com/zhimma/grove/pkg/database"
	"github.com/zhimma/grove/pkg/errx"
	"github.com/zhimma/grove/pkg/request"
	"github.com/zhimma/grove/pkg/response"
	"github.com/zhimma/grove/pkg/route"
	"github.com/zhimma/grove/pkg/validation"
)

type SessionHandler struct {
	sessions *consoleservice.SessionService
}

type ListSessionsRequest struct {
	Page     int    `form:"page" binding:"omitempty,min=1" label:"页码"`
	PageSize int    `form:"page_size" binding:"omitempty,min=1,max=100" label:"每页数量"`
	AdminID  string `form:"admin_id" label:"管理员ID"`
	Keyword  string `form:"keyword" label:"关键词"`
	Status   string `form:"status" binding:"omitempty,oneof=active revoked expired" label:"会话状态"`
}

type SessionAdminResponse struct {
	ID          string `json:"id"`
	Account     string `json:"account"`
	DisplayName string `json:"display_name"`
}

type SessionResponse struct {
	ID           string                `json:"id"`
	AdminID      string                `json:"admin_id"`
	Admin        *SessionAdminResponse `json:"admin,omitempty"`
	DeviceName   string                `json:"device_name"`
	ClientIP     string                `json:"client_ip"`
	UserAgent    string                `json:"user_agent"`
	LastActiveAt string                `json:"last_active_at"`
	ExpiresAt    string                `json:"expires_at"`
	RevokedAt    string                `json:"revoked_at"`
	RevokeReason string                `json:"revoke_reason"`
	Status       string                `json:"status"`
	Current      bool                  `json:"current"`
}

type ListSessionsResponse struct {
	List []SessionResponse `json:"list"`
	Meta ListMeta          `json:"meta"`
}

func RegisterSessionRoutes(protected *gin.RouterGroup, dbs database.Connections, tokens *auth.Manager, policies []consoleservice.PagePolicy, catalog *route.Catalog) {
	h := &SessionHandler{sessions: consoleservice.NewSessionService(dbs, tokens, policies...)}
	sessions := wrapRoute(protected.Group("/sessions"), catalog)
	sessions.GET("", h.List).Name("系统管理.会话列表")
	sessions.DELETE("/:id", h.Revoke).Name("系统管理.强制下线")
}

func (h *SessionHandler) List(c *gin.Context) {
	var req ListSessionsRequest
	if err := validation.BindQuery(c, &req); err != nil {
		response.Fail(c, err)
		return
	}
	result, err := h.sessions.List(c.Request.Context(), consoleservice.ListSessionsInput{
		Page: req.Page, PageSize: req.PageSize, AdminID: req.AdminID, Keyword: req.Keyword, Status: req.Status,
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	items := make([]SessionResponse, 0, len(result.List))
	currentSessionID := request.GetSessionID(c)
	for _, session := range result.List {
		item := SessionResponse{
			ID: session.ID, AdminID: session.AdminID, DeviceName: session.DeviceName,
			ClientIP: session.ClientIP, UserAgent: session.UserAgent,
			LastActiveAt: formatSessionTime(session.LastActiveAt), ExpiresAt: formatSessionTime(session.ExpiresAt),
			RevokeReason: session.RevokeReason, Current: session.ID == currentSessionID,
		}
		if session.RevokedAt != nil {
			item.RevokedAt = formatSessionTime(*session.RevokedAt)
			item.Status = "revoked"
		} else if !session.ExpiresAt.After(time.Now()) {
			item.Status = "expired"
		} else {
			item.Status = "active"
		}
		if session.Admin != nil {
			item.Admin = &SessionAdminResponse{ID: session.Admin.ID, Account: session.Admin.Account, DisplayName: session.Admin.GetDisplayName()}
		}
		items = append(items, item)
	}
	response.Success(c, ListSessionsResponse{
		List: items,
		Meta: ListMeta{Total: result.Meta.Total, Page: result.Meta.Page, PageSize: result.Meta.PageSize, TotalPages: result.Meta.TotalPages},
	})
}

func formatSessionTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339)
}

func (h *SessionHandler) Revoke(c *gin.Context) {
	sessionID := strings.TrimSpace(c.Param("id"))
	if sessionID == "" {
		response.Fail(c, errx.InvalidParams().WithHTTPStatus(422).WithMessage("会话ID不能为空"))
		return
	}
	if err := h.sessions.Revoke(c.Request.Context(), sessionID, "forced_offline"); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}
