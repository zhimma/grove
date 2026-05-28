package handler

import (
	"github.com/gin-gonic/gin"

	consoleservice "github.com/zhimma/grove/app/console/internal/service"
	"github.com/zhimma/grove/pkg/response"
	"github.com/zhimma/grove/pkg/route"
)

type PermissionHandler struct {
	runtimeCatalog *consoleservice.RuntimePermissionCatalog
}

type APIPermissionTreeItem struct {
	Key        string                  `json:"key"`
	Title      string                  `json:"title"`
	Identifier string                  `json:"identifier,omitempty"`
	Method     string                  `json:"method,omitempty"`
	Path       string                  `json:"path,omitempty"`
	Children   []APIPermissionTreeItem `json:"children,omitempty"`
}

func RegisterPermissionRoutes(protected *gin.RouterGroup, runtimeCatalog *consoleservice.RuntimePermissionCatalog) {
	h := &PermissionHandler{runtimeCatalog: runtimeCatalog}
	permissions := route.Wrap(protected.Group("/permissions"))
	permissions.GET("/apis", h.GetAPIPermissionOptions).Name("权限管理.接口权限选项")
}

func (h *PermissionHandler) GetAPIPermissionOptions(c *gin.Context) {
	response.Success(c, buildAPIPermissionTree(h.runtimeCatalog.ListAPIPermissions()))
}

func buildAPIPermissionTree(options []consoleservice.APIPermissionOption) []APIPermissionTreeItem {
	grouped := make(map[string][]APIPermissionTreeItem)
	orderedCategories := make([]string, 0)

	for _, item := range options {
		category := item.Category
		if _, ok := grouped[category]; !ok {
			orderedCategories = append(orderedCategories, category)
		}
		grouped[category] = append(grouped[category], APIPermissionTreeItem{
			Key:        item.Identifier,
			Title:      item.Name,
			Identifier: item.Identifier,
			Method:     item.Method,
			Path:       item.Path,
		})
	}

	tree := make([]APIPermissionTreeItem, 0, len(orderedCategories))
	for _, category := range orderedCategories {
		tree = append(tree, APIPermissionTreeItem{
			Key:      category,
			Title:    category,
			Children: grouped[category],
		})
	}
	return tree
}
