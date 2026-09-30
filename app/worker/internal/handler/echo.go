package handler

import (
	"context"

	"github.com/hibiken/asynq"

	"github.com/zhimma/grove/internal/jobtask"
	"github.com/zhimma/grove/pkg/job"
	"github.com/zhimma/grove/pkg/logger"
)

func RegisterEchoJob(server *job.Server) {
	if server == nil {
		return
	}
	if err := server.Register(jobtask.TaskEcho, handleEcho); err != nil {
		logger.Error().Err(err).Str("task_type", jobtask.TaskEcho).Msg("注册默认任务失败")
	}
}

func handleEcho(_ context.Context, task *asynq.Task) error {
	var payload jobtask.EchoPayload
	if err := job.ParsePayload(task, &payload); err != nil {
		return err
	}

	logger.Info().
		Str("task_type", jobtask.TaskEcho).
		Str("requested_by", payload.RequestedBy).
		Str("request_id", payload.RequestID).
		Str("message", payload.Message).
		Msg("echo 任务已处理")
	return nil
}
