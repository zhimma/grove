package route

import (
	"net/http"
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
