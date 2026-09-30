package request

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func newTestContext(t *testing.T) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/orders", nil)
	return c
}

// Services receive a context.Context, not a *gin.Context, so request meta set
// by middleware has to reach the std context too. Without that bridge a service
// logs an empty request ID and client IP with no other symptom.
func TestSetRequestMetaReachesTheStdContext(t *testing.T) {
	c := newTestContext(t)
	meta := RequestMeta{
		RequestID: "req-1",
		App:       "console",
		Method:    http.MethodGet,
		Path:      "/orders",
		ClientIP:  "203.0.113.7",
	}

	SetRequestMeta(c, meta)

	if got := RequestMetaOf(c); got != meta {
		t.Fatalf("gin-side meta = %+v, want %+v", got, meta)
	}
	if got := RequestMetaFromContext(c.Request.Context()); got != meta {
		t.Fatalf("std-context meta = %+v, want %+v", got, meta)
	}
	// SetRequestMeta also publishes the ID on its own key.
	if got := RequestID(c); got != "req-1" {
		t.Fatalf("request id = %q, want req-1", got)
	}
}

// gin.CreateTestContext leaves Request nil, and middleware tests hit that.
// A setter that dereferences it takes down the request instead of degrading.
func TestSettersToleratePartiallyBuiltContexts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	bare, _ := gin.CreateTestContext(httptest.NewRecorder())

	SetRequestMeta(bare, RequestMeta{RequestID: "req-2"})
	SetIdentity(bare, Identity{AdminID: "admin-1"})
	SetRequestID(bare, "req-3")
	SetAuthToken(bare, "token")
	SetErrorMeta(bare, ErrorMeta{HTTPStatus: 500})
	SetAuditMeta(bare, AuditMeta{TargetType: "order"})

	if got := IdentityOf(bare).AdminID; got != "admin-1" {
		t.Fatalf("admin id = %q, want admin-1 even without a Request", got)
	}
	if got := RequestID(bare); got != "req-3" {
		t.Fatalf("request id = %q, want req-3", got)
	}
	if got := AuthToken(bare); got != "token" {
		t.Fatalf("auth token = %q, want token", got)
	}
}

func TestSettersAndGettersIgnoreANilContext(t *testing.T) {
	SetRequestMeta(nil, RequestMeta{RequestID: "req"})
	SetIdentity(nil, Identity{AdminID: "admin"})
	SetRequestID(nil, "req")
	SetAuthToken(nil, "token")
	SetErrorMeta(nil, ErrorMeta{})
	SetAuditMeta(nil, AuditMeta{})

	if RequestID(nil) != "" || AuthToken(nil) != "" {
		t.Fatal("nil context must read as empty")
	}
	if RequestMetaOf(nil) != (RequestMeta{}) || ErrorMetaOf(nil) != (ErrorMeta{}) {
		t.Fatal("nil context must read as zero meta")
	}
	if IdentityOf(nil) != (Identity{}) {
		t.Fatal("nil context must read as zero identity")
	}
	if AuditMetaOf(nil).TargetType != "" {
		t.Fatal("nil context must read as zero audit meta")
	}
}

// Every getter type-asserts its key. Anything else writing that key must read
// back as zero rather than panic, because the keys are plain strings shared
// with whatever else touches the gin context.
func TestGettersReadZeroWhenAKeyHoldsTheWrongType(t *testing.T) {
	c := newTestContext(t)
	for _, key := range []string{
		RequestIDKey, RequestMetaKey, ErrorMetaKey, IdentityKey, AuthTokenKey, AuditMetaKey,
	} {
		c.Set(key, 12345)
	}

	if RequestID(c) != "" || AuthToken(c) != "" {
		t.Fatal("string getters must ignore a non-string value")
	}
	if RequestMetaOf(c) != (RequestMeta{}) || ErrorMetaOf(c) != (ErrorMeta{}) {
		t.Fatal("meta getters must ignore a mistyped value")
	}
	if IdentityOf(c) != (Identity{}) {
		t.Fatal("identity getter must ignore a mistyped value")
	}
	if AuditMetaOf(c).Detail != nil {
		t.Fatal("audit getter must ignore a mistyped value")
	}
}

func TestIdentityAccessorsReadTheStoredIdentity(t *testing.T) {
	c := newTestContext(t)
	SetIdentity(c, Identity{
		AdminID:   "admin-1",
		UserID:    "user-1",
		SessionID: "session-1",
		IsSuper:   true,
	})

	if got := AdminID(c); got != "admin-1" {
		t.Errorf("admin id = %q", got)
	}
	if got := UserID(c); got != "user-1" {
		t.Errorf("user id = %q", got)
	}
	if got := SessionID(c); got != "session-1" {
		t.Errorf("session id = %q", got)
	}
	if !IsSuper(c) {
		t.Error("IsSuper = false, want true")
	}
}

// SetUserID is the API surface's entry point and has to fill the subject
// fields, since authorization reads SubjectType to tell an api caller from a
// console one.
func TestSetUserIDMarksAnAPISubject(t *testing.T) {
	c := newTestContext(t)
	SetUserID(c, "user-9")

	identity := IdentityOf(c)
	if identity.UserID != "user-9" || identity.SubjectID != "user-9" {
		t.Fatalf("identity = %+v, want the user id in both fields", identity)
	}
	if identity.SubjectType != "api" {
		t.Fatalf("subject type = %q, want api", identity.SubjectType)
	}
	if identity.AdminID != "" || identity.IsSuper {
		t.Fatalf("api subject must not carry admin fields: %+v", identity)
	}
}

func TestRequestMetaFromABareContextIsZero(t *testing.T) {
	if got := RequestMetaFromContext(context.Background()); got != (RequestMeta{}) {
		t.Fatalf("meta = %+v, want zero", got)
	}
}

func TestErrorMetaRoundtrip(t *testing.T) {
	c := newTestContext(t)
	meta := ErrorMeta{HTTPStatus: 403, Code: "forbidden", Message: "无权限"}
	SetErrorMeta(c, meta)
	if got := ErrorMetaOf(c); got != meta {
		t.Fatalf("error meta = %+v, want %+v", got, meta)
	}
}

func TestAuditMetaRoundtrip(t *testing.T) {
	c := newTestContext(t)
	SetAuditMeta(c, AuditMeta{TargetType: "role", TargetID: "r_1", Detail: map[string]any{"code": "admin"}})
	got := AuditMetaOf(c)
	if got.TargetType != "role" || got.TargetID != "r_1" || got.Detail["code"] != "admin" {
		t.Fatalf("audit fields were not preserved: %+v", got)
	}
}
