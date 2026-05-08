package router

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/zhimma/grove/app/console/handler"
	consolemiddleware "github.com/zhimma/grove/app/console/middleware"
	consoleservice "github.com/zhimma/grove/app/console/service"
	"github.com/zhimma/grove/internal/config"
	"github.com/zhimma/grove/internal/provider"
)

type Router struct {
	cfg *config.Config
	p   *provider.Provider
}

func New(cfg *config.Config, p *provider.Provider) *Router {
	return &Router{cfg: cfg, p: p}
}

func (r *Router) InstallToEngine(engine *gin.Engine) {
	_ = r.cfg
	v1 := engine.Group("/console/v1")
	authStateResolver := consoleservice.NewAdminAuthStateResolver(r.p.DB)
	runtimeCatalog := consoleservice.NewRuntimePermissionCatalog()

	public := v1.Group("")
	authed := v1.Group("")
	authed.Use(consolemiddleware.AdminAuthn(r.p.TokenManager, authStateResolver))
	protected := v1.Group("")
	var auditDB *gorm.DB
	if r.p != nil && r.p.DB != nil {
		auditDB = r.p.DB.Default()
	}
	protected.Use(
		consolemiddleware.AdminAuthn(r.p.TokenManager, authStateResolver),
		consolemiddleware.AuditOperation(auditDB),
		consolemiddleware.AdminPermission(r.p.GetEnforcer("console"), r.cfg.App.Env),
	)

	handler.RegisterAuthRoutes(public, authed, r.p)
	handler.RegisterDashboardRoutes(protected, r.p)
	handler.RegisterRoleRoutes(protected, r.p, runtimeCatalog)
	handler.RegisterPermissionRoutes(protected, runtimeCatalog)
	handler.RegisterAdminRoutes(protected, r.p)
	handler.RegisterSystemConfigRoutes(protected, r.p)
	handler.RegisterStorageRoutes(protected, r.p)
	handler.RegisterLogRoutes(protected, r.p)
	// artisan:register-routes

	runtimeCatalog.LoadRoutes(engine.Routes())
}
