package server

import (
	"context"

	apidocs "github.com/zhimma/grove/app/api/internal/docs"
	"github.com/zhimma/grove/app/api/internal/router"
	"github.com/zhimma/grove/internal/config"
	"github.com/zhimma/grove/internal/provider"
	pkgserver "github.com/zhimma/grove/pkg/server"
)

type APIApp struct {
	*pkgserver.CoreServer
}

func NewServer(cfg *config.Config) (*APIApp, func(), error) {
	base, cleanup, err := pkgserver.NewCoreServer(
		cfg,
		"api",
		cfg.Port,
		provider.APIOptions()...,
	)
	if err != nil {
		return nil, nil, err
	}

	apidocs.RegisterDocs(base.Router, cfg)
	router.New(cfg, base.Provider).InstallToEngine(base.Router)

	return &APIApp{CoreServer: base}, cleanup, nil
}

func (a *APIApp) Start() error {
	return a.CoreServer.Start("api")
}

func (a *APIApp) Stop(ctx context.Context) error {
	return a.CoreServer.Stop(ctx)
}