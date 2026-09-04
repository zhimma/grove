package permission

import (
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/zhimma/grove/pkg/route"
)

func TestCollectProtectedRoutesSkipsIgnoredAndTechnicalRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	catalog := route.NewCatalog()
	console := route.Wrap(engine.Group("/console/v1"), catalog)
	console.GET("/roles", func(c *gin.Context) {}).Name("角色权限.角色列表")
	console.OPTIONS("/roles", func(c *gin.Context) {})
	console.HEAD("/roles", func(c *gin.Context) {})
	console.GET("/health", func(c *gin.Context) {}).Ignore()
	engine.GET("/api/v1/public", func(c *gin.Context) {})

	routes := CollectProtectedRoutes(engine.Routes(), AppConsole, "console", "admin_auth", catalog)

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
	group := route.Wrap(engine.Group("/console/v1"), catalog)
	group.GET("/roles", func(*gin.Context) {}).Name("角色.列表").Scope("tenant")

	items := CollectProtectedRoutes(engine.Routes(), AppConsole, "console", "admin_auth", catalog)
	if len(items) != 1 || items[0].Scope != "tenant" {
		t.Fatalf("unexpected catalog routes: %#v", items)
	}
	if got := BuildDisplayName(catalog, items[0].Method, items[0].Path); got != "角色.列表" {
		t.Fatalf("expected catalog display name, got %q", got)
	}
}

func TestBuildAPIIdentifierIsTheCanonicalPermissionString(t *testing.T) {
	cases := map[string]struct{ method, path, want string }{
		"upper cases the method":  {"get", "/console/v1/roles", "GET /console/v1/roles"},
		"trims surrounding space": {" post ", " /console/v1/roles ", "POST /console/v1/roles"},
		"keeps path parameters":   {"PUT", "/console/v1/roles/:id", "PUT /console/v1/roles/:id"},
	}
	for name, input := range cases {
		if got := BuildAPIIdentifier(input.method, input.path); got != input.want {
			t.Errorf("%s: BuildAPIIdentifier(%q, %q) = %q, want %q", name, input.method, input.path, got, input.want)
		}
	}
}

// The module code groups permissions in the console tree, so the framework's
// own path prefixes have to drop out and parameters must not become segments.
func TestBuildModuleCodeIgnoresPrefixesAndParameters(t *testing.T) {
	cases := map[string]string{
		"/console/v1/roles":                 "roles",
		"/api/v1/orders":                    "orders",
		"/merchant/v1/settlements":          "settlements",
		"/console/v1/scheduled-tasks":       "scheduled_tasks",
		"/console/v1/roles/:id/permissions": "roles",
		"/console/v1/files/*filepath":       "files",
		"/console/v1":                       "root",
		"/":                                 "root",
		"":                                  "root",
	}
	for path, want := range cases {
		if got := BuildModuleCode(path); got != want {
			t.Errorf("BuildModuleCode(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestBuildModuleCodeFromDisplayNameNeedsBothSegments(t *testing.T) {
	if got, ok := BuildModuleCodeFromDisplayName("角色权限.角色列表"); !ok || got != "角色权限" {
		t.Errorf("BuildModuleCodeFromDisplayName = %q, %v; want 角色权限, true", got, ok)
	}
	// A single segment carries no module, so callers must fall back rather
	// than treat the whole name as one.
	for _, input := range []string{"角色列表", "", ".", "  ", "角色权限."} {
		if got, ok := BuildModuleCodeFromDisplayName(input); ok {
			t.Errorf("BuildModuleCodeFromDisplayName(%q) = %q, true; want false", input, got)
		}
	}
}

func TestBuildDisplayNameFallsBackToMethodAndModule(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	catalog := route.NewCatalog()
	group := route.Wrap(engine.Group("/console/v1"), catalog)
	group.GET("/roles", func(*gin.Context) {}).Name("角色权限.角色列表")

	if got := BuildDisplayName(catalog, "GET", "/console/v1/roles"); got != "角色权限.角色列表" {
		t.Errorf("named route display name = %q", got)
	}
	// An unnamed route still needs something an operator can read.
	if got := BuildDisplayName(catalog, "post", "/console/v1/orders"); got != "POST orders" {
		t.Errorf("unnamed route display name = %q, want \"POST orders\"", got)
	}
	// A nil catalog must not panic; it just has no names.
	if got := BuildDisplayName(nil, "GET", "/console/v1/roles"); got != "GET roles" {
		t.Errorf("nil catalog display name = %q, want \"GET roles\"", got)
	}
}

func TestNormalizeScopeDefaultsToGlobal(t *testing.T) {
	for _, input := range []string{"", "   ", "global"} {
		if got := NormalizeScope(input); got != ScopeGlobal {
			t.Errorf("NormalizeScope(%q) = %q, want %q", input, got, ScopeGlobal)
		}
	}
	if got := NormalizeScope("  tenant  "); got != "tenant" {
		t.Errorf("NormalizeScope trimmed value = %q, want tenant", got)
	}
}
