package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/zhimma/grove/internal/model"
	"github.com/zhimma/grove/internal/testkit"
	"github.com/zhimma/grove/pkg/errx"
	"github.com/zhimma/grove/pkg/request"
	"github.com/zhimma/grove/pkg/response"
)

func TestAuditOperationUsesErrorMetaMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := testkit.OpenDB(t, &model.ConsoleOperationLog{})

	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		request.SetRequestID(c, "req-audit")
		request.SetRequestMeta(c, request.RequestMeta{
			RequestID: "req-audit",
			Method:    c.Request.Method,
			Path:      c.Request.URL.Path,
			Route:     "/console/v1/admins/:id",
			ClientIP:  c.ClientIP(),
			UserAgent: c.Request.UserAgent(),
		})
		c.Next()
	})
	engine.Use(AuditOperation(db))
	engine.DELETE("/console/v1/admins/:id", func(c *gin.Context) {
		response.Fail(c, errx.Forbidden().WithMessage("不能删除超级管理员").WithCode("super_admin_forbidden"))
	})

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/console/v1/admins/1", nil)
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", recorder.Code)
	}

	var log model.ConsoleOperationLog
	if err := db.First(&log).Error; err != nil {
		t.Fatalf("read operation log: %v", err)
	}
	if log.ErrorMessage != "不能删除超级管理员" {
		t.Fatalf("expected business error message, got %q", log.ErrorMessage)
	}
}

func TestAuditRequestQueryRedactsSensitiveValuesAndBoundsLength(t *testing.T) {
	query := "page=2&keyword=alice&password=plain-password&Authorization=bearer-secret&token=one-time-token"
	got := auditRequestQuery("/console/v1/admins/:id", query)

	for _, secret := range []string{"plain-password", "bearer-secret", "one-time-token"} {
		if strings.Contains(got, secret) {
			t.Fatalf("audit query leaked %q: %s", secret, got)
		}
	}
	values, err := url.ParseQuery(got)
	if err != nil {
		t.Fatalf("redacted query must remain parseable: %v (%s)", err, got)
	}
	if values.Get("password") != auditRedactedValue || values.Get("Authorization") != auditRedactedValue || values.Get("token") != auditRedactedValue {
		t.Fatalf("sensitive query values were not redacted: %#v", values)
	}
	if values.Get("keyword") != "alice" || values.Get("page") != "2" {
		t.Fatalf("non-sensitive query values should remain useful: %#v", values)
	}

	longQuery := "keyword=" + strings.Repeat("x", maxAuditQueryLength*2) + "&password=still-secret"
	got = auditRequestQuery("/console/v1/admins", longQuery)
	if len(got) > maxAuditQueryLength {
		t.Fatalf("audit query exceeded limit: got %d, want <= %d", len(got), maxAuditQueryLength)
	}
	if strings.Contains(got, "still-secret") {
		t.Fatal("long audit query leaked a sensitive value")
	}
}

func TestAuditOperationRedactsNestedDetailValues(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testkit.OpenDB(t, &model.ConsoleOperationLog{})

	engine := gin.New()
	engine.Use(AuditOperation(db))
	engine.POST("/console/v1/admins/:id", func(c *gin.Context) {
		request.SetAuditMeta(c, request.AuditMeta{
			TargetType: "console_admin",
			TargetID:   "admin-1",
			Detail: map[string]any{
				"headers": map[string]any{
					"Authorization": "header-secret",
				},
				"body": map[string]any{
					"password":     "body-secret",
					"display_name": "Alice",
				},
				"long_value": strings.Repeat("z", maxAuditDetailValueLength*2),
			},
		})
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/console/v1/admins/admin-1", nil)
	engine.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", recorder.Code)
	}

	var log model.ConsoleOperationLog
	if err := db.First(&log).Error; err != nil {
		t.Fatalf("read operation log: %v", err)
	}
	for _, secret := range []string{"header-secret", "body-secret"} {
		if strings.Contains(log.DetailJSON, secret) {
			t.Fatalf("detail leaked %q: %s", secret, log.DetailJSON)
		}
	}
	if !strings.Contains(log.DetailJSON, `"Authorization":"`+auditRedactedValue+`"`) || !strings.Contains(log.DetailJSON, `"password":"`+auditRedactedValue+`"`) {
		t.Fatalf("detail redaction marker missing: %s", log.DetailJSON)
	}
	if len(log.DetailJSON) > maxAuditDetailLength {
		t.Fatalf("detail exceeded limit: got %d, want <= %d", len(log.DetailJSON), maxAuditDetailLength)
	}
	var detail map[string]any
	if err := json.Unmarshal([]byte(log.DetailJSON), &detail); err != nil {
		t.Fatalf("detail JSON must remain valid: %v (%s)", err, log.DetailJSON)
	}
}

func TestAuditOperationUsesValidTruncationMarkerForLargeDetail(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testkit.OpenDB(t, &model.ConsoleOperationLog{})

	engine := gin.New()
	engine.Use(AuditOperation(db))
	engine.POST("/console/v1/admins/:id", func(c *gin.Context) {
		detail := make(map[string]any, 8)
		for i := 0; i < 8; i++ {
			detail["field_"+strconv.Itoa(i)] = strings.Repeat("z", maxAuditDetailValueLength)
		}
		request.SetAuditMeta(c, request.AuditMeta{Detail: detail})
		c.Status(http.StatusNoContent)
	})

	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, httptest.NewRequest(http.MethodPost, "/console/v1/admins/admin-1", nil))
	if resp.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", resp.Code)
	}

	var log model.ConsoleOperationLog
	if err := db.First(&log).Error; err != nil {
		t.Fatalf("read operation log: %v", err)
	}
	if len(log.DetailJSON) > maxAuditDetailLength {
		t.Fatalf("detail exceeded limit: got %d, want <= %d", len(log.DetailJSON), maxAuditDetailLength)
	}
	var detail map[string]any
	if err := json.Unmarshal([]byte(log.DetailJSON), &detail); err != nil {
		t.Fatalf("detail JSON must remain valid: %v (%s)", err, log.DetailJSON)
	}
	if detail["_truncated"] != true {
		t.Fatalf("expected truncation marker, got %#v", detail)
	}
}

func TestAuditRequestQueryOmitsSystemConfigValues(t *testing.T) {
	if got := auditRequestQuery("/console/v1/system-configs/:id", "key=name&value=secret"); got != "" {
		t.Fatalf("system config query must not be retained, got %q", got)
	}
}
