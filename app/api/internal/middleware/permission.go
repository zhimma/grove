package middleware

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/zhimma/grove/pkg/errx"
	permissionpkg "github.com/zhimma/grove/pkg/permission"
	"github.com/zhimma/grove/pkg/rbac"
	"github.com/zhimma/grove/pkg/request"
	"github.com/zhimma/grove/pkg/response"
)

type PermissionSet struct {
	enforcer       *rbac.Enforcer
	domainResolver func(*gin.Context) (string, error)
}

func NewPermissionSet(enforcer *rbac.Enforcer, domainResolver func(*gin.Context) (string, error)) *PermissionSet {
	return &PermissionSet{
		enforcer:       enforcer,
		domainResolver: domainResolver,
	}
}

func (s *PermissionSet) Require(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if s == nil || s.enforcer == nil {
			response.Fail(c, errx.ServiceUnavailable().WithMessage("权限控制器未配置"))
			c.Abort()
			return
		}

		userID := request.GetUserID(c)
		if userID == "" {
			response.Fail(c, errx.Unauthorized().WithMessage("缺少用户身份信息"))
			c.Abort()
			return
		}

		permission = strings.TrimSpace(permission)
		if permission == "" {
			resource := c.FullPath()
			if resource == "" {
				resource = c.Request.URL.Path
			}
			permission = permissionpkg.BuildAPIIdentifier(c.Request.Method, resource)
		}
		if permission == "" {
			response.Fail(c, errx.ServiceUnavailable().WithMessage("权限标识未配置"))
			c.Abort()
			return
		}

		var (
			allowed bool
			err     error
		)
		if s.enforcer.Mode() == rbac.ModeRBACDomains {
			if s.domainResolver == nil {
				response.Fail(c, errx.ServiceUnavailable().WithMessage("权限域解析器未配置"))
				c.Abort()
				return
			}
			domain, resolveErr := s.domainResolver(c)
			if resolveErr != nil {
				response.Fail(c, errx.InvalidParams().WithMessage(resolveErr.Error()))
				c.Abort()
				return
			}
			allowed, err = s.enforcer.CanInDomain(domain, userID, permission)
		} else {
			allowed, err = s.enforcer.Can(userID, permission)
		}
		if err != nil {
			response.Fail(c, errx.Internal().WithCause(err))
			c.Abort()
			return
		}
		if !allowed {
			response.Fail(c, errx.Forbidden().WithMessage(fmt.Sprintf("无权限访问: %s", permission)))
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireRoute derives the canonical METHOD + full path permission identifier
// at request time. It is useful for a protected router group where every
// registered route is expected to be governed by the same fail-closed policy.
func (s *PermissionSet) RequireRoute() gin.HandlerFunc {
	return s.Require("")
}
