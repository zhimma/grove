package service

import (
	"context"
	"strings"

	"github.com/zhimma/grove/pkg/auth"
	"github.com/zhimma/grove/pkg/errx"
)

type DemoAuthService struct {
	tokens *auth.Tokens
}

type IssueAccessTokenInput struct {
	UserID string
}

type IssueAccessTokenOutput struct {
	UserID      string
	AccessToken string
	TokenType   string
}

func NewDemoAuthService(tokens *auth.Tokens) *DemoAuthService {
	return &DemoAuthService{tokens: tokens}
}

func (s *DemoAuthService) IssueAccessToken(_ context.Context, input IssueAccessTokenInput) (IssueAccessTokenOutput, error) {
	if s.tokens == nil {
		return IssueAccessTokenOutput{}, errx.ServiceUnavailable().WithMessage("令牌管理器未配置")
	}

	userID := strings.TrimSpace(input.UserID)
	if userID == "" {
		userID = "api-user"
	}

	token, err := s.tokens.IssueAccessToken(userID)
	if err != nil {
		return IssueAccessTokenOutput{}, errx.Internal().WithCause(err)
	}

	return IssueAccessTokenOutput{
		UserID:      userID,
		AccessToken: token,
		TokenType:   "Bearer",
	}, nil
}
