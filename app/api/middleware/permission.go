package middleware

import (
	"fmt"

	"github.com/gin-gonic/gin"

	pkgcasbin "github.com/zhimma/grove/pkg/casbin"
	pkgerrors "github.com/zhimma/grove/pkg/errors"
	"github.com/zhimma/grove/pkg/reqctx"
	"github.com/zhimma/grove/pkg/response"
)

type PermissionSet struct {
	enforcer       *pkgcasbin.Enforcer
	domainResolver func(*gin.Context) (string, error)
}

func NewPermissionSet(enforcer *pkgcasbin.Enforcer, domainResolver func(*gin.Context) (string, error)) *PermissionSet {
	return &PermissionSet{
		enforcer:       enforcer,
		domainResolver: domainResolver,
	}
}

func (s *PermissionSet) Require(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if s == nil || s.enforcer == nil {
			response.Fail(c, pkgerrors.ServiceUnavailable().WithMessage("权限控制器未配置"))
			c.Abort()
			return
		}

		userID := reqctx.GetUserID(c)
		if userID == "" {
			response.Fail(c, pkgerrors.Unauthorized().WithMessage("缺少用户身份信息"))
			c.Abort()
			return
		}

		var (
			allowed bool
			err     error
		)
		if s.enforcer.Mode() == pkgcasbin.ModeRBACDomains {
			if s.domainResolver == nil {
				response.Fail(c, pkgerrors.ServiceUnavailable().WithMessage("权限域解析器未配置"))
				c.Abort()
				return
			}
			domain, resolveErr := s.domainResolver(c)
			if resolveErr != nil {
				response.Fail(c, pkgerrors.InvalidParams().WithMessage(resolveErr.Error()))
				c.Abort()
				return
			}
			allowed, err = s.enforcer.CanInDomain(domain, userID, permission)
		} else {
			allowed, err = s.enforcer.Can(userID, permission)
		}
		if err != nil {
			response.Fail(c, pkgerrors.Internal().WithCause(err))
			c.Abort()
			return
		}
		if !allowed {
			response.Fail(c, pkgerrors.Forbidden().WithMessage(fmt.Sprintf("无权限访问: %s", permission)))
			c.Abort()
			return
		}

		c.Next()
	}
}
