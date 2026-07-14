package router

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/zhimma/grove/app/api/handler"
)

func (r *Router) installDemoRoutes(public, protected *gin.RouterGroup) {
	if r == nil || r.cfg == nil || !r.cfg.Demo.Enabled || strings.EqualFold(strings.TrimSpace(r.cfg.App.Env), "production") {
		return
	}
	handler.RegisterDemoAuthRoutes(public, r.p)
	handler.RegisterDemoStarterRoutes(public, protected, r.p)
}
