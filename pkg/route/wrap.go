package route

import (
	"path"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

type Route struct {
	method string
	path   string
}

func (r *Route) Name(displayName string) *Route {
	routeNameStore.Store(routeKey(r.method, r.path), strings.TrimSpace(displayName))
	return r
}

func (r *Route) Scope(scope string) *Route {
	routeScopeStore.Store(routeKey(r.method, r.path), strings.TrimSpace(scope))
	return r
}

func (r *Route) Ignore() *Route {
	ignoredRouteStore.Store(routeKey(r.method, r.path), true)
	return r
}

var (
	routeNameStore    sync.Map
	routeScopeStore   sync.Map
	ignoredRouteStore sync.Map
)

func GetName(method, routePath string) (string, bool) {
	value, ok := routeNameStore.Load(routeKey(method, routePath))
	if !ok {
		return "", false
	}
	name, ok := value.(string)
	return name, ok && name != ""
}

func GetScope(method, routePath string) (string, bool) {
	value, ok := routeScopeStore.Load(routeKey(method, routePath))
	if !ok {
		return "", false
	}
	scope, ok := value.(string)
	return scope, ok && scope != ""
}

func IsIgnored(method, routePath string) bool {
	value, ok := ignoredRouteStore.Load(routeKey(method, routePath))
	if !ok {
		return false
	}
	ignored, _ := value.(bool)
	return ignored
}

type Group struct {
	group        *gin.RouterGroup
	defaultScope string
}

func Wrap(g *gin.RouterGroup) *Group {
	return &Group{group: g}
}

func (g *Group) Group(relativePath string, handlers ...gin.HandlerFunc) *Group {
	return &Group{
		group:        g.group.Group(relativePath, handlers...),
		defaultScope: g.defaultScope,
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
		method: strings.ToUpper(method),
		path:   joinRoutePath(g.group.BasePath(), routePath),
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

func ResetForTest() {
	routeNameStore = sync.Map{}
	routeScopeStore = sync.Map{}
	ignoredRouteStore = sync.Map{}
}
