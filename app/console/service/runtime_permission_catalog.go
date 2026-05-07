package service

import (
	"sort"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"

	"github.com/zhimma/grove/pkg/permission"
)

type APIPermissionOption struct {
	Identifier string `json:"identifier"`
	Method     string `json:"method"`
	Path       string `json:"path"`
	Name       string `json:"name"`
	Category   string `json:"category"`
}

type RuntimePermissionCatalog struct {
	mu             sync.RWMutex
	apiPermissions []APIPermissionOption
	identifiers    map[string]struct{}
}

func NewRuntimePermissionCatalog() *RuntimePermissionCatalog {
	return &RuntimePermissionCatalog{
		identifiers: map[string]struct{}{},
	}
}

func (c *RuntimePermissionCatalog) LoadRoutes(routes gin.RoutesInfo) {
	options := buildConsoleAPIPermissionOptions(routes)
	identifiers := make(map[string]struct{}, len(options))
	for _, item := range options {
		identifiers[item.Identifier] = struct{}{}
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.apiPermissions = options
	c.identifiers = identifiers
}

func (c *RuntimePermissionCatalog) ListAPIPermissions() []APIPermissionOption {
	c.mu.RLock()
	defer c.mu.RUnlock()

	items := make([]APIPermissionOption, len(c.apiPermissions))
	copy(items, c.apiPermissions)
	return items
}

func (c *RuntimePermissionCatalog) HasAPIIdentifier(identifier string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	_, ok := c.identifiers[strings.TrimSpace(identifier)]
	return ok
}

func buildConsoleAPIPermissionOptions(routes gin.RoutesInfo) []APIPermissionOption {
	collected := permission.CollectProtectedRoutes(
		routes,
		permission.AppConsole,
		"console",
		"admin_auth",
	)

	items := make([]APIPermissionOption, 0, len(collected))
	for _, item := range collected {
		name := permission.BuildDisplayName(item.Method, item.Path)
		category := permission.BuildModuleCode(item.Path)
		if namedCategory, ok := permission.BuildModuleCodeFromDisplayName(name); ok {
			category = namedCategory
		}

		items = append(items, APIPermissionOption{
			Identifier: permission.BuildAPIIdentifier(item.Method, item.Path),
			Method:     item.Method,
			Path:       item.Path,
			Name:       name,
			Category:   category,
		})
	}

	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Category == items[j].Category {
			if items[i].Method == items[j].Method {
				return items[i].Path < items[j].Path
			}
			return items[i].Method < items[j].Method
		}
		return items[i].Category < items[j].Category
	})

	return items
}
