package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/zhimma/grove/pkg/request"
)

func TestAdminPermissionFailsClosedWithoutEnforcerInEveryEnvironment(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, environment := range []string{"", "development", "production"} {
		t.Run(environment, func(t *testing.T) {
			handlerCalled := false
			engine := gin.New()
			engine.Use(func(c *gin.Context) {
				request.SetIdentity(c, request.Identity{AdminID: "admin-1"})
				c.Next()
			})
			engine.Use(AdminPermission(nil, environment))
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
		})
	}
}
