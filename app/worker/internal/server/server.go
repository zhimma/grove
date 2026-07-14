package server

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/zhimma/grove/app/worker/internal/handler"
	"github.com/zhimma/grove/internal/config"
	"github.com/zhimma/grove/internal/provider"
	"github.com/zhimma/grove/pkg/logger"
	pkgserver "github.com/zhimma/grove/pkg/server"
)

var ErrWorkerDisabled = errors.New("worker requires job or scheduler to be enabled")

type WorkerApp struct {
	provider *provider.Provider
	health   *pkgserver.CoreServer
	errors   chan error
	done     chan struct{}
	stopOnce sync.Once
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
	health, err := pkgserver.NewHealthServer(cfg, "worker", cfg.WorkerPort, p)
	if err != nil {
		_ = p.Close()
		return nil, nil, err
	}
	app := &WorkerApp{provider: p, health: health, errors: make(chan error, 2), done: make(chan struct{})}
	return app, func() {
		_ = app.Stop(context.Background())
	}, nil
}

func (a *WorkerApp) Start() error {
	if a == nil || a.health == nil {
		return fmt.Errorf("worker health server is not configured")
	}
	if err := a.health.Start("worker-health"); err != nil {
		return err
	}
	go func() {
		select {
		case err := <-a.health.Errors():
			if err != nil {
				a.reportError(fmt.Errorf("worker health server: %w", err))
			}
		case <-a.done:
		}
	}()
	if a.provider.Scheduler != nil {
		if err := a.provider.Scheduler.Start(); err != nil {
			_ = a.health.Stop(context.Background())
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
			a.reportError(fmt.Errorf("job server: %w", err))
		}
	}()
	logger.Info().Msg("工作进程已启动")
	return nil
}

func (a *WorkerApp) Errors() <-chan error {
	if a == nil {
		return nil
	}
	return a.errors
}

func (a *WorkerApp) reportError(err error) {
	if a == nil || err == nil {
		return
	}
	select {
	case a.errors <- err:
	default:
	}
}

func (a *WorkerApp) Stop(ctx context.Context) error {
	if a == nil {
		return nil
	}
	a.stopOnce.Do(func() { close(a.done) })
	if a.health != nil {
		return a.health.Stop(ctx)
	}
	if a.provider != nil {
		return a.provider.Close()
	}
	return nil
}
