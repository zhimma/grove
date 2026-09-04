package handler

import (
	"github.com/gin-gonic/gin"

	consoleservice "github.com/zhimma/grove/app/console/internal/service"
	"github.com/zhimma/grove/pkg/database"
	"github.com/zhimma/grove/pkg/response"
	"github.com/zhimma/grove/pkg/route"
)

type DashboardHandler struct {
	dashboardSvc *consoleservice.DashboardService
}

func RegisterDashboardRoutesWithDeps(protected *gin.RouterGroup, dbs database.Connections, catalog *route.Catalog) {
	h := &DashboardHandler{
		dashboardSvc: consoleservice.NewDashboardService(dbs),
	}
	dashboard := wrapRoute(protected.Group("/dashboard"), catalog)
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
