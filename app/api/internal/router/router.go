package router

import (
	"github.com/gin-gonic/gin"

	"github.com/zhimma/grove/app/api/handler"
	apimiddleware "github.com/zhimma/grove/app/api/middleware"
	"github.com/zhimma/grove/internal/config"
	"github.com/zhimma/grove/internal/provider"
)

type Router struct {
	cfg      *config.Config
	p        *provider.Provider
	userAuth *apimiddleware.UserAuthSet
}

func New(cfg *config.Config, p *provider.Provider) *Router {
	return &Router{
		cfg:      cfg,
		p:        p,
		userAuth: apimiddleware.NewUserAuthSet(p.TokenManager),
	}
}

func (r *Router) InstallToEngine(engine *gin.Engine) {
	prefix := r.cfg.API.Prefix
	if prefix == "" {
		prefix = "/api/v1"
	}

	v1 := engine.Group(prefix)
	public := v1.Group("")
	public.Use(r.userAuth.Optional())

	protected := v1.Group("")
	protected.Use(r.userAuth.Required())

	handler.RegisterAuthRoutes(public, r.p)
	handler.RegisterStarterRoutes(public, protected, r.p)
	// grove:register-routes
}