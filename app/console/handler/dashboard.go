package handler

import (
	"github.com/gin-gonic/gin"

	consoleservice "github.com/zhimma/grove/app/console/service"
	"github.com/zhimma/grove/internal/provider"
	"github.com/zhimma/grove/pkg/response"
	"github.com/zhimma/grove/pkg/route"
)

type DashboardHandler struct {
	dashboardSvc *consoleservice.DashboardService
}

func RegisterDashboardRoutes(protected *gin.RouterGroup, p *provider.Provider) {
	h := &DashboardHandler{
		dashboardSvc: consoleservice.NewDashboardService(p.DB),
	}
	dashboard := route.Wrap(protected.Group("/dashboard"))
	dashboard.GET("/summary", h.Summary).Name("工作台.概览")
}

func (h *DashboardHandler) Summary(c *gin.Context) {
	out, err := h.dashboardSvc.Summary(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, out)
}
