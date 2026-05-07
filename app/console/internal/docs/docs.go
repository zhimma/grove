package docs

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/zhimma/grove/internal/config"
	"github.com/zhimma/grove/internal/docsui"
)

func RegisterDocs(router *gin.Engine, cfg *config.Config) {
	if router == nil || cfg == nil || !cfg.Docs.Enabled {
		return
	}

	targets := []docsui.OpenAPITarget{
		{
			ID:          "local",
			Label:       "本地",
			UpstreamURL: "http://127.0.0.1:" + strings.TrimSpace(cfg.ConsolePort) + "/console/v1",
		},
	}

	docsui.RegisterScalarDocs(router, func(_ *gin.Context) (map[string]any, error) {
		return spec(cfg), nil
	}, docsui.ScalarOptions{
		Title:       "Console - " + strings.TrimSpace(cfg.Docs.Title),
		DocsPath:    "/console/docs",
		OpenAPIPath: "/console/docs/openapi.json",
		Targets:     targets,
	})
}

func spec(cfg *config.Config) map[string]any {
	return docsui.BuildOpenAPIDocument(docsui.Document{
		Title:       "Console - " + cfg.Docs.Title,
		Description: "Console admin endpoints",
		Version:     cfg.Docs.Version,
		Servers:     []string{"/console/v1"},
		Paths: []docsui.Path{
			{Path: "/auth/login", Operations: []docsui.Operation{{Method: "POST", Summary: "Console admin login", Response200: "login success"}}},
			{Path: "/auth/refresh", Operations: []docsui.Operation{{Method: "POST", Summary: "Refresh access token", Response200: "refresh success"}}},
			{Path: "/auth/logout", Operations: []docsui.Operation{{Method: "POST", Summary: "Logout current admin session", Response200: "logout success", BearerAuth: true}}},
			{
				Path: "/auth/me",
				Operations: []docsui.Operation{
					{Method: "GET", Summary: "Current admin profile", Response200: "current admin", BearerAuth: true},
					{Method: "PUT", Summary: "Update current admin profile", Response200: "updated current admin", BearerAuth: true},
				},
			},
			{Path: "/auth/permissions", Operations: []docsui.Operation{{Method: "GET", Summary: "Current admin authorization overview", Response200: "authorization overview", BearerAuth: true}}},
			{Path: "/permissions/apis", Operations: []docsui.Operation{{Method: "GET", Summary: "Runtime API permission options", Response200: "api permission tree", BearerAuth: true}}},
			{Path: "/dashboard/summary", Operations: []docsui.Operation{{Method: "GET", Summary: "Dashboard summary", Response200: "summary", BearerAuth: true}}},
			{
				Path: "/roles",
				Operations: []docsui.Operation{
					{Method: "GET", Summary: "Role list", Response200: "role list", BearerAuth: true},
					{Method: "POST", Summary: "Create role", Response200: "role detail", BearerAuth: true},
				},
			},
			{
				Path: "/roles/{id}",
				Operations: []docsui.Operation{
					{Method: "GET", Summary: "Role detail", Response200: "role detail", BearerAuth: true},
					{Method: "PUT", Summary: "Update role", Response200: "role detail", BearerAuth: true},
					{Method: "DELETE", Summary: "Delete role", Response200: "deleted", BearerAuth: true},
				},
			},
			{
				Path: "/roles/{id}/permissions",
				Operations: []docsui.Operation{
					{Method: "GET", Summary: "Role permissions", Response200: "permission keys", BearerAuth: true},
					{Method: "POST", Summary: "Assign role permissions", Response200: "assignment success", BearerAuth: true},
				},
			},
			{
				Path: "/roles/{id}/menus",
				Operations: []docsui.Operation{
					{Method: "GET", Summary: "Role menus", Response200: "menu keys", BearerAuth: true},
					{Method: "POST", Summary: "Assign role menus", Response200: "assignment success", BearerAuth: true},
				},
			},
		},
	})
}
