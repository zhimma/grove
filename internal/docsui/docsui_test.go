package docsui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestBuildOpenAPIDocument(t *testing.T) {
	doc := BuildOpenAPIDocument(Document{
		Title:       "Test Docs",
		Description: "test description",
		Version:     "1.0.0",
		Servers:     []string{"/api/v1"},
		Paths: []Path{
			{
				Path: "/ping",
				Operations: []Operation{
					{
						Method:      http.MethodGet,
						Summary:     "Ping",
						Response200: "pong",
						Parameters: []Parameter{
							{
								Name:     "name",
								In:       "query",
								Required: false,
								Type:     "string",
							},
						},
					},
				},
			},
			{
				Path: "/profile",
				Operations: []Operation{
					{
						Method:      http.MethodGet,
						Summary:     "Profile",
						Response200: "profile",
						BearerAuth:  true,
					},
				},
			},
		},
	})

	if doc["openapi"] != "3.0.3" {
		t.Fatalf("expected openapi 3.0.3, got %#v", doc["openapi"])
	}

	paths, ok := doc["paths"].(map[string]any)
	if !ok {
		t.Fatalf("expected paths map, got %#v", doc["paths"])
	}

	pingPath, ok := paths["/ping"].(map[string]any)
	if !ok {
		t.Fatalf("expected /ping path, got %#v", paths["/ping"])
	}
	getOp, ok := pingPath["get"].(map[string]any)
	if !ok {
		t.Fatalf("expected get operation, got %#v", pingPath["get"])
	}
	if getOp["summary"] != "Ping" {
		t.Fatalf("expected Ping summary, got %#v", getOp["summary"])
	}

	profilePath, ok := paths["/profile"].(map[string]any)
	if !ok {
		t.Fatalf("expected /profile path, got %#v", paths["/profile"])
	}
	profileGet, ok := profilePath["get"].(map[string]any)
	if !ok {
		t.Fatalf("expected /profile get operation, got %#v", profilePath["get"])
	}
	if _, ok := profileGet["security"]; !ok {
		t.Fatalf("expected bearer security on profile operation")
	}
}

func TestRegisterScalarDocs(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	RegisterScalarDocs(engine, func(_ *gin.Context) (map[string]any, error) {
		return map[string]any{
			"openapi": "3.0.3",
			"info": map[string]any{
				"title": "Test Docs",
			},
		}, nil
	}, ScalarOptions{
		Title:       "Test Docs",
		DocsPath:    "/docs",
		OpenAPIPath: "/docs/openapi.json",
	})

	pageReq := httptest.NewRequest(http.MethodGet, "/docs", nil)
	pageResp := httptest.NewRecorder()
	engine.ServeHTTP(pageResp, pageReq)
	if pageResp.Code != http.StatusOK {
		t.Fatalf("expected docs page 200, got %d", pageResp.Code)
	}
	if !strings.Contains(pageResp.Body.String(), "@scalar/api-reference") {
		t.Fatalf("expected scalar page content, got %s", pageResp.Body.String())
	}

	openapiReq := httptest.NewRequest(http.MethodGet, "/docs/openapi.json", nil)
	openapiResp := httptest.NewRecorder()
	engine.ServeHTTP(openapiResp, openapiReq)
	if openapiResp.Code != http.StatusOK {
		t.Fatalf("expected openapi 200, got %d", openapiResp.Code)
	}
	if !strings.Contains(openapiResp.Body.String(), `"openapi":"3.0.3"`) {
		t.Fatalf("unexpected openapi response: %s", openapiResp.Body.String())
	}
}
