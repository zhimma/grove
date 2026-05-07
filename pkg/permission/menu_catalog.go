package permission

import (
	"sort"
	"strings"

	pkgerrors "github.com/zhimma/grove/pkg/errors"
)

type StaticMenuCatalogItem struct {
	MenuKey   string
	ParentKey string
	Name      string
	Title     string
	Path      string
	Icon      string
	Scope     string
	Sort      int
	Visible   bool
}

func ConsoleMenuCatalog() []StaticMenuCatalogItem {
	return []StaticMenuCatalogItem{
		{
			MenuKey: "ConsoleDashboard",
			Name:    "ConsoleDashboard",
			Title:   "工作台",
			Path:    "/dashboard",
			Icon:    "lucide:layout-dashboard",
			Scope:   ScopeGlobal,
			Sort:    10,
			Visible: true,
		},
		{
			MenuKey:   "ConsoleOverview",
			ParentKey: "ConsoleDashboard",
			Name:      "ConsoleOverview",
			Title:     "工作台",
			Path:      "/dashboard/overview",
			Scope:     ScopeGlobal,
			Sort:      11,
			Visible:   true,
		},
		{
			MenuKey: "ConsoleConfigs",
			Name:    "ConsoleConfigs",
			Title:   "配置管理",
			Path:    "/configs",
			Icon:    "lucide:settings-2",
			Scope:   ScopeGlobal,
			Sort:    20,
			Visible: true,
		},
		{
			MenuKey:   "ConsoleSystemConfigs",
			ParentKey: "ConsoleConfigs",
			Name:      "ConsoleSystemConfigs",
			Title:     "系统配置",
			Path:      "/configs/system",
			Scope:     ScopeGlobal,
			Sort:      21,
			Visible:   true,
		},
		{
			MenuKey: "ConsoleSystem",
			Name:    "ConsoleSystem",
			Title:   "系统管理",
			Path:    "/system",
			Icon:    "lucide:settings",
			Scope:   ScopeGlobal,
			Sort:    30,
			Visible: true,
		},
		{
			MenuKey:   "ConsoleAdmins",
			ParentKey: "ConsoleSystem",
			Name:      "ConsoleAdmins",
			Title:     "管理员管理",
			Path:      "/system/admins",
			Scope:     ScopeGlobal,
			Sort:      31,
			Visible:   true,
		},
		{
			MenuKey:   "ConsoleRoles",
			ParentKey: "ConsoleSystem",
			Name:      "ConsoleRoles",
			Title:     "角色权限",
			Path:      "/system/role",
			Scope:     ScopeGlobal,
			Sort:      32,
			Visible:   true,
		},
	}
}

func FilterConsoleMenuKeys(keys []string) []string {
	if len(keys) == 0 {
		return []string{}
	}
	if containsWildcard(keys) {
		return []string{"*"}
	}

	allowed := make(map[string]StaticMenuCatalogItem, len(ConsoleMenuCatalog()))
	for _, item := range ConsoleMenuCatalog() {
		allowed[item.MenuKey] = item
	}

	filtered := make([]string, 0, len(keys))
	seen := map[string]struct{}{}
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if _, ok := allowed[key]; !ok {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		filtered = append(filtered, key)
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		return consoleMenuSortWeight(filtered[i]) < consoleMenuSortWeight(filtered[j])
	})
	return filtered
}

func ValidateConsoleMenuKeys(keys []string) error {
	if len(keys) == 0 || containsWildcard(keys) {
		return nil
	}

	allowed := make(map[string]struct{}, len(ConsoleMenuCatalog()))
	for _, item := range ConsoleMenuCatalog() {
		allowed[item.MenuKey] = struct{}{}
	}

	invalid := make([]string, 0)
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if _, ok := allowed[key]; ok {
			continue
		}
		invalid = append(invalid, key)
	}
	if len(invalid) == 0 {
		return nil
	}
	sort.Strings(invalid)
	return pkgerrors.InvalidParams().WithHTTPStatus(422).WithMessage("存在未注册的菜单权限标识: " + strings.Join(uniqueStrings(invalid), ", "))
}

func BuildConsoleMenuTree() []MenuTreeItem {
	return buildStaticMenuTree(ConsoleMenuCatalog())
}

func buildStaticMenuTree(catalogs []StaticMenuCatalogItem) []MenuTreeItem {
	nodes := make(map[string]*MenuTreeItem, len(catalogs))
	order := make([]string, 0, len(catalogs))

	for _, catalog := range catalogs {
		node := &MenuTreeItem{
			MenuKey:   catalog.MenuKey,
			ParentKey: catalog.ParentKey,
			Name:      catalog.Name,
			Title:     catalog.Title,
			Path:      catalog.Path,
			Icon:      catalog.Icon,
			Scope:     catalog.Scope,
			Sort:      catalog.Sort,
			Visible:   catalog.Visible,
		}
		nodes[catalog.MenuKey] = node
		order = append(order, catalog.MenuKey)
	}

	roots := make([]*MenuTreeItem, 0)
	for _, key := range order {
		node := nodes[key]
		if node == nil {
			continue
		}
		if node.ParentKey == "" {
			roots = append(roots, node)
			continue
		}
		parent := nodes[node.ParentKey]
		if parent == nil {
			roots = append(roots, node)
			continue
		}
		parent.Children = append(parent.Children, *node)
	}

	sortTreeItems(roots)
	result := make([]MenuTreeItem, 0, len(roots))
	for _, root := range roots {
		result = append(result, *root)
	}
	return result
}

func containsWildcard(keys []string) bool {
	for _, key := range keys {
		if strings.TrimSpace(key) == "*" {
			return true
		}
	}
	return false
}

func consoleMenuSortWeight(key string) int {
	catalogs := ConsoleMenuCatalog()
	for index, item := range catalogs {
		if item.MenuKey == key {
			return index
		}
	}
	return len(catalogs) + 1
}

func uniqueStrings(items []string) []string {
	if len(items) == 0 {
		return []string{}
	}

	result := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	return result
}
