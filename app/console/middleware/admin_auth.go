package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"

	consoleservice "github.com/zhimma/grove/app/console/service"
	"github.com/zhimma/grove/pkg/auth"
	pkgcasbin "github.com/zhimma/grove/pkg/casbin"
	pkgerrors "github.com/zhimma/grove/pkg/errors"
	"github.com/zhimma/grove/pkg/permission"
	"github.com/zhimma/grove/pkg/reqctx"
	"github.com/zhimma/grove/pkg/response"
	pkgroute "github.com/zhimma/grove/pkg/route"
)

type adminAuthResult struct {
	AdminID     string
	Username    string
	RoleID      string
	TokenString string
	IsSuper     bool
}

func writeAdminIdentity(c *gin.Context, result adminAuthResult) {
	reqctx.SetAuthToken(c, result.TokenString)
	reqctx.SetIdentity(c, reqctx.Identity{
		SubjectID:   result.AdminID,
		SubjectType: "console",
		UserID:      result.AdminID,
		AdminID:     result.AdminID,
		Username:    result.Username,
		RoleID:      result.RoleID,
		IsSuper:     result.IsSuper,
	})
}

func authenticateAdmin(c *gin.Context, tokenManager *auth.Manager, resolver consoleservice.AdminAuthStateResolver) (*adminAuthResult, bool) {
	header := strings.TrimSpace(c.GetHeader("Authorization"))
	if header == "" {
		response.Fail(c, pkgerrors.Unauthorized().WithMessage("缺少访问令牌"))
		c.Abort()
		return nil, false
	}

	tokenString := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	if tokenString == header && strings.HasPrefix(strings.ToLower(header), "bearer ") {
		tokenString = strings.TrimSpace(header[7:])
	}
	if tokenString == "" || tokenManager == nil {
		response.Fail(c, pkgerrors.Unauthorized().WithMessage("访问令牌无效"))
		c.Abort()
		return nil, false
	}

	claims, err := tokenManager.ParseAccessToken(tokenString)
	if err != nil || claims.UserType != "console" {
		response.Fail(c, pkgerrors.Unauthorized().WithMessage("控制台令牌无效").WithCause(err))
		c.Abort()
		return nil, false
	}
	if claims.AdminID == "" {
		response.Fail(c, pkgerrors.Unauthorized().WithMessage("控制台令牌无效"))
		c.Abort()
		return nil, false
	}

	state, err := resolver.ResolveAdminAuthState(c.Request.Context(), claims.AdminID)
	if err != nil {
		response.Fail(c, err)
		c.Abort()
		return nil, false
	}

	return &adminAuthResult{
		AdminID:     state.AdminID,
		Username:    state.Username,
		RoleID:      state.RoleID,
		TokenString: tokenString,
		IsSuper:     state.IsSuper,
	}, true
}

func AdminAuthn(tokenManager *auth.Manager, resolver consoleservice.AdminAuthStateResolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, ok := authenticateAdmin(c, tokenManager, resolver)
		if !ok {
			return
		}
		writeAdminIdentity(c, *result)
		c.Next()
	}
}

func AdminPermission(enforcer *pkgcasbin.Enforcer) gin.HandlerFunc {
	return func(c *gin.Context) {
		if reqctx.IsSuper(c) {
			c.Next()
			return
		}

		adminID := reqctx.GetAdminID(c)
		if adminID == "" {
			response.Fail(c, pkgerrors.Unauthorized().WithMessage("缺少管理员身份信息"))
			c.Abort()
			return
		}

		resource := c.FullPath()
		if resource == "" {
			resource = c.Request.URL.Path
		}
		if enforcer == nil || pkgroute.IsIgnored(c.Request.Method, resource) {
			c.Next()
			return
		}

		permissionIdentifier := permission.BuildAPIIdentifier(c.Request.Method, resource)
		allowed, err := enforcer.CheckConsolePermission(adminID, permissionIdentifier)
		if err != nil {
			response.Fail(c, pkgerrors.Internal().WithCause(err))
			c.Abort()
			return
		}
		if !allowed {
			response.Fail(c, pkgerrors.Forbidden().WithMessage("无权限访问该接口"))
			c.Abort()
			return
		}

		c.Next()
	}
}
