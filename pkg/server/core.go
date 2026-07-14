package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
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

	startMu     sync.Mutex
	listener    net.Listener
	started     bool
	serveErrors chan error
}

func NewCoreServer(cfg *config.Config, serviceName, port string, opts ...provider.Option) (*CoreServer, func(), error) {
	p, err := provider.New(cfg, serviceName, opts...)
	if err != nil {
		return nil, nil, err
	}

	if strings.EqualFold(strings.TrimSpace(cfg.App.Env), "production") {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	if err := router.SetTrustedProxies(cfg.Security.TrustedProxies); err != nil {
		_ = p.Close()
		return nil, nil, fmt.Errorf("configure trusted proxies: %w", err)
	}
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

	core := &CoreServer{
		Config:      cfg,
		Provider:    p,
		Router:      router,
		Server:      srv,
		serveErrors: make(chan error, 1),
	}
	cleanup := func() {
		_ = p.Close()
	}
	return core, cleanup, nil
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
	if s == nil || s.Server == nil {
		return fmt.Errorf("server is not configured")
	}
	s.startMu.Lock()
	defer s.startMu.Unlock()
	if s.started {
		return fmt.Errorf("server %q is already started", name)
	}
	listener, err := net.Listen("tcp", s.Server.Addr)
	if err != nil {
		return fmt.Errorf("listen %s on %s: %w", name, s.Server.Addr, err)
	}
	s.listener = listener
	s.started = true

	logger.Info().Str("addr", listener.Addr().String()).Str("server", name).Msg("服务启动中")
	go func() {
		if err := s.Server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error().Err(err).Str("server", name).Msg("服务异常停止")
			select {
			case s.serveErrors <- err:
			default:
			}
		}
	}()
	return nil
}

func (s *CoreServer) Errors() <-chan error {
	if s == nil {
		return nil
	}
	return s.serveErrors
}

func (s *CoreServer) Stop(ctx context.Context) error {
	if s == nil {
		return nil
	}
	timeout := time.Duration(s.Config.Server.ShutdownTimeout) * time.Second
	shutdownCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var errs []error
	if err := s.Server.Shutdown(shutdownCtx); err != nil {
		errs = append(errs, fmt.Errorf("shutdown http server: %w", err))
	}
	if s.Provider != nil {
		if err := s.Provider.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close provider: %w", err))
		}
	}
	return errors.Join(errs...)
}
