package request

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func newCtx() (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/x", nil)
	return c, w
}

func TestRequestID_Roundtrip(t *testing.T) {
	c, _ := newCtx()
	SetRequestID(c, "req_001")
	if got := GetRequestID(c); got != "req_001" {
		t.Errorf("got %q want req_001", got)
	}
}

func TestRequestID_NilContextReturnsEmpty(t *testing.T) {
	if got := GetRequestID(nil); got != "" {
		t.Errorf("nil ctx should return empty, got %q", got)
	}
}

func TestRequestMeta_GinAndContextStayInSync(t *testing.T) {
	c, _ := newCtx()
	meta := RequestMeta{
		RequestID: "req_meta_1",
		App:       "console",
		Debug:     true,
		Method:    "POST",
		Path:      "/x",
		Route:     "/x/:id",
		ClientIP:  "127.0.0.1",
		UserAgent: "ua",
	}
	SetRequestMeta(c, meta)

	if got := GetRequestMeta(c); got != meta {
		t.Errorf("GetRequestMeta: got %+v want %+v", got, meta)
	}
	if got := GetRequestMetaFromContext(c.Request.Context()); got != meta {
		t.Errorf("GetRequestMetaFromContext: got %+v want %+v", got, meta)
	}
	if got := GetRequestID(c); got != "req_meta_1" {
		t.Errorf("GetRequestID: got %q", got)
	}
}

func TestRequestMeta_DefaultEmpty(t *testing.T) {
	c, _ := newCtx()
	if got := GetRequestMeta(c); got.RequestID != "" || got.Debug {
		t.Errorf("expected zero meta, got %+v", got)
	}
}

func TestErrorMeta_Roundtrip(t *testing.T) {
	c, _ := newCtx()
	meta := ErrorMeta{
		HTTPStatus:    403,
		Code:          "forbidden",
		Message:       "无权限",
		InternalError: false,
		HasCause:      false,
	}
	SetErrorMeta(c, meta)
	if got := GetErrorMeta(c); got != meta {
		t.Errorf("got %+v want %+v", got, meta)
	}
}

func TestErrorMeta_NilContextReturnsZero(t *testing.T) {
	if got := GetErrorMeta(nil); got != (ErrorMeta{}) {
		t.Errorf("nil ctx should return zero, got %+v", got)
	}
}

func TestIdentity_GinAndContextStayInSync(t *testing.T) {
	c, _ := newCtx()
	ident := Identity{
		SubjectID:   "u_1",
		SubjectType: "console",
		UserID:      "u_1",
		AdminID:     "a_1",
		Username:    "alice",
		Email:       "alice@example.com",
		RoleID:      "r_1",
		IsSuper:     true,
	}
	SetIdentity(c, ident)

	if got := GetIdentity(c); got != ident {
		t.Errorf("GetIdentity: got %+v want %+v", got, ident)
	}
	if got := GetIdentityFromContext(c.Request.Context()); got != ident {
		t.Errorf("GetIdentityFromContext: got %+v want %+v", got, ident)
	}
}

func TestSetUserID_PopulatesSubjectFields(t *testing.T) {
	c, _ := newCtx()
	SetUserID(c, "u_42")

	ident := GetIdentity(c)
	if ident.SubjectID != "u_42" || ident.UserID != "u_42" {
		t.Errorf("subject/user id not set: %+v", ident)
	}
	if ident.SubjectType != "api" {
		t.Errorf("SubjectType: got %q want api", ident.SubjectType)
	}
	if ident.AdminID != "" {
		t.Errorf("AdminID should be empty for api identity, got %q", ident.AdminID)
	}
}

func TestIdentity_Helpers(t *testing.T) {
	c, _ := newCtx()

	if got := GetUserID(c); got != "" {
		t.Errorf("default UserID should be empty, got %q", got)
	}
	if got := GetAdminID(c); got != "" {
		t.Errorf("default AdminID should be empty, got %q", got)
	}
	if IsSuper(c) {
		t.Error("default IsSuper should be false")
	}

	SetIdentity(c, Identity{UserID: "u_x", AdminID: "a_x", IsSuper: true})
	if got := GetUserID(c); got != "u_x" {
		t.Errorf("UserID: got %q", got)
	}
	if got := GetAdminID(c); got != "a_x" {
		t.Errorf("AdminID: got %q", got)
	}
	if !IsSuper(c) {
		t.Error("IsSuper should be true after set")
	}
}

func TestIdentity_NilContextReturnsZero(t *testing.T) {
	if got := GetIdentity(nil); got != (Identity{}) {
		t.Errorf("nil ctx should return zero identity, got %+v", got)
	}
	if got := GetUserID(nil); got != "" {
		t.Errorf("nil ctx GetUserID should be empty, got %q", got)
	}
	if got := GetAdminID(nil); got != "" {
		t.Errorf("nil ctx GetAdminID should be empty, got %q", got)
	}
	if IsSuper(nil) {
		t.Error("nil ctx IsSuper should be false")
	}
}

func TestAuthToken_Roundtrip(t *testing.T) {
	c, _ := newCtx()
	SetAuthToken(c, "jwt_xyz")
	if got := GetAuthToken(c); got != "jwt_xyz" {
		t.Errorf("got %q want jwt_xyz", got)
	}
}

func TestAuthToken_NilContextReturnsEmpty(t *testing.T) {
	if got := GetAuthToken(nil); got != "" {
		t.Errorf("nil ctx should return empty, got %q", got)
	}
}

func TestAuditMeta_Roundtrip(t *testing.T) {
	c, _ := newCtx()
	meta := AuditMeta{
		TargetType: "role",
		TargetID:   "r_1",
		Detail:     map[string]any{"code": "admin"},
	}
	SetAuditMeta(c, meta)
	got := GetAuditMeta(c)
	if got.TargetType != "role" || got.TargetID != "r_1" {
		t.Errorf("AuditMeta fields not preserved: %+v", got)
	}
	if got.Detail["code"] != "admin" {
		t.Errorf("AuditMeta.Detail not preserved: %v", got.Detail)
	}
}

func TestAuditMeta_NilContextReturnsZero(t *testing.T) {
	got := GetAuditMeta(nil)
	if got.TargetType != "" || got.TargetID != "" || got.Detail != nil {
		t.Errorf("nil ctx should return zero, got %+v", got)
	}
}
