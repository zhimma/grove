package middleware

import (
	"github.com/gin-gonic/gin"

	"github.com/zhimma/grove/pkg/auth"
	"github.com/zhimma/grove/pkg/errx"
	"github.com/zhimma/grove/pkg/request"
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
		tokenString, ok := auth.ExtractBearer(c.GetHeader("Authorization"))
		if !ok {
			if required {
				response.Fail(c, errx.Unauthorized().WithMessage("缺少访问令牌"))
				c.Abort()
			}
			return
		}
		if s.tokenManager == nil {
			if required {
				response.Fail(c, errx.Unauthorized().WithMessage("访问令牌无效"))
				c.Abort()
			}
			return
		}

		claims, err := s.tokenManager.ParseAccessToken(tokenString)
		if err != nil {
			if required {
				response.Fail(c, errx.Unauthorized().WithMessage("访问令牌无效").WithCause(err))
				c.Abort()
			}
			return
		}

		request.SetAuthToken(c, tokenString)
		request.SetIdentity(c, request.Identity{
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
