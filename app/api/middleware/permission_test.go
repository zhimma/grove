package middleware

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/zhimma/grove/pkg/casbin"
	"github.com/zhimma/grove/pkg/reqctx"
)

func TestPermissionSetRequiresEnforcer(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	engine.GET("/ping", NewPermissionSet(nil, nil).Require("api.ping"), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	resp := performPermissionRequest(t, engine, "")
	if resp.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", resp.Code)
	}
	assertPermissionMessage(t, resp, "权限控制器未配置")
}

func TestPermissionSetRequiresUserIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enforcer := openPermissionTestEnforcer(t, casbin.ModeRBAC)
	engine := gin.New()
	engine.GET("/ping", NewPermissionSet(enforcer, nil).Require("api.ping"), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	resp := performPermissionRequest(t, engine, "")
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.Code)
	}
	assertPermissionMessage(t, resp, "缺少用户身份信息")
}

func TestPermissionSetRejectsWithoutPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enforcer := openPermissionTestEnforcer(t, casbin.ModeRBAC)
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		reqctx.SetUserID(c, "user-1")
		c.Next()
	})
	engine.GET("/ping", NewPermissionSet(enforcer, nil).Require("api.ping"), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	resp := performPermissionRequest(t, engine, "")
	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.Code)
	}
	assertPermissionMessage(t, resp, "无权限访问: api.ping")
}

func TestPermissionSetAllowsWhenPermissionGranted(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enforcer := openPermissionTestEnforcer(t, casbin.ModeRBAC)
	if _, err := enforcer.AddPolicy("member", "api.ping"); err != nil {
		t.Fatalf("add policy: %v", err)
	}
	if _, err := enforcer.AddGroupingPolicy("user-1", "member"); err != nil {
		t.Fatalf("add grouping policy: %v", err)
	}

	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		reqctx.SetUserID(c, "user-1")
		c.Next()
	})
	engine.GET("/ping", NewPermissionSet(enforcer, nil).Require("api.ping"), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	resp := performPermissionRequest(t, engine, "")
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", resp.Code, resp.Body.String())
	}
}

func TestPermissionSetRequiresDomainResolverInDomainMode(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enforcer := openPermissionTestEnforcer(t, casbin.ModeRBACDomains)
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		reqctx.SetUserID(c, "user-1")
		c.Next()
	})
	engine.GET("/ping", NewPermissionSet(enforcer, nil).Require("api.ping"), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	resp := performPermissionRequest(t, engine, "")
	if resp.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", resp.Code)
	}
	assertPermissionMessage(t, resp, "权限域解析器未配置")
}

func TestPermissionSetReturnsResolverError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enforcer := openPermissionTestEnforcer(t, casbin.ModeRBACDomains)
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		reqctx.SetUserID(c, "user-1")
		c.Next()
	})
	engine.GET("/ping", NewPermissionSet(enforcer, func(*gin.Context) (string, error) {
		return "", errors.New("租户标识不能为空")
	}).Require("api.ping"), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	resp := performPermissionRequest(t, engine, "")
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.Code)
	}
	assertPermissionMessage(t, resp, "租户标识不能为空")
}

func openPermissionTestEnforcer(t *testing.T, mode casbin.Mode) *casbin.Enforcer {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/permission.db"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec(`
CREATE TABLE IF NOT EXISTS casbin_rules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    ptype TEXT,
    v0 TEXT,
    v1 TEXT,
    v2 TEXT,
    v3 TEXT,
    v4 TEXT,
    v5 TEXT
);`).Error; err != nil {
		t.Fatalf("create casbin table: %v", err)
	}

	enforcer, err := casbin.New(db, &casbin.Config{Mode: mode, TableName: "casbin_rules"})
	if err != nil {
		t.Fatalf("new enforcer: %v", err)
	}
	return enforcer
}

func performPermissionRequest(t *testing.T, engine *gin.Engine, token string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	if token != "" {
		req.Header.Set("Authorization", token)
	}
	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, req)
	return resp
}

func assertPermissionMessage(t *testing.T, resp *httptest.ResponseRecorder, expected string) {
	t.Helper()

	var payload map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["message"] != expected {
		t.Fatalf("expected message %q, got %#v", expected, payload["message"])
	}
}
