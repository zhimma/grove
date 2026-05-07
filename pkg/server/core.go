package server

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/zhimma/grove/internal/bootstrap"
	"github.com/zhimma/grove/internal/config"
	"github.com/zhimma/grove/internal/provider"
	"github.com/zhimma/grove/pkg/logger"
)

type CoreServer struct {
	Config   *config.Config
	Provider *provider.Provider
	Router   *gin.Engine
	Server   *http.Server
}

func NewCoreServer(cfg *config.Config, serviceName, port string, opts ...provider.Option) (*CoreServer, func(), error) {
	p, err := provider.New(cfg, serviceName, opts...)
	if err != nil {
		return nil, nil, err
	}

	if cfg.App.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	loader := bootstrap.NewMiddlewareLoader(cfg, serviceName)
	router.Use(loader.Global()...)
	registerHealthCheck(router, serviceName)

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           router,
		ReadTimeout:       time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout:      time.Duration(cfg.Server.WriteTimeout) * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		MaxHeaderBytes:    cfg.Server.MaxHeaderBytes,
	}

	return &CoreServer{
			Config:   cfg,
			Provider: p,
			Router:   router,
			Server:   srv,
		}, func() {
			_ = p.Close()
		}, nil
}

func registerHealthCheck(router *gin.Engine, serviceName string) {
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"service": serviceName,
		})
	})
}

func (s *CoreServer) Start(name string) error {
	go func() {
		logger.Info().Str("addr", s.Server.Addr).Str("server", name).Msg("服务启动中")
		if err := s.Server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal().Err(err).Str("server", name).Msg("服务异常停止")
		}
	}()
	return nil
}

func (s *CoreServer) Stop(ctx context.Context) error {
	timeout := time.Duration(s.Config.Server.ShutdownTimeout) * time.Second
	shutdownCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return s.Server.Shutdown(shutdownCtx)
}
