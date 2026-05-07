package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/zhimma/grove/pkg/auth"
	pkgerrors "github.com/zhimma/grove/pkg/errors"
	"github.com/zhimma/grove/pkg/reqctx"
	"github.com/zhimma/grove/pkg/response"
)

type UserAuthSet struct {
	tokenManager *auth.Manager
}

func NewUserAuthSet(tokenManager *auth.Manager) *UserAuthSet {
	return &UserAuthSet{tokenManager: tokenManager}
}

func (s *UserAuthSet) Optional() gin.HandlerFunc {
	return s.authenticate(false)
}

func (s *UserAuthSet) Required() gin.HandlerFunc {
	return s.authenticate(true)
}

func (s *UserAuthSet) authenticate(required bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := strings.TrimSpace(c.GetHeader("Authorization"))
		if header == "" {
			if required {
				response.Fail(c, pkgerrors.Unauthorized().WithMessage("缺少访问令牌"))
				c.Abort()
				return
			}
			c.Next()
			return
		}

		tokenString := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
		if tokenString == header && strings.HasPrefix(strings.ToLower(header), "bearer ") {
			tokenString = strings.TrimSpace(header[7:])
		}
		if tokenString == "" || s.tokenManager == nil {
			if required {
				response.Fail(c, pkgerrors.Unauthorized().WithMessage("访问令牌无效"))
				c.Abort()
				return
			}
			c.Next()
			return
		}

		claims, err := s.tokenManager.ParseAccessToken(tokenString)
		if err != nil {
			if required {
				response.Fail(c, pkgerrors.Unauthorized().WithMessage("访问令牌无效").WithCause(err))
				c.Abort()
				return
			}
			c.Next()
			return
		}

		reqctx.SetAuthToken(c, tokenString)
		reqctx.SetIdentity(c, reqctx.Identity{
			SubjectID:   claims.UserID,
			SubjectType: claims.UserType,
			UserID:      claims.UserID,
			Email:       claims.Email,
			RoleID:      claims.RoleID,
			IsSuper:     claims.IsSuper,
		})
		c.Next()
	}
}
