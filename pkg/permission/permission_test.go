package permission

import (
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/zhimma/grove/pkg/route"
)

func TestBuildConsoleMenuTree(t *testing.T) {
	tree := BuildConsoleMenuTree()

	if len(tree) == 0 {
		t.Fatal("expected static console menu tree")
	}
	if tree[0].MenuKey != "ConsoleDashboard" {
		t.Fatalf("expected ConsoleDashboard first, got %s", tree[0].MenuKey)
	}
	if len(tree) < 3 || tree[2].MenuKey != "ConsoleSystem" {
		t.Fatalf("expected ConsoleSystem root node, got %#v", tree)
	}
	if len(tree[2].Children) != 2 {
		t.Fatalf("expected ConsoleSystem to have 2 children, got %#v", tree[2].Children)
	}
	if tree[2].Children[0].MenuKey != "ConsoleAdmins" || tree[2].Children[1].MenuKey != "ConsoleRoles" {
		t.Fatalf("unexpected ConsoleSystem children: %#v", tree[2].Children)
	}
}

func TestFilterConsoleMenuKeys(t *testing.T) {
	filtered := FilterConsoleMenuKeys([]string{
		"ConsoleRoles",
		"invalid",
		"ConsoleDashboard",
		"ConsoleRoles",
	})

	if len(filtered) != 2 {
		t.Fatalf("expected 2 filtered keys, got %d", len(filtered))
	}
	if filtered[0] != "ConsoleDashboard" || filtered[1] != "ConsoleRoles" {
		t.Fatalf("unexpected filtered keys: %#v", filtered)
	}
}

func TestValidateConsoleMenuKeys(t *testing.T) {
	if err := ValidateConsoleMenuKeys([]string{"ConsoleDashboard", "ConsoleRoles"}); err != nil {
		t.Fatalf("expected valid keys, got %v", err)
	}
	if err := ValidateConsoleMenuKeys([]string{"ConsoleDashboard", "unknown"}); err == nil {
		t.Fatal("expected invalid menu key error")
	}
}

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
