package route

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestCatalogIsolatesRouteMetadata(t *testing.T) {
	gin.SetMode(gin.TestMode)
	first := NewCatalog()
	second := NewCatalog()

	engine := gin.New()
	a := Wrap(engine.Group("/one"), first)
	b := Wrap(engine.Group("/two"), second)
	a.GET("/items", func(*gin.Context) {}).Name("first").Scope("tenant-a")
	b.GET("/items", func(*gin.Context) {}).Name("second").Scope("tenant-b").Ignore()

	if name, ok := first.GetName(http.MethodGet, "/one/items"); !ok || name != "first" {
		t.Fatalf("first catalog name = %q, %v", name, ok)
	}
	if name, ok := second.GetName(http.MethodGet, "/two/items"); !ok || name != "second" {
		t.Fatalf("second catalog name = %q, %v", name, ok)
	}
	if first.IsIgnored(http.MethodGet, "/one/items") || !second.IsIgnored(http.MethodGet, "/two/items") {
		t.Fatal("route ignore metadata crossed catalog boundary")
	}
}

// Nested groups must inherit the scope and the catalog, and the recorded path
// must be the full one gin will match. The permission catalog is built from
// these keys, so a wrong path silently produces a permission nobody holds.
func TestNestedGroupsInheritScopeAndRecordFullPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	catalog := NewCatalog()

	v1 := Wrap(engine.Group("/console/v1"), catalog).Scope("tenant")
	orders := v1.Group("/orders")
	orders.GET("", func(*gin.Context) {}).Name("订单.列表")
	orders.POST("/:id/refund", func(*gin.Context) {}).Name("订单.退款")

	for _, expected := range []struct {
		method, path, name string
	}{
		{"GET", "/console/v1/orders", "订单.列表"},
		{"POST", "/console/v1/orders/:id/refund", "订单.退款"},
	} {
		name, ok := catalog.GetName(expected.method, expected.path)
		if !ok || name != expected.name {
			t.Errorf("GetName(%s %s) = %q, %v; want %q", expected.method, expected.path, name, ok, expected.name)
		}
		// Scope set on the parent has to reach a route registered on a child.
		scope, ok := catalog.GetScope(expected.method, expected.path)
		if !ok || scope != "tenant" {
			t.Errorf("GetScope(%s %s) = %q, %v; want tenant", expected.method, expected.path, scope, ok)
		}
	}
}

// A child overriding the scope must not rewrite its parent's.
func TestChildScopeDoesNotLeakToTheParent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	catalog := NewCatalog()

	root := Wrap(engine.Group("/console/v1"), catalog).Scope("tenant")
	root.Group("/internal").Scope("global").GET("/health", func(*gin.Context) {}).Name("内部.健康")
	root.GET("/orders", func(*gin.Context) {}).Name("订单.列表")

	if scope, _ := catalog.GetScope("GET", "/console/v1/internal/health"); scope != "global" {
		t.Errorf("child scope = %q, want global", scope)
	}
	if scope, _ := catalog.GetScope("GET", "/console/v1/orders"); scope != "tenant" {
		t.Errorf("parent scope = %q, want tenant", scope)
	}
}

func TestEveryVerbRegistersWithGinAndTheCatalog(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	catalog := NewCatalog()
	group := Wrap(engine.Group("/api"), catalog)

	handler := func(*gin.Context) {}
	group.GET("/resource", handler).Name("资源.读")
	group.POST("/resource", handler).Name("资源.建")
	group.PUT("/resource", handler).Name("资源.替换")
	group.PATCH("/resource", handler).Name("资源.改")
	group.DELETE("/resource", handler).Name("资源.删")
	group.HEAD("/resource", handler).Name("资源.头")
	group.OPTIONS("/resource", handler).Name("资源.选项")

	registered := map[string]bool{}
	for _, route := range engine.Routes() {
		registered[route.Method+" "+route.Path] = true
	}
	for _, method := range []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"} {
		key := method + " /api/resource"
		if !registered[key] {
			t.Errorf("%s was not registered with gin", key)
		}
		if _, ok := catalog.GetName(method, "/api/resource"); !ok {
			t.Errorf("%s has no catalog name", key)
		}
	}
}

// Use must apply middleware without disturbing the catalog wiring.
func TestUseKeepsTheGroupChainable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	catalog := NewCatalog()

	called := false
	group := Wrap(engine.Group("/api"), catalog).Use(func(c *gin.Context) {
		called = true
		c.Next()
	})
	group.GET("/ping", func(c *gin.Context) { c.Status(http.StatusNoContent) }).Name("探活.ping")

	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/ping", nil))

	if !called {
		t.Fatal("middleware registered through Use did not run")
	}
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", response.Code)
	}
	if _, ok := catalog.GetName("GET", "/api/ping"); !ok {
		t.Fatal("Use dropped the catalog")
	}
}

// Wrap without a catalog must still work: metadata lands in a private one
// rather than a process-wide store, which is what the old globals did.
func TestWrapWithoutACatalogKeepsMetadataPrivate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	shared := NewCatalog()

	Wrap(engine.Group("/api"), nil).GET("/orphan", func(*gin.Context) {}).Name("孤儿.路由")

	if _, ok := shared.GetName("GET", "/api/orphan"); ok {
		t.Fatal("metadata leaked into an unrelated catalog")
	}
}

func TestRouteKeyNormalisesMethodAndPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	catalog := NewCatalog()
	Wrap(engine.Group("/api/"), catalog).GET("/orders/", func(*gin.Context) {}).Name("订单.列表")

	// Lookups use the cleaned path and an upper-cased method regardless of how
	// the caller spells them.
	for _, lookup := range []struct{ method, path string }{
		{"GET", "/api/orders"},
		{"get", "/api/orders"},
		{"GET", "/api/orders/"},
		{"GET", "//api//orders"},
	} {
		if _, ok := catalog.GetName(lookup.method, lookup.path); !ok {
			t.Errorf("GetName(%q, %q) missed the route", lookup.method, lookup.path)
		}
	}
}

func TestNilCatalogAndNilRouteAreSafe(t *testing.T) {
	var catalog *Catalog
	if name, ok := catalog.GetName("GET", "/x"); ok || name != "" {
		t.Error("nil catalog must read empty")
	}
	if scope, ok := catalog.GetScope("GET", "/x"); ok || scope != "" {
		t.Error("nil catalog must read empty scope")
	}
	if catalog.IsIgnored("GET", "/x") {
		t.Error("nil catalog must not report ignored")
	}
	catalog.Name("GET", "/x", "n")
	catalog.Scope("GET", "/x", "s")
	catalog.Ignore("GET", "/x")

	var route *Route
	if route.Name("n") != nil || route.Scope("s") != nil || route.Ignore() != nil {
		t.Error("nil route chain must stay nil")
	}
}
