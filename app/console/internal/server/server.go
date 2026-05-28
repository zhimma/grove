package server

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/zhimma/grove/app/console/internal/docs"
	"github.com/zhimma/grove/app/console/internal/router"
	"github.com/zhimma/grove/internal/config"
	"github.com/zhimma/grove/internal/provider"
	pkgserver "github.com/zhimma/grove/pkg/server"
)

type ConsoleApp struct {
	*pkgserver.CoreServer
}

func NewServer(cfg *config.Config) (*ConsoleApp, func(), error) {
	base, cleanup, err := pkgserver.NewCoreServer(
		cfg,
		"console",
		cfg.ConsolePort,
		provider.ConsoleOptions()...,
	)
	if err != nil {
		return nil, nil, err
	}

	registerLocalStorageRoutes(base.Router, cfg)

	docs.RegisterDocs(base.Router, cfg)
	router.New(cfg, base.Provider).InstallToEngine(base.Router)

	return &ConsoleApp{CoreServer: base}, cleanup, nil
}

func (a *ConsoleApp) Start() error {
	return a.CoreServer.Start("console")
}

func (a *ConsoleApp) Stop(ctx context.Context) error {
	return a.CoreServer.Stop(ctx)
}

func registerLocalStorageRoutes(engine *gin.Engine, cfg *config.Config) {
	if engine == nil || cfg == nil {
		return
	}

	for _, disk := range cfg.Storage.Disks {
		if strings.TrimSpace(strings.ToLower(disk.Driver)) != "local" {
			continue
		}
		if strings.TrimSpace(disk.BaseURL) == "" || strings.TrimSpace(disk.Root) == "" {
			continue
		}
		engine.Static(strings.TrimRight(disk.BaseURL, "/"), disk.Root)
	}
}