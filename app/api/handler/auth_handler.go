package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/zhimma/grove/app/api/service"
	"github.com/zhimma/grove/pkg/auth"
	"github.com/zhimma/grove/pkg/response"
	"github.com/zhimma/grove/pkg/route"
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

// RegisterDemoAuthRoutes registers the demo authentication endpoint with its
// exact runtime dependency. Provider ownership stays in the API router.
func RegisterDemoAuthRoutes(public *route.Group, tokenManager *auth.Manager) {
	h := &DemoAuthHandler{
		authSvc: service.NewDemoAuthService(tokenManager),
	}
	public.POST("/auth/access-token", h.IssueAccessToken).Name("示例.签发访问令牌").Ignore()
}

func (h *DemoAuthHandler) IssueAccessToken(c *gin.Context) {
	var req IssueAccessTokenRequest
	if c.Request.ContentLength > 0 {
		if err := validation.BindJSONStrict(c, &req); err != nil {
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
