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

	docsui.RegisterScalarDocs(router, func(_ *gin.Context) (docsui.Document, error) {
		return spec(cfg), nil
	}, docsui.ScalarOptions{
		Title:       "Console - " + strings.TrimSpace(cfg.Docs.Title),
		DocsPath:    "/console/docs",
		OpenAPIPath: "/console/docs/openapi.json",
		ScriptURL:   cfg.Docs.ScalarScriptURL,
		Targets:     targets,
	})
}
