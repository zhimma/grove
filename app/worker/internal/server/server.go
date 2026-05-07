package server

import (
	"context"

	"github.com/zhimma/grove/app/worker/handler"
	"github.com/zhimma/grove/internal/config"
	"github.com/zhimma/grove/internal/provider"
	"github.com/zhimma/grove/pkg/logger"
)

type WorkerApp struct {
	provider *provider.Provider
}

func NewServer(cfg *config.Config) (*WorkerApp, func(), error) {
	p, err := provider.New(cfg, "worker", provider.WorkerOptions()...)
	if err != nil {
		return nil, nil, err
	}

	handler.RegisterDefaultJobs(p.JobServer)
	return &WorkerApp{provider: p}, func() {
		_ = p.Close()
	}, nil
}

func (a *WorkerApp) Start() error {
	if a.provider.JobServer == nil {
		logger.Warn().Msg("任务服务未启用")
		return nil
	}

	go func() {
		if err := a.provider.JobServer.Run(); err != nil {
			logger.Fatal().Err(err).Msg("工作进程异常停止")
		}
	}()
	logger.Info().Msg("工作进程已启动")
	return nil
}

func (a *WorkerApp) Stop(_ context.Context) error {
	if a.provider != nil {
		return a.provider.Close()
	}
	return nil
}
