package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/zhimma/grove/internal/config"
	appmiddleware "github.com/zhimma/grove/internal/middleware"
	"github.com/zhimma/grove/internal/model"
	"github.com/zhimma/grove/internal/provider"
	"github.com/zhimma/grove/internal/testkit"
	"github.com/zhimma/grove/pkg/database"
	"github.com/zhimma/grove/pkg/rbac"
)

func TestRouterDoesNotRegisterDemoRoutesInProduction(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := newRouterTestConfig(t, "production")
	cfg.Demo.Enabled = true
	engine, _ := newRouterTestEngine(t, cfg)

	for _, request := range []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/api/v1/ping"},
		{method: http.MethodPost, path: "/api/v1/auth/access-token"},
		{method: http.MethodGet, path: "/api/v1/profile"},
		{method: http.MethodPost, path: "/api/v1/jobs/echo"},
	} {
		resp := httptest.NewRecorder()
		req := httptest.NewRequest(request.method, request.path, nil)
		engine.ServeHTTP(resp, req)
		if resp.Code != http.StatusNotFound {
			t.Fatalf("%s %s expected 404, got %d body=%s", request.method, request.path, resp.Code, resp.Body.String())
		}
	}
}

func TestRouterPingAndProfile(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := newRouterTestConfig(t, "test")
	cfg.Demo.Enabled = true

	p, err := provider.New(cfg, "api", provider.WithAuth())
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	t.Cleanup(func() { _ = p.Close() })
	db := testkit.OpenDB(t, &model.User{})
	if err := db.Create(&model.User{
		Base:   model.Base{ID: "api-user"},
		Name:   "API User",
		Email:  "api-user@example.test",
		Status: model.UserStatusActive,
	}).Error; err != nil {
		t.Fatalf("create API user: %v", err)
	}
	p.DB = database.NewConnectionsFromDBs(db, nil)
	attachAPIEnforcer(t, p, db, "GET /api/v1/profile")

	engine := gin.New()
	engine.Use(appmiddleware.RequestID(), appmiddleware.RequestMeta("api"), appmiddleware.Recovery())
	New(cfg, p).InstallToEngine(engine)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ping?name=codex", nil)
	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
	if !strings.Contains(resp.Body.String(), "pong, codex") {
		t.Fatalf("unexpected ping response: %s", resp.Body.String())
	}

	token, err := p.Tokens.IssueAccessToken("api-user")
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/profile", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp = httptest.NewRecorder()
	engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 for profile, got %d body=%s", resp.Code, resp.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode profile response: %v", err)
	}
	if int(payload["code"].(float64)) != 0 {
		t.Fatalf("expected success response, got %v", payload)
	}
}

func TestRouterReturnsFieldErrorsForInvalidRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := newRouterTestConfig(t, "test")
	cfg.Demo.Enabled = true
	engine, p := newRouterTestEngine(t, cfg, "POST /api/v1/jobs/echo")

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/access-token", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(resp, req)
	assertFieldError(t, resp, http.StatusUnprocessableEntity, "user_id", "用户ID不能为空")

	token, err := p.Tokens.IssueAccessToken("api-user")
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	resp = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/jobs/echo", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	engine.ServeHTTP(resp, req)
	assertFieldError(t, resp, http.StatusUnprocessableEntity, "message", "消息内容不能为空")
}

func assertFieldError(t *testing.T, resp *httptest.ResponseRecorder, status int, field, message string) {
	t.Helper()
	if resp.Code != status {
		t.Fatalf("expected status %d, got %d body=%s", status, resp.Code, resp.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v body=%s", err, resp.Body.String())
	}
	data, _ := payload["data"].(map[string]any)
	errorsPayload, _ := data["errors"].(map[string]any)
	values, _ := errorsPayload[field].([]any)
	if len(values) != 1 || values[0] != message {
		t.Fatalf("expected %s error %q, got %#v", field, message, payload)
	}
}

func newRouterTestConfig(t *testing.T, env string) *config.Config {
	t.Helper()
	return &config.Config{
		App:  config.AppConfig{Name: "grove", Env: env},
		Port: "8080",
		Log: config.LogConfig{
			Level:   "error",
			Path:    t.TempDir(),
			Console: false,
			Service: "api-test",
		},
		JWT: config.JWTConfig{
			Secret:            "0123456789abcdef0123456789abcdef",
			Issuer:            "grove",
			AccessExpiryHours: 24,
		},
		API: config.APIConfig{
			Prefix: "/api/v1",
		},
	}
}

func TestRouterProtectedRoutesFailClosedWithoutAPIEnforcer(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := newRouterTestConfig(t, "test")
	cfg.Demo.Enabled = true
	engine, p := newRouterTestEngine(t, cfg)

	token, err := p.Tokens.IssueAccessToken("api-user")
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/profile", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, req)
	if resp.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 without API enforcer, got %d body=%s", resp.Code, resp.Body.String())
	}
}

func newRouterTestEngine(t *testing.T, cfg *config.Config, permissions ...string) (*gin.Engine, *provider.Provider) {
	t.Helper()
	p, err := provider.New(cfg, "api", provider.WithAuth())
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	t.Cleanup(func() { _ = p.Close() })
	if len(permissions) > 0 {
		db := testkit.OpenDB(t)
		attachAPIEnforcer(t, p, db, permissions...)
	}

	engine := gin.New()
	engine.Use(appmiddleware.RequestID(), appmiddleware.RequestMeta("api"), appmiddleware.Recovery())
	New(cfg, p).InstallToEngine(engine)
	return engine, p
}

func attachAPIEnforcer(t *testing.T, p *provider.Provider, db *gorm.DB, permissions ...string) {
	t.Helper()
	testkit.CreateCasbinTable(t, db, "casbin_rules")
	enforcer, err := rbac.New(db, &rbac.Config{TableName: "casbin_rules"})
	if err != nil {
		t.Fatalf("new API enforcer: %v", err)
	}
	if _, err := enforcer.AddGroupingPolicy("api-user", "api-role"); err != nil {
		t.Fatalf("add API role: %v", err)
	}
	for _, permission := range permissions {
		if _, err := enforcer.AddPolicy("api-role", permission); err != nil {
			t.Fatalf("add API policy %q: %v", permission, err)
		}
	}
	if p.Enforcers == nil {
		p.Enforcers = make(map[string]*rbac.Enforcer)
	}
	p.Enforcers["api"] = enforcer
}
