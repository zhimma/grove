package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"

	consoleservice "github.com/zhimma/grove/app/console/internal/service"
	"github.com/zhimma/grove/pkg/auth"
	"github.com/zhimma/grove/pkg/errx"
	"github.com/zhimma/grove/pkg/permission"
	"github.com/zhimma/grove/pkg/rbac"
	"github.com/zhimma/grove/pkg/request"
	"github.com/zhimma/grove/pkg/response"
	pkgroute "github.com/zhimma/grove/pkg/route"
)

type adminAuthResult struct {
	AdminID     string
	SessionID   string
	Username    string
	RoleID      string
	TokenString string
	IsSuper     bool
}

func writeAdminIdentity(c *gin.Context, result adminAuthResult) {
	request.SetAuthToken(c, result.TokenString)
	request.SetIdentity(c, request.Identity{
		SubjectID:   result.AdminID,
		SubjectType: "console",
		UserID:      result.AdminID,
		AdminID:     result.AdminID,
		SessionID:   result.SessionID,
		Username:    result.Username,
		RoleID:      result.RoleID,
		IsSuper:     result.IsSuper,
	})
}

func authenticateAdmin(c *gin.Context, tokenManager *auth.Manager, sessions *consoleservice.SessionService, resolver consoleservice.AdminAuthStateResolver) (*adminAuthResult, bool) {
	header := strings.TrimSpace(c.GetHeader("Authorization"))
	if header == "" {
		response.Fail(c, errx.Unauthorized().WithMessage("缺少访问令牌"))
		c.Abort()
		return nil, false
	}

	tokenString := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	if tokenString == header && strings.HasPrefix(strings.ToLower(header), "bearer ") {
		tokenString = strings.TrimSpace(header[7:])
	}
	if tokenString == "" || tokenManager == nil {
		response.Fail(c, errx.Unauthorized().WithMessage("访问令牌无效"))
		c.Abort()
		return nil, false
	}

	claims, err := tokenManager.ParseAccessTokenForUserType(tokenString, auth.UserTypeConsole)
	if err != nil {
		response.Fail(c, errx.Unauthorized().WithMessage("控制台令牌无效").WithCause(err))
		c.Abort()
		return nil, false
	}
	if claims.AdminID == "" || claims.SessionID == "" {
		response.Fail(c, errx.Unauthorized().WithMessage("控制台令牌无效"))
		c.Abort()
		return nil, false
	}
	if sessions == nil {
		response.Fail(c, errx.ServiceUnavailable().WithMessage("会话服务未配置"))
		c.Abort()
		return nil, false
	}
	if _, err := sessions.Validate(c.Request.Context(), claims.AdminID, claims.SessionID); err != nil {
		response.Fail(c, err)
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
		SessionID:   claims.SessionID,
		Username:    state.Username,
		RoleID:      state.RoleID,
		TokenString: tokenString,
		IsSuper:     state.IsSuper,
	}, true
}

func AdminAuthn(tokenManager *auth.Manager, sessions *consoleservice.SessionService, resolver consoleservice.AdminAuthStateResolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, ok := authenticateAdmin(c, tokenManager, sessions, resolver)
		if !ok {
			return
		}
		writeAdminIdentity(c, *result)
		c.Next()
	}
}

func AdminPermission(enforcer *rbac.Enforcer, catalog *pkgroute.Catalog) gin.HandlerFunc {
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
		if catalog.IsIgnored(c.Request.Method, resource) {
			c.Next()
			return
		}
		if enforcer == nil {
			response.Fail(c, errx.ServiceUnavailable().WithMessage("权限控制器未配置"))
			c.Abort()
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
