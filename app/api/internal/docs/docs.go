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

	docsui.RegisterScalarDocs(router, func(_ *gin.Context) (docsui.Document, error) {
		return spec(cfg), nil
	}, docsui.ScalarOptions{
		Title:       strings.TrimSpace(cfg.Docs.Title),
		DocsPath:    "/docs",
		OpenAPIPath: "/docs/openapi.json",
		ScriptURL:   cfg.Docs.ScalarScriptURL,
		Targets:     targets,
	})
}

func demoEnabled(cfg *config.Config) bool {
	return cfg != nil && cfg.Demo.Enabled && !strings.EqualFold(strings.TrimSpace(cfg.App.Env), "production")
}

func resolveAPIBasePath(cfg *config.Config) string {
	if strings.TrimSpace(cfg.API.Prefix) != "" {
		return strings.TrimSpace(cfg.API.Prefix)
	}
	basePath := strings.TrimSpace(cfg.Docs.BasePath)
	if basePath != "" {
		return basePath
	}
	return "/api/v1"
}
