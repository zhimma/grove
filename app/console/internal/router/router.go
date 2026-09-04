package router

import (
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/zhimma/grove/app/console/internal/handler"
	consolemiddleware "github.com/zhimma/grove/app/console/internal/middleware"
	consoleservice "github.com/zhimma/grove/app/console/internal/service"
	"github.com/zhimma/grove/internal/config"
	"github.com/zhimma/grove/internal/provider"
	"github.com/zhimma/grove/pkg/ratelimit"
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
	sessions := consoleservice.NewSessionService(r.p.DB, r.p.TokenManager)
	runtimeCatalog := consoleservice.NewRuntimePermissionCatalog()
	var loginGuard ratelimit.LoginGuard
	pagePolicies := []consoleservice.PagePolicy{consoleservice.NewPagePolicy(r.cfg.API.DefaultPerPage, r.cfg.API.MaxPerPage)}
	if r.p.Config != nil && r.p.Config.Security.Login.Enabled {
		loginCfg := r.p.Config.Security.Login
		loginGuard = ratelimit.NewLoginGuard(ratelimit.LoginConfig{
			AttemptsPerMinute: loginCfg.AttemptsPerMinute,
			Burst:             loginCfg.Burst,
			FailureLimit:      loginCfg.FailureLimit,
			LockDuration:      time.Duration(loginCfg.LockSeconds) * time.Second,
		}, r.p.RedisClient)
	}

	public := v1.Group("")
	authed := v1.Group("")
	authed.Use(consolemiddleware.AdminAuthn(r.p.TokenManager, sessions, authStateResolver))
	protected := v1.Group("")
	var auditDB *gorm.DB
	if r.p != nil && r.p.DB != nil {
		auditDB = r.p.DB.Default()
	}
	protected.Use(
		consolemiddleware.AdminAuthn(r.p.TokenManager, sessions, authStateResolver),
		consolemiddleware.AuditOperation(auditDB),
		consolemiddleware.AdminPermission(r.p.GetEnforcer("console"), r.p.RouteCatalog),
	)

	catalog := r.p.RouteCatalog
	handler.RegisterAuthRoutes(public, authed, r.p.DB, r.p.GetEnforcer("console"), r.p.TokenManager, loginGuard, catalog)
	handler.RegisterDashboardRoutes(protected, r.p.DB, catalog)
	handler.RegisterRoleRoutes(protected, r.p.DB, r.p.GetEnforcer("console"), runtimeCatalog, pagePolicies, catalog)
	handler.RegisterPermissionRoutes(protected, runtimeCatalog, catalog)
	handler.RegisterAdminRoutes(protected, r.p.DB, r.p.GetEnforcer("console"), pagePolicies, catalog)
	handler.RegisterUserRoutes(protected, r.p.DB, catalog)
	handler.RegisterArticleRoutes(protected, r.p.DB, pagePolicies, catalog)
	handler.RegisterSessionRoutes(protected, r.p.DB, r.p.TokenManager, pagePolicies, catalog)
	handler.RegisterSystemConfigRoutes(protected, r.p.DB, r.p.ConfigSecrets, pagePolicies, catalog)
	handler.RegisterStorageRoutes(protected, r.p.Storage, catalog)
	handler.RegisterLogRoutes(protected, r.p.DB, pagePolicies, catalog)
	handler.RegisterScheduledTaskRoutes(protected, r.p.DB, pagePolicies, catalog)
	// grove:register-routes

	runtimeCatalog.LoadRoutes(engine.Routes(), catalog)
}
