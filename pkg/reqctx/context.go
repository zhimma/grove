package reqctx

import (
	"context"

	"github.com/gin-gonic/gin"
)

const (
	RequestIDKey   = "request_id"
	RequestMetaKey = "request_meta"
	ErrorMetaKey   = "error_meta"
	IdentityKey    = "identity"
	AuthTokenKey   = "auth_token"
	AuditMetaKey   = "audit_meta"
)

type contextKey string

const (
	requestMetaStdKey contextKey = "request_meta"
	identityStdKey    contextKey = "identity"
)

type RequestMeta struct {
	RequestID string
	App       string
	Debug     bool
	Method    string
	Path      string
	Route     string
	ClientIP  string
	UserAgent string
}

type ErrorMeta struct {
	HTTPStatus    int
	Code          string
	Message       string
	InternalError bool
	HasCause      bool
}

type Identity struct {
	SubjectID   string
	SubjectType string
	UserID      string
	AdminID     string
	Username    string
	Email       string
	RoleID      string
	IsSuper     bool
}

type AuditMeta struct {
	TargetType string
	TargetID   string
	Detail     map[string]any
}

func SetRequestID(c *gin.Context, requestID string) {
	c.Set(RequestIDKey, requestID)
}

func GetRequestID(c *gin.Context) string {
	if c == nil {
		return ""
	}
	if value, exists := c.Get(RequestIDKey); exists {
		if requestID, ok := value.(string); ok {
			return requestID
		}
	}
	return ""
}

func SetRequestMeta(c *gin.Context, meta RequestMeta) {
	c.Set(RequestMetaKey, meta)
	SetRequestID(c, meta.RequestID)
	c.Request = c.Request.WithContext(WithRequestMeta(c.Request.Context(), meta))
}

func GetRequestMeta(c *gin.Context) RequestMeta {
	if c == nil {
		return RequestMeta{}
	}
	if value, exists := c.Get(RequestMetaKey); exists {
		if meta, ok := value.(RequestMeta); ok {
			return meta
		}
	}
	return RequestMeta{}
}

func WithRequestMeta(ctx context.Context, meta RequestMeta) context.Context {
	return context.WithValue(ctx, requestMetaStdKey, meta)
}

func GetRequestMetaFromContext(ctx context.Context) RequestMeta {
	if meta, ok := ctx.Value(requestMetaStdKey).(RequestMeta); ok {
		return meta
	}
	return RequestMeta{}
}

func SetErrorMeta(c *gin.Context, meta ErrorMeta) {
	if c == nil {
		return
	}
	c.Set(ErrorMetaKey, meta)
}

func GetErrorMeta(c *gin.Context) ErrorMeta {
	if c == nil {
		return ErrorMeta{}
	}
	if value, exists := c.Get(ErrorMetaKey); exists {
		if meta, ok := value.(ErrorMeta); ok {
			return meta
		}
	}
	return ErrorMeta{}
}

func SetIdentity(c *gin.Context, identity Identity) {
	c.Set(IdentityKey, identity)
	c.Request = c.Request.WithContext(WithIdentity(c.Request.Context(), identity))
}

func GetIdentity(c *gin.Context) Identity {
	if c == nil {
		return Identity{}
	}
	if value, exists := c.Get(IdentityKey); exists {
		if identity, ok := value.(Identity); ok {
			return identity
		}
	}
	return Identity{}
}

func SetUserID(c *gin.Context, userID string) {
	SetIdentity(c, Identity{
		SubjectID:   userID,
		SubjectType: "api",
		UserID:      userID,
	})
}

func GetUserID(c *gin.Context) string {
	return GetIdentity(c).UserID
}

func GetAdminID(c *gin.Context) string {
	return GetIdentity(c).AdminID
}

func IsSuper(c *gin.Context) bool {
	return GetIdentity(c).IsSuper
}

func SetAuthToken(c *gin.Context, token string) {
	c.Set(AuthTokenKey, token)
}

func GetAuthToken(c *gin.Context) string {
	if c == nil {
		return ""
	}
	if value, exists := c.Get(AuthTokenKey); exists {
		if token, ok := value.(string); ok {
			return token
		}
	}
	return ""
}

func SetAuditMeta(c *gin.Context, meta AuditMeta) {
	if c == nil {
		return
	}
	c.Set(AuditMetaKey, meta)
}

func GetAuditMeta(c *gin.Context) AuditMeta {
	if c == nil {
		return AuditMeta{}
	}
	if value, exists := c.Get(AuditMetaKey); exists {
		if meta, ok := value.(AuditMeta); ok {
			return meta
		}
	}
	return AuditMeta{}
}

func WithIdentity(ctx context.Context, identity Identity) context.Context {
	return context.WithValue(ctx, identityStdKey, identity)
}

func GetIdentityFromContext(ctx context.Context) Identity {
	if identity, ok := ctx.Value(identityStdKey).(Identity); ok {
		return identity
	}
	return Identity{}
}
