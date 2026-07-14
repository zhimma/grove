package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/zhimma/grove/app/api/service"
	"github.com/zhimma/grove/internal/provider"
	"github.com/zhimma/grove/pkg/response"
	"github.com/zhimma/grove/pkg/validation"
)

type DemoAuthHandler struct {
	authSvc *service.DemoAuthService
}

type IssueAccessTokenRequest struct {
	UserID string `json:"user_id" binding:"required" label:"用户ID"`
}

type IssueAccessTokenResponse struct {
	UserID      string `json:"user_id"`
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
}

func RegisterDemoAuthRoutes(public *gin.RouterGroup, p *provider.Provider) {
	h := &DemoAuthHandler{
		authSvc: service.NewDemoAuthService(p.TokenManager),
	}
	public.POST("/auth/access-token", h.IssueAccessToken)
}

func (h *DemoAuthHandler) IssueAccessToken(c *gin.Context) {
	var req IssueAccessTokenRequest
	if c.Request.ContentLength > 0 {
		if err := validation.BindJSON(c, &req); err != nil {
			response.Fail(c, err)
			return
		}
	}

	out, err := h.authSvc.IssueAccessToken(c.Request.Context(), service.IssueAccessTokenInput{
		UserID: req.UserID,
	})
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, IssueAccessTokenResponse{
		UserID:      out.UserID,
		AccessToken: out.AccessToken,
		TokenType:   out.TokenType,
	})
}
