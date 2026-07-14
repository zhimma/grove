package docsui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type docsTestRequest struct {
	Name   string   `json:"name" form:"name" binding:"required,min=2,max=20" label:"名称"`
	Status string   `json:"status" form:"status" binding:"omitempty,oneof=active disabled" label:"状态"`
	Tags   []string `json:"tags" form:"tags"`
}

type docsTestResponse struct {
	ID   string         `json:"id"`
	Name string         `json:"name"`
	Meta map[string]any `json:"meta"`
}

func TestDocumentBuildsTypedOperationAndSchemas(t *testing.T) {
	doc := NewDocument("Test Docs", "test description", "1.0.0", "/api/v1")
	requestRef := doc.AddSchema("DocsTestRequest", docsTestRequest{})
	responseRef := doc.AddSchema("DocsTestResponse", docsTestResponse{})
	doc.Add("/items", Operation{
		Method:      http.MethodPost,
		ID:          "createItem",
		Summary:     "Create item",
		BearerAuth:  true,
		Parameters:  ParametersFor(docsTestRequest{}, "form", "query"),
		RequestBody: JSONBody("item", requestRef, true),
		Responses:   StandardResponses("created", responseRef),
	})

	if err := doc.Validate(); err != nil {
		t.Fatalf("validate document: %v", err)
	}
	op := doc.Paths["/items"]["post"]
	if op.ID != "createItem" || len(op.Security) != 1 {
		t.Fatalf("unexpected operation: %#v", op)
	}
	if op.RequestBody.Content["application/json"].Schema.Ref != "#/components/schemas/DocsTestRequest" {
		t.Fatalf("unexpected request schema: %#v", op.RequestBody)
	}
	responseSchema := op.Responses["200"].Content["application/json"].Schema
	if responseSchema.Properties["data"].Ref != "#/components/schemas/DocsTestResponse" {
		t.Fatalf("unexpected success response schema: %#v", responseSchema)
	}
	requestSchema := doc.Components.Schemas["DocsTestRequest"]
	if len(requestSchema.Required) != 1 || requestSchema.Required[0] != "name" {
		t.Fatalf("unexpected required fields: %#v", requestSchema.Required)
	}
	if requestSchema.Properties["name"].MinLength == nil || *requestSchema.Properties["name"].MinLength != 2 {
		t.Fatalf("expected minLength from binding: %#v", requestSchema.Properties["name"])
	}
	if additional, ok := doc.Components.Schemas["DocsTestResponse"].Properties["meta"].AdditionalProperties.(bool); !ok || !additional {
		t.Fatalf("map[string]any must allow arbitrary properties: %#v", doc.Components.Schemas["DocsTestResponse"].Properties["meta"])
	}
	if len(op.Parameters) != 3 || op.Parameters[0].Name != "name" || !op.Parameters[0].Required {
		t.Fatalf("unexpected parameters: %#v", op.Parameters)
	}
}

func TestCompareRoutesNormalizesGinParametersAndDetectsDrift(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/api/v1/items/:id", func(*gin.Context) {})

	doc := NewDocument("Test", "", "1.0.0", "/api/v1")
	doc.Add("/items/{id}", Operation{
		Method:    http.MethodGet,
		ID:        "getItem",
		Summary:   "Get item",
		Responses: StandardResponses("item", Schema{}),
	})
	if err := CompareRoutes(engine.Routes(), doc, "/api/v1"); err != nil {
		t.Fatalf("compare routes: %v", err)
	}

	doc.Add("/items", Operation{
		Method:    http.MethodPost,
		ID:        "createItem",
		Summary:   "Create item",
		Responses: StandardResponses("item", Schema{}),
	})
	err := CompareRoutes(engine.Routes(), doc, "/api/v1")
	if err == nil || !strings.Contains(err.Error(), "POST /api/v1/items") {
		t.Fatalf("expected extra operation error, got %v", err)
	}
}

func TestRegisterScalarDocsSupportsSelfHostedScript(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	RegisterScalarDocs(engine, func(_ *gin.Context) (Document, error) {
		doc := NewDocument("Test Docs", "", "1.0.0")
		doc.Add("/health", Operation{
			Method:    http.MethodGet,
			ID:        "health",
			Summary:   "Health",
			Responses: StandardResponses("ok", Schema{}),
		})
		return doc, nil
	}, ScalarOptions{
		Title:       "Test Docs",
		DocsPath:    "/docs",
		OpenAPIPath: "/docs/openapi.json",
		ScriptURL:   "/assets/scalar.js",
	})

	pageReq := httptest.NewRequest(http.MethodGet, "/docs", nil)
	pageResp := httptest.NewRecorder()
	engine.ServeHTTP(pageResp, pageReq)
	if pageResp.Code != http.StatusOK {
		t.Fatalf("expected docs page 200, got %d", pageResp.Code)
	}
	if !strings.Contains(pageResp.Body.String(), `src="/assets/scalar.js"`) {
		t.Fatalf("expected self-hosted scalar script, got %s", pageResp.Body.String())
	}
	if csp := pageResp.Header().Get("Content-Security-Policy"); strings.Contains(csp, "cdn.jsdelivr.net") {
		t.Fatalf("self-hosted CSP must not allow jsDelivr: %s", csp)
	}

	openapiReq := httptest.NewRequest(http.MethodGet, "/docs/openapi.json", nil)
	openapiResp := httptest.NewRecorder()
	engine.ServeHTTP(openapiResp, openapiReq)
	if openapiResp.Code != http.StatusOK {
		t.Fatalf("expected openapi 200, got %d", openapiResp.Code)
	}
	if !strings.Contains(openapiResp.Body.String(), `"operationId":"health"`) {
		t.Fatalf("unexpected openapi response: %s", openapiResp.Body.String())
	}
}
