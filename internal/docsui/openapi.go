package docsui

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
)

const OpenAPIVersion = "3.0.3"

type Document struct {
	OpenAPI    string     `json:"openapi"`
	Info       Info       `json:"info"`
	Servers    []Server   `json:"servers,omitempty"`
	Paths      Paths      `json:"paths"`
	Components Components `json:"components,omitempty"`
}

type Info struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Version     string `json:"version"`
}

type Server struct {
	URL string `json:"url"`
}

type Paths map[string]PathItem

type PathItem map[string]Operation

type Operation struct {
	Method      string                `json:"-"`
	ID          string                `json:"operationId"`
	Summary     string                `json:"summary"`
	Tags        []string              `json:"tags,omitempty"`
	BearerAuth  bool                  `json:"-"`
	Parameters  []Parameter           `json:"parameters,omitempty"`
	RequestBody *RequestBody          `json:"requestBody,omitempty"`
	Responses   map[string]Response   `json:"responses"`
	Security    []SecurityRequirement `json:"security,omitempty"`
}

type SecurityRequirement map[string][]string

type Parameter struct {
	Name        string `json:"name"`
	In          string `json:"in"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
	Schema      Schema `json:"schema"`
}

type RequestBody struct {
	Description string               `json:"description,omitempty"`
	Required    bool                 `json:"required,omitempty"`
	Content     map[string]MediaType `json:"content"`
}

type Response struct {
	Description string               `json:"description"`
	Content     map[string]MediaType `json:"content,omitempty"`
}

type MediaType struct {
	Schema Schema `json:"schema"`
}

type Components struct {
	Schemas         map[string]Schema         `json:"schemas,omitempty"`
	SecuritySchemes map[string]SecurityScheme `json:"securitySchemes,omitempty"`
}

type SecurityScheme struct {
	Type         string `json:"type"`
	Scheme       string `json:"scheme,omitempty"`
	BearerFormat string `json:"bearerFormat,omitempty"`
}

type Schema struct {
	Ref                  string            `json:"$ref,omitempty"`
	Type                 string            `json:"type,omitempty"`
	Format               string            `json:"format,omitempty"`
	Description          string            `json:"description,omitempty"`
	Properties           map[string]Schema `json:"properties,omitempty"`
	Items                *Schema           `json:"items,omitempty"`
	Required             []string          `json:"required,omitempty"`
	Enum                 []any             `json:"enum,omitempty"`
	AdditionalProperties any               `json:"additionalProperties,omitempty"`
	Minimum              *float64          `json:"minimum,omitempty"`
	Maximum              *float64          `json:"maximum,omitempty"`
	MinLength            *int              `json:"minLength,omitempty"`
	MaxLength            *int              `json:"maxLength,omitempty"`
}

func NewDocument(title, description, version string, serverURLs ...string) Document {
	servers := make([]Server, 0, len(serverURLs))
	for _, serverURL := range serverURLs {
		if serverURL = strings.TrimSpace(serverURL); serverURL != "" {
			servers = append(servers, Server{URL: serverURL})
		}
	}

	return Document{
		OpenAPI: OpenAPIVersion,
		Info: Info{
			Title:       strings.TrimSpace(title),
			Description: strings.TrimSpace(description),
			Version:     strings.TrimSpace(version),
		},
		Servers: servers,
		Paths:   Paths{},
		Components: Components{
			Schemas: map[string]Schema{},
			SecuritySchemes: map[string]SecurityScheme{
				"BearerAuth": {
					Type:         "http",
					Scheme:       "bearer",
					BearerFormat: "JWT",
				},
			},
		},
	}
}

func (d *Document) AddSchema(name string, value any) Schema {
	name = strings.TrimSpace(name)
	if name == "" {
		return SchemaFor(value)
	}
	if d.Components.Schemas == nil {
		d.Components.Schemas = map[string]Schema{}
	}
	d.Components.Schemas[name] = SchemaFor(value)
	return SchemaRef(name)
}

func (d *Document) Add(path string, operation Operation) {
	if d.Paths == nil {
		d.Paths = Paths{}
	}
	path = normalizeOpenAPIPath(path)
	method := strings.ToLower(strings.TrimSpace(operation.Method))
	if path == "" || method == "" {
		return
	}
	operation.Method = ""
	operation.ID = strings.TrimSpace(operation.ID)
	operation.Summary = strings.TrimSpace(operation.Summary)
	if operation.BearerAuth && len(operation.Security) == 0 {
		operation.Security = []SecurityRequirement{{"BearerAuth": {}}}
	}
	operation.BearerAuth = false
	if operation.Responses == nil {
		operation.Responses = StandardResponses("请求成功", Schema{})
	}
	if d.Paths[path] == nil {
		d.Paths[path] = PathItem{}
	}
	d.Paths[path][method] = operation
}

func (d Document) Validate() error {
	var problems []string
	if strings.TrimSpace(d.OpenAPI) == "" {
		problems = append(problems, "missing openapi version")
	}
	if strings.TrimSpace(d.Info.Title) == "" {
		problems = append(problems, "missing document title")
	}
	operationIDs := map[string]string{}
	for path, item := range d.Paths {
		if normalizeOpenAPIPath(path) != path {
			problems = append(problems, fmt.Sprintf("path %q is not normalized", path))
		}
		for method, operation := range item {
			method = strings.ToUpper(strings.TrimSpace(method))
			if !isDocumentedMethod(method) {
				problems = append(problems, fmt.Sprintf("unsupported method %q on %s", method, path))
			}
			if operation.ID == "" {
				problems = append(problems, fmt.Sprintf("missing operationId for %s %s", method, path))
			} else if previous, exists := operationIDs[operation.ID]; exists {
				problems = append(problems, fmt.Sprintf("duplicate operationId %q on %s %s and %s", operation.ID, method, path, previous))
			} else {
				operationIDs[operation.ID] = method + " " + path
			}
			if len(operation.Responses) == 0 {
				problems = append(problems, fmt.Sprintf("missing responses for %s %s", method, path))
			}
		}
	}
	if len(problems) == 0 {
		return nil
	}
	sort.Strings(problems)
	return fmt.Errorf("invalid OpenAPI document: %s", strings.Join(problems, "; "))
}

func SchemaRef(name string) Schema {
	return Schema{Ref: "#/components/schemas/" + strings.TrimSpace(name)}
}

func JSONBody(description string, schema Schema, required bool) *RequestBody {
	return &RequestBody{
		Description: strings.TrimSpace(description),
		Required:    required,
		Content: map[string]MediaType{
			"application/json": {Schema: schema},
		},
	}
}

func MultipartBody(description string, schema Schema, required bool) *RequestBody {
	return &RequestBody{
		Description: strings.TrimSpace(description),
		Required:    required,
		Content: map[string]MediaType{
			"multipart/form-data": {Schema: schema},
		},
	}
}

func StandardResponses(description string, data Schema) map[string]Response {
	return map[string]Response{
		"200": {
			Description: strings.TrimSpace(description),
			Content: map[string]MediaType{
				"application/json": {Schema: successEnvelope(data)},
			},
		},
		"default": {
			Description: "请求失败",
			Content: map[string]MediaType{
				"application/json": {Schema: errorEnvelope()},
			},
		},
	}
}

func successEnvelope(data Schema) Schema {
	properties := map[string]Schema{
		"code":       {Type: "integer", Format: "int32"},
		"message":    {Type: "string"},
		"request_id": {Type: "string"},
	}
	if !isEmptySchema(data) {
		properties["data"] = data
	}
	return Schema{
		Type:       "object",
		Properties: properties,
		Required:   []string{"code", "message"},
	}
}

func isEmptySchema(schema Schema) bool {
	return schema.Ref == "" &&
		schema.Type == "" &&
		schema.Format == "" &&
		schema.Description == "" &&
		len(schema.Properties) == 0 &&
		schema.Items == nil &&
		len(schema.Required) == 0 &&
		len(schema.Enum) == 0 &&
		schema.AdditionalProperties == nil &&
		schema.Minimum == nil &&
		schema.Maximum == nil &&
		schema.MinLength == nil &&
		schema.MaxLength == nil
}

func errorEnvelope() Schema {
	return Schema{
		Type: "object",
		Properties: map[string]Schema{
			"code":       {Type: "integer", Format: "int32"},
			"message":    {Type: "string"},
			"data":       {Type: "object", AdditionalProperties: true},
			"request_id": {Type: "string"},
		},
		Required: []string{"code", "message"},
	}
}

func normalizeOpenAPIPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if path != "/" {
		path = strings.TrimRight(path, "/")
	}
	return path
}

func isDocumentedMethod(method string) bool {
	switch strings.ToUpper(method) {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}
