package router

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/zhimma/grove/app/api/internal/handler"
	"github.com/zhimma/grove/pkg/route"
)

func (r *Router) installDemoRoutes(public, protected *gin.RouterGroup) {
	if r == nil || r.cfg == nil || !r.cfg.Demo.Enabled || strings.EqualFold(strings.TrimSpace(r.cfg.App.Env), "production") {
		return
	}
	publicRoutes := route.Wrap(public, r.catalog)
	protectedRoutes := route.Wrap(protected, r.catalog)
	handler.RegisterDemoAuthRoutes(publicRoutes, r.tokenManager)
	handler.RegisterDemoStarterRoutes(publicRoutes, protectedRoutes, r.db, r.jobClient)
}
