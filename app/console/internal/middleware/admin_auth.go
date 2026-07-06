package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"

	consoleservice "github.com/zhimma/grove/app/console/internal/service"
	"github.com/zhimma/grove/pkg/auth"
	"github.com/zhimma/grove/pkg/rbac"
	"github.com/zhimma/grove/pkg/errx"
	"github.com/zhimma/grove/pkg/permission"
	"github.com/zhimma/grove/pkg/request"
	"github.com/zhimma/grove/pkg/response"
	pkgroute "github.com/zhimma/grove/pkg/route"
)

func AdminAuthn(tokenManager *auth.Manager, resolver consoleservice.AdminAuthStateResolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString, ok := auth.ExtractBearer(c.GetHeader("Authorization"))
		if !ok || tokenManager == nil {
			response.Fail(c, errx.Unauthorized().WithMessage("缺少访问令牌"))
			c.Abort()
			return
		}

		claims, err := tokenManager.ParseAccessToken(tokenString)
		if err != nil || claims.UserType != "console" || claims.AdminID == "" {
			response.Fail(c, errx.Unauthorized().WithMessage("控制台令牌无效").WithCause(err))
			c.Abort()
			return
		}

		state, err := resolver.ResolveAdminAuthState(c.Request.Context(), claims.AdminID)
		if err != nil {
			response.Fail(c, err)
			c.Abort()
			return
		}

		request.SetAuthToken(c, tokenString)
		request.SetIdentity(c, request.Identity{
			SubjectID:   state.AdminID,
			SubjectType: "console",
			UserID:      state.AdminID,
			AdminID:     state.AdminID,
			Username:    state.Username,
			RoleID:      state.RoleID,
			IsSuper:     state.IsSuper,
		})
		c.Next()
	}
}

func AdminPermission(enforcer *rbac.Enforcer, env ...string) gin.HandlerFunc {
	appEnv := ""
	if len(env) > 0 {
		appEnv = strings.TrimSpace(env[0])
	}
	return func(c *gin.Context) {
		if request.IsSuper(c) {
			c.Next()
			return
		}

		adminID := request.GetAdminID(c)
		if adminID == "" {
			response.Fail(c, errx.Unauthorized().WithMessage("缺少管理员身份信息"))
			c.Abort()
			return
		}

		resource := c.FullPath()
		if resource == "" {
			resource = c.Request.URL.Path
		}
		if pkgroute.IsIgnored(c.Request.Method, resource) {
			c.Next()
			return
		}
		if enforcer == nil {
			if strings.EqualFold(appEnv, "production") {
				response.Fail(c, errx.ServiceUnavailable().WithMessage("权限控制器未配置"))
				c.Abort()
				return
			}
			c.Next()
			return
		}

		permissionIdentifier := permission.BuildAPIIdentifier(c.Request.Method, resource)
		allowed, err := enforcer.Can(adminID, permissionIdentifier)
		if err != nil {
			response.Fail(c, errx.Internal().WithCause(err))
			c.Abort()
			return
		}
		if !allowed {
			response.Fail(c, errx.Forbidden().WithMessage("无权限访问该接口"))
			c.Abort()
			return
		}

		c.Next()
	}
}
