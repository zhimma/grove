package server

import (
	"context"
	"errors"

	"github.com/zhimma/grove/app/worker/internal/handler"
	"github.com/zhimma/grove/internal/config"
	"github.com/zhimma/grove/internal/provider"
	"github.com/zhimma/grove/pkg/logger"
)

var ErrWorkerDisabled = errors.New("worker requires job or scheduler to be enabled")

type WorkerApp struct {
	provider *provider.Provider
}

func NewServer(cfg *config.Config) (*WorkerApp, func(), error) {
	if cfg != nil && !cfg.Job.Enabled && !cfg.Scheduler.Enabled {
		return nil, nil, ErrWorkerDisabled
	}
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
	if a.provider.Scheduler != nil {
		if err := a.provider.Scheduler.Start(); err != nil {
			return err
		}
	}
	if a.provider.JobServer == nil {
		if a.provider.Scheduler == nil {
			return ErrWorkerDisabled
		} else {
			logger.Info().Msg("工作进程已启动")
		}
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
