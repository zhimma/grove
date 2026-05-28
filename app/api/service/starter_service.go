package service

import (
	"context"
	"strings"

	"gorm.io/gorm"

	"github.com/zhimma/grove/internal/model"
	"github.com/zhimma/grove/pkg/errx"
	"github.com/zhimma/grove/pkg/job"
	"github.com/zhimma/grove/pkg/request"
)

type StarterService struct {
	db   *gorm.DB
	jobs *job.Client
}

type PingInput struct {
	Name string
}

type PingOutput struct {
	Message   string
	Service   string
	RequestID string
}

type ProfileInput struct {
	UserID string
}

type ProfileOutput struct {
	ID        string
	Name      string
	Email     string
	RequestID string
}

type DispatchEchoJobInput struct {
	UserID    string
	Message   string
	RequestID string
}

type DispatchEchoJobOutput struct {
	TaskID string
}

func NewStarterService(db *gorm.DB, jobs *job.Client) *StarterService {
	return &StarterService{
		db:   db,
		jobs: jobs,
	}
}

func (s *StarterService) Ping(ctx context.Context, input PingInput) (PingOutput, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = "world"
	}

	meta := request.GetRequestMetaFromContext(ctx)
	return PingOutput{
		Message:   "pong, " + name,
		Service:   meta.App,
		RequestID: meta.RequestID,
	}, nil
}

func (s *StarterService) Profile(ctx context.Context, input ProfileInput) (ProfileOutput, error) {
	user, err := model.FindUserByID(ctx, s.db, input.UserID)
	if err != nil {
		return ProfileOutput{}, err
	}

	meta := request.GetRequestMetaFromContext(ctx)
	return ProfileOutput{
		ID:        user.ID,
		Name:      user.Name,
		Email:     user.Email,
		RequestID: meta.RequestID,
	}, nil
}

func (s *StarterService) DispatchEchoJob(ctx context.Context, input DispatchEchoJobInput) (DispatchEchoJobOutput, error) {
	if s.jobs == nil {
		return DispatchEchoJobOutput{}, errx.ServiceUnavailable().WithMessage("任务客户端未启用")
	}
	message := strings.TrimSpace(input.Message)
	if message == "" {
		return DispatchEchoJobOutput{}, errx.InvalidParams().WithMessage("消息内容不能为空")
	}

	taskID, err := s.jobs.Enqueue(ctx, job.TaskEcho, job.EchoPayload{
		Message:     message,
		RequestedBy: input.UserID,
		RequestID:   input.RequestID,
	})
	if err != nil {
		return DispatchEchoJobOutput{}, errx.Internal().WithCause(err)
	}

	return DispatchEchoJobOutput{TaskID: taskID}, nil
}
