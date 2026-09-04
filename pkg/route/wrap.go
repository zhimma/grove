package route

import (
	"path"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

type Route struct {
	method  string
	path    string
	catalog *Catalog
}

// Catalog owns route metadata for one HTTP engine. It prevents API and
// Console registrations (and parallel tests) from overwriting each other.
type Catalog struct {
	nameStore  sync.Map
	scopeStore sync.Map
	ignored    sync.Map
}

func NewCatalog() *Catalog { return &Catalog{} }

func (c *Catalog) Name(method, routePath, displayName string) {
	if c != nil {
		c.nameStore.Store(routeKey(method, routePath), strings.TrimSpace(displayName))
	}
}

func (c *Catalog) Scope(method, routePath, scope string) {
	if c != nil {
		c.scopeStore.Store(routeKey(method, routePath), strings.TrimSpace(scope))
	}
}

func (c *Catalog) Ignore(method, routePath string) {
	if c != nil {
		c.ignored.Store(routeKey(method, routePath), true)
	}
}

func (c *Catalog) GetName(method, routePath string) (string, bool) {
	if c == nil {
		return "", false
	}
	value, ok := c.nameStore.Load(routeKey(method, routePath))
	name, valid := value.(string)
	return name, ok && valid && name != ""
}

func (c *Catalog) GetScope(method, routePath string) (string, bool) {
	if c == nil {
		return "", false
	}
	value, ok := c.scopeStore.Load(routeKey(method, routePath))
	scope, valid := value.(string)
	return scope, ok && valid && scope != ""
}

func (c *Catalog) IsIgnored(method, routePath string) bool {
	if c == nil {
		return false
	}
	value, ok := c.ignored.Load(routeKey(method, routePath))
	ignored, valid := value.(bool)
	return ok && valid && ignored
}

func (r *Route) Name(displayName string) *Route {
	if r == nil {
		return r
	}
	r.catalog.Name(r.method, r.path, displayName)
	return r
}

func (r *Route) Scope(scope string) *Route {
	if r == nil {
		return r
	}
	r.catalog.Scope(r.method, r.path, scope)
	return r
}

func (r *Route) Ignore() *Route {
	if r == nil {
		return r
	}
	r.catalog.Ignore(r.method, r.path)
	return r
}

type Group struct {
	group        *gin.RouterGroup
	defaultScope string
	catalog      *Catalog
}

// Wrap binds a Gin group to the catalog that owns its route metadata.
// The catalog is required: metadata with no owner used to land in package
// globals, which leaked between engines and between parallel tests.
func Wrap(g *gin.RouterGroup, catalog *Catalog) *Group {
	if catalog == nil {
		catalog = NewCatalog()
	}
	return &Group{group: g, catalog: catalog}
}

func (g *Group) Group(relativePath string, handlers ...gin.HandlerFunc) *Group {
	return &Group{
		group:        g.group.Group(relativePath, handlers...),
		defaultScope: g.defaultScope,
		catalog:      g.catalog,
	}
}

func (g *Group) Use(handlers ...gin.HandlerFunc) *Group {
	g.group.Use(handlers...)
	return g
}

func (g *Group) Scope(scope string) *Group {
	g.defaultScope = strings.TrimSpace(scope)
	return g
}

func (g *Group) GET(routePath string, handlers ...gin.HandlerFunc) *Route {
	return g.handle("GET", routePath, handlers...)
}

func (g *Group) POST(routePath string, handlers ...gin.HandlerFunc) *Route {
	return g.handle("POST", routePath, handlers...)
}

func (g *Group) PUT(routePath string, handlers ...gin.HandlerFunc) *Route {
	return g.handle("PUT", routePath, handlers...)
}

func (g *Group) DELETE(routePath string, handlers ...gin.HandlerFunc) *Route {
	return g.handle("DELETE", routePath, handlers...)
}

func (g *Group) PATCH(routePath string, handlers ...gin.HandlerFunc) *Route {
	return g.handle("PATCH", routePath, handlers...)
}

func (g *Group) OPTIONS(routePath string, handlers ...gin.HandlerFunc) *Route {
	return g.handle("OPTIONS", routePath, handlers...)
}

func (g *Group) HEAD(routePath string, handlers ...gin.HandlerFunc) *Route {
	return g.handle("HEAD", routePath, handlers...)
}

func (g *Group) handle(method, routePath string, handlers ...gin.HandlerFunc) *Route {
	switch method {
	case "GET":
		g.group.GET(routePath, handlers...)
	case "POST":
		g.group.POST(routePath, handlers...)
	case "PUT":
		g.group.PUT(routePath, handlers...)
	case "DELETE":
		g.group.DELETE(routePath, handlers...)
	case "PATCH":
		g.group.PATCH(routePath, handlers...)
	case "OPTIONS":
		g.group.OPTIONS(routePath, handlers...)
	case "HEAD":
		g.group.HEAD(routePath, handlers...)
	default:
		g.group.Handle(method, routePath, handlers...)
	}

	registeredRoute := &Route{
		method:  strings.ToUpper(method),
		path:    joinRoutePath(g.group.BasePath(), routePath),
		catalog: g.catalog,
	}
	if g.defaultScope != "" {
		registeredRoute.Scope(g.defaultScope)
	}
	return registeredRoute
}

func routeKey(method, routePath string) string {
	return strings.ToUpper(strings.TrimSpace(method)) + " " + normalizeRoutePath(routePath)
}

func joinRoutePath(basePath, routePath string) string {
	return normalizeRoutePath(path.Join(normalizeRoutePath(basePath), routePath))
}

func normalizeRoutePath(routePath string) string {
	routePath = strings.TrimSpace(routePath)
	if routePath == "" {
		return "/"
	}
	if !strings.HasPrefix(routePath, "/") {
		routePath = "/" + routePath
	}
	cleaned := path.Clean(routePath)
	if cleaned == "." {
		return "/"
	}
	return cleaned
}
