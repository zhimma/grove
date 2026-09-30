package server

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/zhimma/grove/app/worker/internal/handler"
	"github.com/zhimma/grove/app/worker/internal/task"
	"github.com/zhimma/grove/internal/config"
	"github.com/zhimma/grove/internal/provider"
	coreserver "github.com/zhimma/grove/internal/server"
	"github.com/zhimma/grove/pkg/logger"
)

var ErrWorkerDisabled = errors.New("worker requires job or scheduler to be enabled")

type WorkerApp struct {
	provider    *provider.Provider
	health      *coreserver.CoreServer
	reconciler  *task.Reconciler
	errors      chan error
	done        chan struct{}
	stopOnce    sync.Once
	stopTasks   context.CancelFunc
	tasksClosed chan struct{}
}

func NewServer(cfg *config.Config) (*WorkerApp, func(), error) {
	if cfg != nil && !cfg.Job.Enabled && !cfg.Scheduler.Enabled {
		return nil, nil, ErrWorkerDisabled
	}
	p, err := provider.New(cfg, "worker", provider.WorkerOptions()...)
	if err != nil {
		return nil, nil, err
	}

	handler.RegisterEchoJob(p.JobServer)

	// The scheduler is driven by console_scheduled_tasks, so a task the code
	// does not define cannot be introduced from the console. Without a database
	// there is no schedule table: the scheduler still runs tasks registered
	// directly in code, they just cannot be managed from the console.
	var reconciler *task.Reconciler
	hasDatabase := p.DB != nil && p.DB.Default() != nil
	if p.Scheduler != nil && !hasDatabase {
		logger.Warn().Msg("未配置数据库，计划任务无法在后台管理")
	}
	if p.Scheduler != nil && hasDatabase {
		definitions, definitionsErr := task.Definitions(p.DB)
		if definitionsErr != nil {
			_ = p.Close()
			return nil, nil, definitionsErr
		}
		reconciler, err = task.NewReconciler(p.DB, p.Scheduler, definitions, 0)
		if err != nil {
			_ = p.Close()
			return nil, nil, err
		}
	}

	health, err := coreserver.NewHealthServer(cfg, "worker", cfg.WorkerPort, p)
	if err != nil {
		_ = p.Close()
		return nil, nil, err
	}
	app := &WorkerApp{
		provider:   p,
		health:     health,
		reconciler: reconciler,
		errors:     make(chan error, 2),
		done:       make(chan struct{}),
	}
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
	if a.reconciler != nil {
		tasksCtx, cancel := context.WithCancel(context.Background())
		a.stopTasks = cancel
		a.tasksClosed = make(chan struct{})
		go func() {
			defer close(a.tasksClosed)
			a.reconciler.Run(tasksCtx)
		}()
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
	a.stopOnce.Do(func() {
		close(a.done)
		if a.stopTasks != nil {
			a.stopTasks()
		}
	})
	if a.tasksClosed != nil {
		select {
		case <-a.tasksClosed:
		case <-ctx.Done():
		}
	}
	if a.health != nil {
		return a.health.Stop(ctx)
	}
	if a.provider != nil {
		return a.provider.Close()
	}
	return nil
}
