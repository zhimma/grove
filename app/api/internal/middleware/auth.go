package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/zhimma/grove/pkg/auth"
	"github.com/zhimma/grove/pkg/errx"
	"github.com/zhimma/grove/pkg/request"
	"github.com/zhimma/grove/pkg/response"
)

type UserAuthSet struct {
	tokens           *auth.Tokens
	expectedUserType string
}

func NewUserAuthSet(tokens *auth.Tokens, userType ...string) *UserAuthSet {
	expected := auth.UserTypeAPI
	if len(userType) > 0 && strings.TrimSpace(userType[0]) != "" {
		expected = strings.ToLower(strings.TrimSpace(userType[0]))
	}
	return &UserAuthSet{tokens: tokens, expectedUserType: expected}
}

func (s *UserAuthSet) Optional() gin.HandlerFunc {
	return s.authenticate(false)
}

func (s *UserAuthSet) Required() gin.HandlerFunc {
	return s.authenticate(true)
}

func (s *UserAuthSet) authenticate(required bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if s == nil {
			if required {
				response.Fail(c, errx.Unauthorized().WithMessage("访问令牌无效"))
				c.Abort()
				return
			}
			c.Next()
			return
		}
		tokenString, ok := auth.ExtractBearer(c.GetHeader("Authorization"))
		if !ok {
			if required {
				response.Fail(c, errx.Unauthorized().WithMessage("缺少访问令牌"))
				c.Abort()
				return
			}
			c.Next()
			return
		}

		if s.tokens == nil {
			if required {
				response.Fail(c, errx.Unauthorized().WithMessage("访问令牌无效"))
				c.Abort()
				return
			}
			c.Next()
			return
		}

		claims, err := s.tokens.ParseAccessTokenForUserType(tokenString, s.expectedUserType)
		if err != nil {
			if required {
				response.Fail(c, errx.Unauthorized().WithMessage("访问令牌无效").WithCause(err))
				c.Abort()
				return
			}
			c.Next()
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
