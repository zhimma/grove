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
			UpstreamURL: "http://127.0.0.1:" + strings.TrimSpace(cfg.Port) + resolveAPIBasePath(cfg),
		},
	}

	docsui.RegisterScalarDocs(router, func(_ *gin.Context) (map[string]any, error) {
		return spec(cfg), nil
	}, docsui.ScalarOptions{
		Title:       strings.TrimSpace(cfg.Docs.Title),
		DocsPath:    "/docs",
		OpenAPIPath: "/docs/openapi.json",
		Targets:     targets,
	})
}

func spec(cfg *config.Config) map[string]any {
	basePath := resolveAPIBasePath(cfg)

	return docsui.BuildOpenAPIDocument(docsui.Document{
		Title:       cfg.Docs.Title,
		Description: cfg.Docs.Description,
		Version:     cfg.Docs.Version,
		Servers:     []string{basePath},
		Paths: []docsui.Path{
			{
				Path: "/health",
				Operations: []docsui.Operation{
					{Method: "GET", Summary: "Health check", Response200: "ok"},
				},
			},
			{
				Path: "/ping",
				Operations: []docsui.Operation{
					{
						Method:      "GET",
						Summary:     "Public ping",
						Response200: "pong",
						Parameters: []docsui.Parameter{
							{Name: "name", In: "query", Type: "string"},
						},
					},
				},
			},
			{
				Path: "/auth/access-token",
				Operations: []docsui.Operation{
					{Method: "POST", Summary: "Issue access token", Response200: "token issued"},
				},
			},
			{
				Path: "/profile",
				Operations: []docsui.Operation{
					{Method: "GET", Summary: "Current user profile", Response200: "profile", BearerAuth: true},
				},
			},
			{
				Path: "/jobs/echo",
				Operations: []docsui.Operation{
					{Method: "POST", Summary: "Enqueue echo job", Response200: "queued", BearerAuth: true},
				},
			},
		},
	})
}

func resolveAPIBasePath(cfg *config.Config) string {
	basePath := strings.TrimSpace(cfg.Docs.BasePath)
	if basePath != "" {
		return basePath
	}
	if strings.TrimSpace(cfg.API.Prefix) != "" {
		return strings.TrimSpace(cfg.API.Prefix)
	}
	return "/api/v1"
}
