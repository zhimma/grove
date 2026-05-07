package bootstrap

import (
	"github.com/gin-gonic/gin"

	"github.com/zhimma/grove/internal/config"
	appmiddleware "github.com/zhimma/grove/internal/middleware"
)

type MiddlewareLoader struct {
	cfg         *config.Config
	serviceName string
}

func NewMiddlewareLoader(cfg *config.Config, serviceName string) *MiddlewareLoader {
	return &MiddlewareLoader{
		cfg:         cfg,
		serviceName: serviceName,
	}
}

func (l *MiddlewareLoader) Global() []gin.HandlerFunc {
	middlewares := []gin.HandlerFunc{
		appmiddleware.RequestID(),
		appmiddleware.RequestMeta(l.serviceName),
		appmiddleware.AccessLog(),
		appmiddleware.Recovery(),
	}
	if l.cfg != nil && l.cfg.CORS.Enabled {
		middlewares = append(middlewares, appmiddleware.CORS(l.cfg.CORS))
	}
	return middlewares
}
