package service

import (
	"context"
	"strings"

	"github.com/zhimma/grove/pkg/auth"
	"github.com/zhimma/grove/pkg/errx"
)

type AuthService struct {
	tokenManager *auth.Manager
}

type IssueAccessTokenInput struct {
	UserID string
}

type IssueAccessTokenOutput struct {
	UserID      string
	AccessToken string
	TokenType   string
}

func NewAuthService(tokenManager *auth.Manager) *AuthService {
	return &AuthService{tokenManager: tokenManager}
}

func (s *AuthService) IssueAccessToken(_ context.Context, input IssueAccessTokenInput) (IssueAccessTokenOutput, error) {
	if s.tokenManager == nil {
		return IssueAccessTokenOutput{}, errx.ServiceUnavailable().WithMessage("令牌管理器未配置")
	}

	userID := strings.TrimSpace(input.UserID)
	if userID == "" {
		userID = "api-user"
	}

	token, err := s.tokenManager.IssueAccessToken(userID)
	if err != nil {
		return IssueAccessTokenOutput{}, errx.Internal().WithCause(err)
	}

	return IssueAccessTokenOutput{
		UserID:      userID,
		AccessToken: token,
		TokenType:   "Bearer",
	}, nil
}
