package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/zhimma/grove/pkg/auth"
	"github.com/zhimma/grove/pkg/request"
	"github.com/zhimma/grove/pkg/route"
)

func TestAdminAuthRequiresBearerSchemeAndSessionService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tokens, err := auth.NewTokens(auth.Config{Secret: "test-secret", Issuer: "test-issuer", AccessExpiry: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	pair, err := tokens.GenerateAdminTokenPair("admin-1", "session-1", auth.UserTypeConsole)
	if err != nil {
		t.Fatal(err)
	}
	engine := gin.New()
	engine.GET("/private", AdminAuthn(tokens, nil, nil), func(c *gin.Context) { t.Error("authentication must reject the request") })
	for _, tc := range []struct {
		header string
		status int
	}{
		{"bEaReR " + pair.AccessToken, http.StatusServiceUnavailable},
		{pair.AccessToken, http.StatusUnauthorized},
		{"Basic " + pair.AccessToken, http.StatusUnauthorized},
	} {
		req := httptest.NewRequest(http.MethodGet, "/private", nil)
		req.Header.Set("Authorization", tc.header)
		resp := httptest.NewRecorder()
		engine.ServeHTTP(resp, req)
		if resp.Code != tc.status {
			t.Fatalf("expected %d, got %d", tc.status, resp.Code)
		}
	}
}

func TestAdminPermissionFailsClosedWithoutEnforcer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handlerCalled := false
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		request.SetIdentity(c, request.Identity{AdminID: "admin-1"})
		c.Next()
	})
	engine.Use(AdminPermission(nil, route.NewCatalog()))
	engine.GET("/console/v1/roles", func(c *gin.Context) {
		handlerCalled = true
		c.Status(http.StatusNoContent)
	})

	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/console/v1/roles", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", response.Code, response.Body.String())
	}
	if handlerCalled {
		t.Fatal("protected handler must not run without an enforcer")
	}
}
