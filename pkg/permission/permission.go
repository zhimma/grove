package permission

import (
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/zhimma/grove/pkg/route"
)

const (
	AppConsole  = "console"
	ScopeGlobal = "global"
)

type CatalogRoute struct {
	AppCode     string
	ServiceCode string
	AuthScope   string
	Scope       string
	Method      string
	Path        string
}

var nonAlphaNum = regexp.MustCompile(`[^a-z0-9]+`)

func NormalizeScope(scope string) string {
	normalized := strings.TrimSpace(strings.ToLower(scope))
	if normalized == "" {
		return ScopeGlobal
	}
	return normalized
}

func BuildPermissionKey(app, method, path string) string {
	return strings.ToLower(strings.TrimSpace(app)) + ":" + strings.ToUpper(strings.TrimSpace(method)) + ":" + strings.TrimSpace(path)
}

func BuildAPIIdentifier(method, fullPath string) string {
	return strings.ToUpper(strings.TrimSpace(method)) + " " + strings.TrimSpace(fullPath)
}

func BuildDisplayName(method, fullPath string) string {
	return BuildDisplayNameWithCatalog(nil, method, fullPath)
}

func BuildDisplayNameWithCatalog(catalog *route.Catalog, method, fullPath string) string {
	if name, ok := routeName(catalog, method, fullPath); ok {
		return name
	}
	module := BuildModuleCode(fullPath)
	return strings.ToUpper(strings.TrimSpace(method)) + " " + module
}

func BuildModuleCode(fullPath string) string {
	segments := splitPermissionSegments(fullPath)
	if len(segments) == 0 {
		return "root"
	}
	return segments[0]
}

func BuildModuleCodeFromDisplayName(displayName string) (string, bool) {
	segments := splitDisplayNameSegments(displayName)
	if len(segments) < 2 {
		return "", false
	}
	return segments[0], true
}

func CollectProtectedRoutes(routes gin.RoutesInfo, appCode, serviceCode, authScope string) []CatalogRoute {
	return CollectProtectedRoutesWithCatalog(routes, appCode, serviceCode, authScope, nil)
}

func CollectProtectedRoutesWithCatalog(routes gin.RoutesInfo, appCode, serviceCode, authScope string, catalog *route.Catalog) []CatalogRoute {
	items := make([]CatalogRoute, 0, len(routes))
	for _, r := range routes {
		skip := shouldSkipRouteWithCatalog(appCode, r.Method, r.Path, catalog)
		if catalog == nil {
			skip = shouldSkipRoute(appCode, r.Method, r.Path)
		}
		if skip {
			continue
		}
		scope := resolveCatalogRouteScopeWithCatalog(r.Method, r.Path, catalog)
		if catalog == nil {
			scope = resolveCatalogRouteScope(r.Method, r.Path)
		}
		items = append(items, CatalogRoute{
			AppCode:     appCode,
			ServiceCode: serviceCode,
			AuthScope:   authScope,
			Scope:       scope,
			Method:      r.Method,
			Path:        r.Path,
		})
	}
	return items
}

func shouldSkipRoute(appCode, method, routePath string) bool {
	return shouldSkipRouteWithCatalog(appCode, method, routePath, nil)
}

func shouldSkipRouteWithCatalog(appCode, method, routePath string, catalog *route.Catalog) bool {
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case "HEAD", "OPTIONS":
		return true
	}
	if isIgnored(catalog, method, routePath) {
		return true
	}
	switch appCode {
	case AppConsole:
		return !strings.HasPrefix(routePath, "/console/v1/")
	default:
		return true
	}
}

func resolveCatalogRouteScope(method, routePath string) string {
	return resolveCatalogRouteScopeWithCatalog(method, routePath, nil)
}

func resolveCatalogRouteScopeWithCatalog(method, routePath string, catalog *route.Catalog) string {
	if scope, ok := routeScope(catalog, method, routePath); ok {
		return NormalizeScope(scope)
	}
	return ScopeGlobal
}

func routeName(catalog *route.Catalog, method, routePath string) (string, bool) {
	if catalog != nil {
		return catalog.GetName(method, routePath)
	}
	return route.GetName(method, routePath)
}

func routeScope(catalog *route.Catalog, method, routePath string) (string, bool) {
	if catalog != nil {
		return catalog.GetScope(method, routePath)
	}
	return route.GetScope(method, routePath)
}

func isIgnored(catalog *route.Catalog, method, routePath string) bool {
	if catalog != nil {
		return catalog.IsIgnored(method, routePath)
	}
	return route.IsIgnored(method, routePath)
}

func splitPermissionSegments(fullPath string) []string {
	fullPath = strings.TrimSpace(fullPath)
	fullPath = strings.Trim(fullPath, "/")
	if fullPath == "" {
		return nil
	}
	parts := strings.Split(fullPath, "/")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" || part == "api" || part == "console" || part == "merchant" || part == "v1" {
			continue
		}
		part = strings.TrimPrefix(part, ":")
		part = strings.ReplaceAll(part, "*", "wildcard")
		part = nonAlphaNum.ReplaceAllString(strings.ToLower(part), "_")
		part = strings.Trim(part, "_")
		if part == "" {
			continue
		}
		result = append(result, part)
	}
	return result
}

func splitDisplayNameSegments(displayName string) []string {
	parts := strings.Split(strings.TrimSpace(displayName), ".")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		result = append(result, part)
	}
	return result
}
