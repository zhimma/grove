package router

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	apimiddleware "github.com/zhimma/grove/app/api/internal/middleware"
	"github.com/zhimma/grove/internal/config"
	"github.com/zhimma/grove/internal/provider"
	"github.com/zhimma/grove/pkg/auth"
	"github.com/zhimma/grove/pkg/job"
	"github.com/zhimma/grove/pkg/rbac"
	"github.com/zhimma/grove/pkg/route"
)

type Router struct {
	cfg          *config.Config
	tokenManager *auth.Manager
	db           *gorm.DB
	jobClient    *job.Client
	apiEnforcer  *rbac.Enforcer
	catalog      *route.Catalog
	userAuth     *apimiddleware.UserAuthSet
}

func New(cfg *config.Config, p *provider.Provider) *Router {
	r := &Router{cfg: cfg}
	if p != nil {
		r.tokenManager = p.TokenManager
		r.jobClient = p.JobClient
		r.apiEnforcer = p.GetEnforcer("api")
		r.catalog = p.RouteCatalog
		if p.DB != nil {
			r.db = p.DB.Default()
		}
	}
	if r.catalog == nil {
		r.catalog = route.NewCatalog()
	}
	r.userAuth = apimiddleware.NewUserAuthSet(r.tokenManager)
	return r
}

func (r *Router) InstallToEngine(engine *gin.Engine) {
	if r == nil || r.cfg == nil || engine == nil {
		return
	}
	prefix := r.cfg.API.Prefix
	if prefix == "" {
		prefix = "/api/v1"
	}

	v1 := engine.Group(prefix)
	public := v1.Group("")
	if r.userAuth != nil {
		public.Use(r.userAuth.Optional())
	}

	protected := v1.Group("")
	permissionSet := apimiddleware.NewPermissionSet(r.apiEnforcer, nil)
	if r.userAuth != nil {
		protected.Use(r.userAuth.Required())
	}
	// Every protected route is governed by the canonical METHOD + path
	// identifier. PermissionSet deliberately returns 503 when its enforcer is
	// absent, so a configuration error never downgrades a protected endpoint to
	// authentication-only access.
	protected.Use(permissionSet.RequireRoute())

	r.installDemoRoutes(public, protected)
	// grove:register-routes
}
