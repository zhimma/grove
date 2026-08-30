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

func TestCollectProtectedRoutesUsesEngineScopedCatalog(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	catalog := route.NewCatalog()
	group := route.WrapWithCatalog(engine.Group("/console/v1"), catalog)
	group.GET("/roles", func(*gin.Context) {}).Name("角色.列表").Scope("tenant")

	items := CollectProtectedRoutesWithCatalog(engine.Routes(), AppConsole, "console", "admin_auth", catalog)
	if len(items) != 1 || items[0].Scope != "tenant" {
		t.Fatalf("unexpected catalog routes: %#v", items)
	}
	if got := BuildDisplayNameWithCatalog(catalog, items[0].Method, items[0].Path); got != "角色.列表" {
		t.Fatalf("expected catalog display name, got %q", got)
	}
}
