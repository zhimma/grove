package docsui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type OpenAPITarget struct {
	ID          string
	Label       string
	UpstreamURL string
}

type Document struct {
	Title       string
	Description string
	Version     string
	Servers     []string
	Paths       []Path
}

type Path struct {
	Path       string
	Operations []Operation
}

type Operation struct {
	Method      string
	Summary     string
	Response200 string
	BearerAuth  bool
	Parameters  []Parameter
}

type Parameter struct {
	Name        string
	In          string
	Required    bool
	Type        string
	Description string
}

type ScalarOptions struct {
	Title       string
	DocsPath    string
	OpenAPIPath string
	Targets     []OpenAPITarget
}

type OpenAPIDocumentBuilder func(*gin.Context) (map[string]any, error)

func BuildOpenAPIDocument(doc Document) map[string]any {
	paths := make(map[string]any, len(doc.Paths))
	for _, path := range doc.Paths {
		operations := make(map[string]any, len(path.Operations))
		for _, operation := range path.Operations {
			method := strings.ToLower(strings.TrimSpace(operation.Method))
			if method == "" {
				continue
			}

			op := map[string]any{
				"summary": operation.Summary,
				"responses": map[string]any{
					"200": map[string]any{
						"description": operation.Response200,
					},
				},
			}
			if operation.BearerAuth {
				op["security"] = []map[string]any{{"BearerAuth": []string{}}}
			}
			if len(operation.Parameters) > 0 {
				parameters := make([]map[string]any, 0, len(operation.Parameters))
				for _, parameter := range operation.Parameters {
					param := map[string]any{
						"name":     parameter.Name,
						"in":       parameter.In,
						"required": parameter.Required,
						"schema": map[string]any{
							"type": strings.TrimSpace(parameter.Type),
						},
					}
					if parameter.Description != "" {
						param["description"] = parameter.Description
					}
					parameters = append(parameters, param)
				}
				op["parameters"] = parameters
			}
			operations[method] = op
		}
		if len(operations) > 0 {
			paths[path.Path] = operations
		}
	}

	servers := make([]map[string]any, 0, len(doc.Servers))
	for _, server := range doc.Servers {
		server = strings.TrimSpace(server)
		if server == "" {
			continue
		}
		servers = append(servers, map[string]any{"url": server})
	}

	result := map[string]any{
		"openapi": "3.0.3",
		"info": map[string]any{
			"title":       doc.Title,
			"description": doc.Description,
			"version":     doc.Version,
		},
		"paths": paths,
		"components": map[string]any{
			"securitySchemes": map[string]any{
				"BearerAuth": map[string]any{
					"type":         "http",
					"scheme":       "bearer",
					"bearerFormat": "JWT",
				},
			},
		},
	}
	if len(servers) > 0 {
		result["servers"] = servers
	}
	return result
}

func RegisterScalarDocs(router gin.IRoutes, builder OpenAPIDocumentBuilder, opts ScalarOptions) {
	if router == nil || builder == nil {
		return
	}

	docsPath := normalizePath(opts.DocsPath, "/docs")
	openAPIPath := strings.TrimSpace(opts.OpenAPIPath)
	if openAPIPath == "" {
		openAPIPath = docsPath + "/openapi.json"
	}
	title := strings.TrimSpace(opts.Title)
	if title == "" {
		title = "API Docs"
	}

	router.GET(docsPath, serveScalarPage(title, openAPIPath))
	router.GET(openAPIPath, func(c *gin.Context) {
		doc, err := builder(c)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    http.StatusInternalServerError,
				"message": "构建接口文档失败",
			})
			return
		}
		payload, err := json.Marshal(doc)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    http.StatusInternalServerError,
				"message": "序列化接口文档失败",
			})
			return
		}
		c.Data(http.StatusOK, "application/json; charset=utf-8", payload)
	})
	if len(opts.Targets) > 0 {
		router.Any(docsPath+"/http-proxy/*target", serveScalarProxy(opts.Targets))
	}
}

func serveScalarPage(title string, openAPIPath string) gin.HandlerFunc {
	return func(c *gin.Context) {
		customCSS := `
        .light-mode {
          --scalar-background-1: #f7fafc;
          --scalar-background-2: #ffffff;
          --scalar-background-3: #edf2f7;
          --scalar-color-1: #1a202c;
          --scalar-color-2: #4a5568;
          --scalar-color-3: #718096;
          --scalar-color-accent: #0f766e;
          --scalar-border-color: rgba(15, 23, 42, 0.08);
        }
      `
		html := fmt.Sprintf(`<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>%s</title>
  <style>
    :root { color-scheme: light; }
    html, body, #app { height: 100%%; }
    body {
      margin: 0;
      background: linear-gradient(180deg, #f7fafc 0%%, #edf2f7 100%%);
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
    }
  </style>
</head>
<body>
  <div id="app"></div>
  <script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script>
  <script>
    const docsOpenApiUrl = new URL(%s, window.location.origin).toString()
    Scalar.createApiReference('#app', {
      url: docsOpenApiUrl,
      theme: 'default',
      layout: 'modern',
      searchHotKey: 'k',
      hideDownloadButton: false,
      hideClientButton: true,
      hiddenClients: true,
      telemetry: false,
      showDeveloperTools: 'never',
      withDefaultFonts: false,
      customCss: %s
    })
  </script>
</body>
</html>`, title, strconv.Quote(openAPIPath), strconv.Quote(customCSS))

		c.Header(
			"Content-Security-Policy",
			"default-src 'self'; script-src 'self' 'unsafe-inline' blob: https://cdn.jsdelivr.net; script-src-elem 'self' 'unsafe-inline' blob: https://cdn.jsdelivr.net; style-src 'self' 'unsafe-inline'; connect-src 'self'; img-src 'self' data: https:;",
		)
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
	}
}

func serveScalarProxy(targets []OpenAPITarget) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(targets))
	for _, target := range targets {
		target = normalizeTarget(target)
		if target.UpstreamURL != "" {
			allowed[target.UpstreamURL] = struct{}{}
		}
	}

	return func(c *gin.Context) {
		targetURL, err := resolveProxyTarget(c.Param("target"), c.Request.URL.RawQuery, allowed)
		if err != nil {
			status := http.StatusBadRequest
			message := "非法文档代理目标"
			if strings.Contains(err.Error(), "not allowed") {
				status = http.StatusForbidden
				message = "文档代理目标未授权"
			}
			c.JSON(status, gin.H{
				"code":    status,
				"message": message,
			})
			return
		}

		proxy := httputil.NewSingleHostReverseProxy(targetURL)
		proxy.Rewrite = func(req *httputil.ProxyRequest) {
			req.SetURL(targetURL)
			req.Out.Host = targetURL.Host
		}
		proxy.ErrorHandler = func(rw http.ResponseWriter, _ *http.Request, _ error) {
			rw.Header().Set("Content-Type", "application/json; charset=utf-8")
			rw.WriteHeader(http.StatusBadGateway)
			_, _ = rw.Write([]byte(`{"code":502,"message":"文档代理请求失败"}`))
		}
		proxy.ServeHTTP(c.Writer, c.Request)
	}
}

func normalizeTarget(target OpenAPITarget) OpenAPITarget {
	target.ID = strings.TrimSpace(target.ID)
	target.Label = strings.TrimSpace(target.Label)
	target.UpstreamURL = strings.TrimSpace(target.UpstreamURL)
	if target.Label == "" {
		target.Label = target.ID
	}
	return target
}

func normalizePath(value, fallback string) string {
	path := strings.TrimSpace(value)
	if path == "" {
		path = fallback
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return strings.TrimRight(path, "/")
}

func resolveProxyTarget(raw string, rawQuery string, allowed map[string]struct{}) (*url.URL, error) {
	baseURL, suffix, err := parseProxyTarget(raw)
	if err != nil {
		return nil, err
	}
	if _, ok := allowed[baseURL.String()]; !ok {
		return nil, fmt.Errorf("proxy target is not allowed")
	}

	targetURL := *baseURL
	targetURL.Path = joinURLPath(baseURL.Path, suffix)
	targetURL.RawQuery = rawQuery
	return &targetURL, nil
}

func parseProxyTarget(raw string) (*url.URL, string, error) {
	trimmed := strings.TrimPrefix(strings.TrimSpace(raw), "/")
	if trimmed == "" {
		return nil, "", fmt.Errorf("missing proxy target")
	}
	parts := strings.SplitN(trimmed, "/", 2)
	base, err := url.PathUnescape(parts[0])
	if err != nil {
		return nil, "", err
	}
	parsed, err := url.Parse(base)
	if err != nil {
		return nil, "", err
	}
	suffix := ""
	if len(parts) == 2 {
		suffix = parts[1]
	}
	return parsed, suffix, nil
}

func joinURLPath(basePath string, suffix string) string {
	basePath = strings.TrimRight(basePath, "/")
	suffix = strings.TrimLeft(suffix, "/")
	switch {
	case basePath == "" && suffix == "":
		return "/"
	case basePath == "":
		return "/" + suffix
	case suffix == "":
		return basePath
	default:
		return basePath + "/" + suffix
	}
}
