package permission

import (
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/zhimma/grove/pkg/route"
)

func TestCollectProtectedRoutesSkipsIgnoredAndTechnicalRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	route.ResetForTest()

	engine := gin.New()
	console := route.Wrap(engine.Group("/console/v1"))
	console.GET("/roles", func(c *gin.Context) {}).Name("角色权限.角色列表")
	console.OPTIONS("/roles", func(c *gin.Context) {})
	console.HEAD("/roles", func(c *gin.Context) {})
	console.GET("/health", func(c *gin.Context) {}).Ignore()
	engine.GET("/api/v1/public", func(c *gin.Context) {})

	routes := CollectProtectedRoutes(engine.Routes(), AppConsole, "console", "admin_auth")

	if len(routes) != 1 {
		t.Fatalf("expected exactly one catalog route, got %#v", routes)
	}
	if routes[0].Method != "GET" || routes[0].Path != "/console/v1/roles" {
		t.Fatalf("unexpected catalog route: %#v", routes[0])
	}
}
