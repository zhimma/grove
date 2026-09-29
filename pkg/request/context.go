package request

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

const requestMetaStdKey contextKey = "request_meta"

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
	SessionID   string
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
	if c == nil {
		return
	}
	c.Set(RequestIDKey, requestID)
}

func RequestID(c *gin.Context) string {
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
	if c == nil {
		return
	}
	c.Set(RequestMetaKey, meta)
	SetRequestID(c, meta.RequestID)
	// Services only receive a context.Context, so the meta is mirrored there.
	// A context built without a Request still gets the gin-side value.
	if c.Request != nil {
		c.Request = c.Request.WithContext(WithRequestMeta(c.Request.Context(), meta))
	}
}

func RequestMetaOf(c *gin.Context) RequestMeta {
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

func RequestMetaFromContext(ctx context.Context) RequestMeta {
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

func ErrorMetaOf(c *gin.Context) ErrorMeta {
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
	if c == nil {
		return
	}
	c.Set(IdentityKey, identity)
}

func IdentityOf(c *gin.Context) Identity {
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

func UserID(c *gin.Context) string {
	return IdentityOf(c).UserID
}

func AdminID(c *gin.Context) string {
	return IdentityOf(c).AdminID
}

func SessionID(c *gin.Context) string {
	return IdentityOf(c).SessionID
}

func IsSuper(c *gin.Context) bool {
	return IdentityOf(c).IsSuper
}

func SetAuthToken(c *gin.Context, token string) {
	if c == nil {
		return
	}
	c.Set(AuthTokenKey, token)
}

func AuthToken(c *gin.Context) string {
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

func AuditMetaOf(c *gin.Context) AuditMeta {
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
