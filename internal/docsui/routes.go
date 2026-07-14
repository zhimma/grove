package docsui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
)

func CompareRoutes(routes gin.RoutesInfo, document Document, basePath string) error {
	if err := document.Validate(); err != nil {
		return err
	}
	basePath = normalizeOpenAPIPath(basePath)
	actual := map[string]struct{}{}
	for _, route := range routes {
		method := strings.ToUpper(strings.TrimSpace(route.Method))
		if !isDocumentedMethod(method) {
			continue
		}
		path := normalizeGinPath(route.Path)
		if !pathWithinBase(path, basePath) {
			continue
		}
		actual[method+" "+path] = struct{}{}
	}

	expected := map[string]struct{}{}
	for path, item := range document.Paths {
		fullPath := joinSpecPath(basePath, path)
		for method := range item {
			method = strings.ToUpper(strings.TrimSpace(method))
			if isDocumentedMethod(method) {
				expected[method+" "+fullPath] = struct{}{}
			}
		}
	}

	missing := difference(actual, expected)
	extra := difference(expected, actual)
	if len(missing) == 0 && len(extra) == 0 {
		return nil
	}
	parts := make([]string, 0, 2)
	if len(missing) > 0 {
		parts = append(parts, "missing OpenAPI operations: "+strings.Join(missing, ", "))
	}
	if len(extra) > 0 {
		parts = append(parts, "OpenAPI operations without Gin routes: "+strings.Join(extra, ", "))
	}
	return fmt.Errorf("route contract drift: %s", strings.Join(parts, "; "))
}

func normalizeGinPath(path string) string {
	path = normalizeOpenAPIPath(path)
	if path == "" || path == "/" {
		return path
	}
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if len(part) > 1 && (part[0] == ':' || part[0] == '*') {
			parts[i] = "{" + part[1:] + "}"
		}
	}
	return strings.Join(parts, "/")
}

func pathWithinBase(path, basePath string) bool {
	if basePath == "" || basePath == "/" {
		return true
	}
	return path == basePath || strings.HasPrefix(path, basePath+"/")
}

func joinSpecPath(basePath, path string) string {
	path = normalizeOpenAPIPath(path)
	if basePath == "" || basePath == "/" {
		return path
	}
	if path == basePath || strings.HasPrefix(path, basePath+"/") {
		return path
	}
	if path == "/" {
		return basePath
	}
	return basePath + path
}

func difference(left, right map[string]struct{}) []string {
	values := make([]string, 0)
	for value := range left {
		if _, exists := right[value]; !exists {
			values = append(values, value)
		}
	}
	sort.Strings(values)
	return values
}
