package main

import (
	"bytes"
	"fmt"
	"go/format"
	"strings"
	"text/template"
	"unicode"
)

// moduleSpec is everything a generated vertical slice is rendered from.
type moduleSpec struct {
	Module    string // Go module path read from the target repo's go.mod
	Name      string // Invoice
	Plural    string // Invoices
	Var       string // invoice
	Snake     string // invoice
	Kebab     string // invoice, the frontend API file name
	Table     string // invoices
	RoutePath string // /invoices
	Label     string // display name used for permissions and OpenAPI tags
	Fields    []field
}

func newModuleSpec(module, input, label string, fields []field) (moduleSpec, error) {
	name := toPascal(input)
	snake := toSnake(input)
	if !isValidGoIdentifier(name) || snake == "" {
		return moduleSpec{}, fmt.Errorf("模块名称 %q 不能转换为合法 Go 标识符", input)
	}
	label = strings.TrimSpace(label)
	if label == "" {
		label = name
	}
	// Permission names split on "." into module and action.
	if strings.Contains(label, ".") {
		return moduleSpec{}, fmt.Errorf("模块显示名 %q 不能包含 \".\"：权限名以它分隔模块与动作", label)
	}
	// The label is written verbatim into Go, TypeScript and Vue string literals.
	if strings.ContainsAny(label, "\"'`\\<>") {
		return moduleSpec{}, fmt.Errorf("模块显示名 %q 不能包含引号、反斜杠或尖括号", label)
	}
	runes := []rune(name)
	runes[0] = unicode.ToLower(runes[0])
	return moduleSpec{
		Module:    module,
		Name:      name,
		Plural:    toPascal(toSnakePlural(input)),
		Var:       string(runes),
		Snake:     snake,
		Kebab:     strings.ReplaceAll(snake, "_", "-"),
		Table:     toSnakePlural(input),
		RoutePath: "/" + toKebabPlural(input),
		Label:     label,
		Fields:    fields,
	}, nil
}

func (m moduleSpec) HasTime() bool {
	for _, f := range m.Fields {
		if f.IsTime() {
			return true
		}
	}
	return false
}

// SearchFields are the columns the list keyword matches. TEXT is left out:
// an unindexed LIKE over large bodies is the slow query nobody asked for.
func (m moduleSpec) SearchFields() []field {
	var result []field
	for _, f := range m.Fields {
		if f.Type == "string" {
			result = append(result, f)
		}
	}
	return result
}

func (m moduleSpec) SearchWhere() string {
	clauses := make([]string, 0, len(m.Fields))
	for _, f := range m.SearchFields() {
		clauses = append(clauses, f.Name+" LIKE ?")
	}
	return strings.Join(clauses, " OR ")
}

// ColumnFields are the fields worth a list column and an ORDER BY: TEXT is
// neither readable in a table cell nor cheap to sort.
func (m moduleSpec) ColumnFields() []field {
	var result []field
	for _, f := range m.Fields {
		if f.Type != "text" {
			result = append(result, f)
		}
	}
	return result
}

// SortCases is the ORDER BY whitelist, rendered as a switch case list.
func (m moduleSpec) SortCases() string {
	columns := make([]string, 0, len(m.Fields)+2)
	for _, f := range m.ColumnFields() {
		columns = append(columns, fmt.Sprintf("%q", f.Name))
	}
	columns = append(columns, `"created_at"`, `"updated_at"`)
	return strings.Join(columns, ", ")
}

func (m moduleSpec) FirstField() field { return m.Fields[0] }

func (m moduleSpec) FirstRequiredString() *field {
	for _, f := range m.Fields {
		if f.Required && f.IsString() {
			return &f
		}
	}
	return nil
}

var templateFuncs = template.FuncMap{
	// Go raw strings cannot hold a backtick, so struct tags are wrapped here.
	"tag": func(s string) string { return "`" + s + "`" },
}

// renderGo executes a template and gofmts the result, so a template mistake
// fails generation instead of writing a file that does not parse.
func renderGo(name, text string, spec moduleSpec) ([]byte, error) {
	rendered, err := render(name, text, spec)
	if err != nil {
		return nil, err
	}
	formatted, err := format.Source(rendered)
	if err != nil {
		return nil, fmt.Errorf("格式化生成的 %s: %w\n%s", name, err, rendered)
	}
	return formatted, nil
}

func render(name, text string, spec moduleSpec) ([]byte, error) {
	tpl, err := template.New(name).Funcs(templateFuncs).Parse(text)
	if err != nil {
		return nil, fmt.Errorf("解析模板 %s: %w", name, err)
	}
	var out bytes.Buffer
	if err := tpl.Execute(&out, spec); err != nil {
		return nil, fmt.Errorf("渲染模板 %s: %w", name, err)
	}
	return out.Bytes(), nil
}

const modelTemplate = `package model
{{if .HasTime}}
import "time"
{{end}}
type {{.Name}} struct {
	Base
{{- range .Fields}}
	{{.GoName}} {{.ModelType}} {{tag .ModelTags}}
{{- end}}
}

func ({{.Name}}) TableName() string {
	return "{{.Table}}"
}
`

const serviceTemplate = `package service

import (
	"context"
	"errors"
	"strings"
{{- if .HasTime}}
	"time"
{{- end}}

	"gorm.io/gorm"

	"{{.Module}}/internal/model"
	"{{.Module}}/pkg/database"
	"{{.Module}}/pkg/errx"
)

type {{.Name}}Service struct {
	dbs        database.Connections
	pagePolicy PagePolicy
}

type List{{.Plural}}Input struct {
	Page        int
	PageSize    int
	Offset      int
	Limit       int
	ListAll     bool
	Keyword     string
	OrderBy     []string
	CreatedFrom string
	CreatedTo   string
}

type List{{.Plural}}Output struct {
	List []model.{{.Name}}
	Meta ListMeta
}

type Create{{.Name}}Input struct {
{{- range .Fields}}
	{{.GoName}} {{.InputType}}
{{- end}}
}

// Update{{.Name}}Input uses pointers so "not sent" differs from "set to zero".
type Update{{.Name}}Input struct {
	{{.Name}}ID string
{{- range .Fields}}
	{{.GoName}} {{.PatchType}}
{{- end}}
}

func New{{.Name}}Service(dbs database.Connections, policies ...PagePolicy) *{{.Name}}Service {
	return &{{.Name}}Service{dbs: dbs, pagePolicy: pagePolicyFromArgs(policies)}
}

func (s *{{.Name}}Service) List{{.Plural}}(ctx context.Context, in List{{.Plural}}Input) (*List{{.Plural}}Output, error) {
	db, err := s.defaultDB()
	if err != nil {
		return nil, err
	}
	page, pageSize := resolvePageWithPolicy(ListRequest{
		Page: in.Page, PageSize: in.PageSize, Offset: in.Offset, Limit: in.Limit, ListAll: in.ListAll,
	}, s.pagePolicy)

	query := db.WithContext(ctx).Model(&model.{{.Name}}{})
{{- if .SearchFields}}
	if keyword := strings.TrimSpace(in.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("{{.SearchWhere}}"{{range .SearchFields}}, like{{end}})
	}
{{- end}}
	query, err = applyTimeRange(query, "created_at", in.CreatedFrom, in.CreatedTo)
	if err != nil {
		return nil, errx.InvalidParams().WithHTTPStatus(422).WithMessage("时间范围格式不正确")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, errx.Internal().WithCause(err)
	}
	if len(in.OrderBy) == 0 {
		query = query.Order("created_at DESC")
	}
	for _, item := range in.OrderBy {
		// Only whitelisted columns reach ORDER BY; anything else is ignored.
		switch column, direction := parseOrderBy(item); column {
		case {{.SortCases}}:
			query = query.Order(column + " " + direction)
		}
	}
	if !in.ListAll {
		offset := in.Offset
		if offset <= 0 {
			offset = (page - 1) * pageSize
		}
		query = query.Offset(offset).Limit(pageSize)
	}

	list := make([]model.{{.Name}}, 0)
	if err := query.Find(&list).Error; err != nil {
		return nil, errx.Internal().WithCause(err)
	}
	return &List{{.Plural}}Output{List: list, Meta: NewListMeta(total, page, pageSize)}, nil
}

func (s *{{.Name}}Service) Get{{.Name}}(ctx context.Context, id string) (*model.{{.Name}}, error) {
	return s.load{{.Name}}(ctx, id)
}

func (s *{{.Name}}Service) Create{{.Name}}(ctx context.Context, in Create{{.Name}}Input) (*model.{{.Name}}, error) {
	db, err := s.defaultDB()
	if err != nil {
		return nil, err
	}
{{- range .Fields}}{{if .IsTime}}
	var {{.TimeVar}} *time.Time
	if value := strings.TrimSpace(in.{{.GoName}}); value != "" {
		parsed, err := parseTimeValue(value, false)
		if err != nil {
			return nil, invalid{{$.Name}}Params("{{.Name}} 时间格式不正确")
		}
		{{.TimeVar}} = &parsed
	}
{{- end}}{{end}}
	item := &model.{{.Name}}{
{{- range .Fields}}
		{{.GoName}}: {{if .IsTime}}{{.TimeVar}}{{else if .IsString}}strings.TrimSpace(in.{{.GoName}}){{else}}in.{{.GoName}}{{end}},
{{- end}}
	}
{{- range .Fields}}{{if .Required}}
	if item.{{.GoName}} == {{if .IsString}}""{{else}}nil{{end}} {
		return nil, invalid{{$.Name}}Params("{{.Name}} 不能为空")
	}
{{- end}}{{end}}
	if err := db.WithContext(ctx).Create(item).Error; err != nil {
		return nil, errx.Internal().WithCause(err)
	}
	return s.load{{.Name}}(ctx, item.ID)
}

func (s *{{.Name}}Service) Update{{.Name}}(ctx context.Context, in Update{{.Name}}Input) (*model.{{.Name}}, error) {
	item, err := s.load{{.Name}}(ctx, in.{{.Name}}ID)
	if err != nil {
		return nil, err
	}
	updates := make(map[string]any)
{{- range .Fields}}
	if in.{{.GoName}} != nil {
	{{- if and .IsString .Required}}
		value := strings.TrimSpace(*in.{{.GoName}})
		if value == "" {
			return nil, invalid{{$.Name}}Params("{{.Name}} 不能为空")
		}
		updates["{{.Name}}"] = value
	{{- else if .IsString}}
		updates["{{.Name}}"] = strings.TrimSpace(*in.{{.GoName}})
	{{- else if .IsTime}}
		value := strings.TrimSpace(*in.{{.GoName}})
		if value == "" {
		{{- if .Required}}
			return nil, invalid{{$.Name}}Params("{{.Name}} 不能为空")
		{{- else}}
			updates["{{.Name}}"] = nil
		{{- end}}
		} else {
			parsed, err := parseTimeValue(value, false)
			if err != nil {
				return nil, invalid{{$.Name}}Params("{{.Name}} 时间格式不正确")
			}
			updates["{{.Name}}"] = parsed
		}
	{{- else}}
		updates["{{.Name}}"] = *in.{{.GoName}}
	{{- end}}
	}
{{- end}}
	if len(updates) > 0 {
		db, dbErr := s.defaultDB()
		if dbErr != nil {
			return nil, dbErr
		}
		if err := db.WithContext(ctx).Model(&model.{{.Name}}{}).Where("id = ?", item.ID).Updates(updates).Error; err != nil {
			return nil, errx.Internal().WithCause(err)
		}
	}
	return s.load{{.Name}}(ctx, item.ID)
}

func (s *{{.Name}}Service) Delete{{.Name}}(ctx context.Context, id string) error {
	item, err := s.load{{.Name}}(ctx, id)
	if err != nil {
		return err
	}
	db, err := s.defaultDB()
	if err != nil {
		return err
	}
	if err := db.WithContext(ctx).Delete(&model.{{.Name}}{}, "id = ?", item.ID).Error; err != nil {
		return errx.Internal().WithCause(err)
	}
	return nil
}

func (s *{{.Name}}Service) load{{.Name}}(ctx context.Context, id string) (*model.{{.Name}}, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, invalid{{.Name}}Params("ID 不能为空")
	}
	db, err := s.defaultDB()
	if err != nil {
		return nil, err
	}
	var item model.{{.Name}}
	if err := db.WithContext(ctx).Where("id = ?", id).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errx.NotFound().WithMessage("{{.Label}}不存在")
		}
		return nil, errx.Internal().WithCause(err)
	}
	return &item, nil
}

func (s *{{.Name}}Service) defaultDB() (*gorm.DB, error) {
	if s == nil || s.dbs == nil || s.dbs.Default() == nil {
		return nil, errx.ServiceUnavailable().WithMessage("默认数据库未配置")
	}
	return s.dbs.Default(), nil
}

func invalid{{.Name}}Params(message string) error {
	return errx.InvalidParams().WithHTTPStatus(422).WithMessage(message)
}
`

// serviceTestTemplate ships a real CRUD test with every module, the way
// Phoenix generators do, so a generated module starts out covered.
const serviceTestTemplate = `package service

import (
	"context"
	"testing"
{{- if .FirstField.IsTime}}
	"time"
{{- end}}

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"{{.Module}}/internal/model"
	"{{.Module}}/pkg/database"
)

func Test{{.Name}}ServiceCRUD(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/{{.Snake}}.db"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.{{.Name}}{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	svc := New{{.Name}}Service(database.NewConnectionsFromDBs(db, nil))
	ctx := context.Background()
{{- with .FirstRequiredString}}

	if _, err := svc.Create{{$.Name}}(ctx, Create{{$.Name}}Input{}); err == nil {
		t.Fatal("create without the required {{.Name}} must fail")
	}
{{- end}}

	created, err := svc.Create{{.Name}}(ctx, Create{{.Name}}Input{
{{- range .Fields}}
		{{.GoName}}: {{.SampleValue}},
{{- end}}
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := svc.Get{{.Name}}(ctx, created.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != created.ID {
		t.Fatalf("get returned %s, want %s", got.ID, created.ID)
	}

	list, err := svc.List{{.Plural}}(ctx, List{{.Plural}}Input{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if list.Meta.Total != 1 || len(list.List) != 1 {
		t.Fatalf("list total = %d, items = %d, want 1", list.Meta.Total, len(list.List))
	}
{{with .FirstField}}
	updatedValue := {{.UpdatedValue}}
	updated, err := svc.Update{{$.Name}}(ctx, Update{{$.Name}}Input{ {{- $.Name}}ID: created.ID, {{.GoName}}: &updatedValue})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
{{- if .IsTime}}
	want, err := time.ParseInLocation("2006-01-02 15:04:05", updatedValue, time.Local)
	if err != nil {
		t.Fatalf("parse expected time: %v", err)
	}
	if updated.{{.GoName}} == nil || !updated.{{.GoName}}.Equal(want) {
{{- else}}
	if updated.{{.GoName}} != updatedValue {
{{- end}}
		t.Fatalf("update did not persist {{.Name}}: %v", updated.{{.GoName}})
	}
{{end}}
	if err := svc.Delete{{.Name}}(ctx, created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := svc.Get{{.Name}}(ctx, created.ID); err == nil {
		t.Fatal("a deleted {{.Snake}} must not be found")
	}
}
`

const handlerTemplate = `package handler

import (
	"github.com/gin-gonic/gin"

	consoleservice "{{.Module}}/app/console/internal/service"
	"{{.Module}}/pkg/database"
	"{{.Module}}/pkg/response"
	"{{.Module}}/pkg/route"
	"{{.Module}}/pkg/validation"
)

type {{.Name}}Handler struct {
	{{.Var}}Svc *consoleservice.{{.Name}}Service
}

type List{{.Plural}}Request struct {
	ListQuery
}

type List{{.Plural}}Response struct {
	List []{{.Name}}Response {{tag "json:\"list\""}}
	Meta ListMeta {{tag "json:\"meta\""}}
}

type Create{{.Name}}Request struct {
{{- range .Fields}}
	{{.GoName}} {{.InputType}} {{tag (printf "json:%q binding:%q label:%q" .Name .CreateBinding .Name)}}
{{- end}}
}

type Update{{.Name}}Request struct {
{{- range .Fields}}
	{{.GoName}} {{.PatchType}} {{tag (printf "json:%q binding:%q label:%q" .Name .PatchBinding .Name)}}
{{- end}}
}

type {{.Name}}PathRequest struct {
	ID string {{tag "uri:\"id\" binding:\"required\" label:\"ID\""}}
}

func Register{{.Name}}Routes(protected *gin.RouterGroup, dbs database.Connections, policies []consoleservice.PagePolicy, catalog *route.Catalog) {
	h := &{{.Name}}Handler{ {{- .Var}}Svc: consoleservice.New{{.Name}}Service(dbs, policies...)}
	group := wrapRoute(protected.Group("{{.RoutePath}}"), catalog)
	group.GET("", h.List).Name("{{.Label}}.列表")
	group.GET("/:id", h.Detail).Name("{{.Label}}.详情")
	group.POST("", h.Create).Name("{{.Label}}.创建")
	group.PUT("/:id", h.Update).Name("{{.Label}}.更新")
	group.DELETE("/:id", h.Delete).Name("{{.Label}}.删除")
}

func (h *{{.Name}}Handler) List(c *gin.Context) {
	var req List{{.Plural}}Request
	if err := validation.BindQuery(c, &req); err != nil {
		response.Fail(c, err)
		return
	}
	result, err := h.{{.Var}}Svc.List{{.Plural}}(c.Request.Context(), consoleservice.List{{.Plural}}Input{
		Page: req.Page, PageSize: req.PageSize, Offset: req.Offset, Limit: req.Limit, ListAll: req.ListAll,
		Keyword: req.Keyword, OrderBy: req.OrderBy, CreatedFrom: req.CreatedFrom, CreatedTo: req.CreatedTo,
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	items := make([]{{.Name}}Response, 0, len(result.List))
	for i := range result.List {
		items = append(items, new{{.Name}}Response(&result.List[i]))
	}
	response.Success(c, List{{.Plural}}Response{List: items, Meta: ListMeta(result.Meta)})
}

func (h *{{.Name}}Handler) Detail(c *gin.Context) {
	var req {{.Name}}PathRequest
	if err := validation.BindURI(c, &req); err != nil {
		response.Fail(c, err)
		return
	}
	item, err := h.{{.Var}}Svc.Get{{.Name}}(c.Request.Context(), req.ID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, new{{.Name}}Response(item))
}

func (h *{{.Name}}Handler) Create(c *gin.Context) {
	var req Create{{.Name}}Request
	if err := validation.BindJSON(c, &req); err != nil {
		response.Fail(c, err)
		return
	}
	item, err := h.{{.Var}}Svc.Create{{.Name}}(c.Request.Context(), consoleservice.Create{{.Name}}Input{
{{- range .Fields}}
		{{.GoName}}: req.{{.GoName}},
{{- end}}
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	setAuditMeta(c, "{{.Snake}}", item.ID, nil)
	response.Success(c, new{{.Name}}Response(item))
}

func (h *{{.Name}}Handler) Update(c *gin.Context) {
	var pathReq {{.Name}}PathRequest
	if err := validation.BindURI(c, &pathReq); err != nil {
		response.Fail(c, err)
		return
	}
	var req Update{{.Name}}Request
	if err := validation.BindJSON(c, &req); err != nil {
		response.Fail(c, err)
		return
	}
	item, err := h.{{.Var}}Svc.Update{{.Name}}(c.Request.Context(), consoleservice.Update{{.Name}}Input{
		{{.Name}}ID: pathReq.ID,
{{- range .Fields}}
		{{.GoName}}: req.{{.GoName}},
{{- end}}
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	setAuditMeta(c, "{{.Snake}}", item.ID, nil)
	response.Success(c, new{{.Name}}Response(item))
}

func (h *{{.Name}}Handler) Delete(c *gin.Context) {
	var req {{.Name}}PathRequest
	if err := validation.BindURI(c, &req); err != nil {
		response.Fail(c, err)
		return
	}
	if err := h.{{.Var}}Svc.Delete{{.Name}}(c.Request.Context(), req.ID); err != nil {
		response.Fail(c, err)
		return
	}
	setAuditMeta(c, "{{.Snake}}", req.ID, map[string]any{"deleted": true})
	response.Success(c, nil)
}
`

const responseTemplate = `package handler

import "{{.Module}}/internal/model"

type {{.Name}}Response struct {
	ID string {{tag "json:\"id\""}}
{{- range .Fields}}
	{{.GoName}} {{.ResponseType}} {{tag .ResponseTags}}
{{- end}}
	CreatedAt string {{tag "json:\"created_at\""}}
	UpdatedAt string {{tag "json:\"updated_at\""}}
}

func new{{.Name}}Response(item *model.{{.Name}}) {{.Name}}Response {
	if item == nil {
		return {{.Name}}Response{}
	}
	result := {{.Name}}Response{
		ID: item.ID,
{{- range .Fields}}{{if not .IsTime}}
		{{.GoName}}: item.{{.GoName}},
{{- end}}{{end}}
		CreatedAt: item.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt: item.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
{{- range .Fields}}{{if .IsTime}}
	if item.{{.GoName}} != nil {
		result.{{.GoName}} = item.{{.GoName}}.Format("2006-01-02 15:04:05")
	}
{{- end}}{{end}}
	return result
}
`

const docsTemplate = `package docs

import (
	"net/http"

	"{{.Module}}/app/console/internal/handler"
	"{{.Module}}/internal/docsui"
)

func add{{.Name}}Operations(doc *docsui.Document) {
	path := docsui.ParametersFor(handler.{{.Name}}PathRequest{}, "uri", "path")
	listResponse := doc.AddSchema("ConsoleList{{.Plural}}Response", handler.List{{.Plural}}Response{})
	itemResponse := doc.AddSchema("Console{{.Name}}Response", handler.{{.Name}}Response{})
	createRequest := doc.AddSchema("ConsoleCreate{{.Name}}Request", handler.Create{{.Name}}Request{})
	updateRequest := doc.AddSchema("ConsoleUpdate{{.Name}}Request", handler.Update{{.Name}}Request{})

	addConsoleOperation(doc, "{{.Label}}", "{{.RoutePath}}", http.MethodGet, "consoleList{{.Plural}}", "获取{{.Label}}列表", true,
		docsui.ParametersFor(handler.List{{.Plural}}Request{}, "form", "query"), nil, listResponse)
	addConsoleOperation(doc, "{{.Label}}", "{{.RoutePath}}", http.MethodPost, "consoleCreate{{.Name}}", "创建{{.Label}}", true,
		nil, docsui.JSONBody("{{.Label}}", createRequest, true), itemResponse)
	addConsoleOperation(doc, "{{.Label}}", "{{.RoutePath}}/{id}", http.MethodGet, "consoleGet{{.Name}}", "获取{{.Label}}详情", true,
		path, nil, itemResponse)
	addConsoleOperation(doc, "{{.Label}}", "{{.RoutePath}}/{id}", http.MethodPut, "consoleUpdate{{.Name}}", "更新{{.Label}}", true,
		path, docsui.JSONBody("{{.Label}}", updateRequest, true), itemResponse)
	addConsoleOperation(doc, "{{.Label}}", "{{.RoutePath}}/{id}", http.MethodDelete, "consoleDelete{{.Name}}", "删除{{.Label}}", true,
		path, nil, docsui.Schema{})
}
`

const postgresUpTemplate = `CREATE TABLE IF NOT EXISTS {{.Table}} (
    id VARCHAR(26) PRIMARY KEY,
{{- range .Fields}}
    {{.Name}} {{.PostgresColumn}},
{{- end}}
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ NULL
);

CREATE INDEX IF NOT EXISTS idx_{{.Table}}_deleted_at ON {{.Table}}(deleted_at);
`

const mysqlUpTemplate = `CREATE TABLE IF NOT EXISTS {{.Table}} (
    id VARCHAR(26) PRIMARY KEY,
{{- range .Fields}}
    {{.Name}} {{.MySQLColumn}},
{{- end}}
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    deleted_at DATETIME(6) NULL,
    INDEX idx_{{.Table}}_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
`

const dropTableTemplate = "DROP TABLE IF EXISTS {{.Table}};\n"
