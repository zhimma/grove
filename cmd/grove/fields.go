package main

import (
	"fmt"
	"regexp"
	"strings"
)

// field is one column of a generated module, parsed from `name:type[:required]`
// in the style of Rails and Phoenix scaffolds. Fields come from the command
// line rather than a live database so generation needs no connection and stays
// identical for both SQL dialects.
type field struct {
	Name     string // snake_case column and JSON name
	GoName   string // exported Go field name
	Type     string // one of fieldTypes
	Required bool
}

var fieldTypes = map[string]bool{
	"string": true,
	"text":   true,
	"int":    true,
	"bool":   true,
	"time":   true,
}

var fieldNamePattern = regexp.MustCompile(`^[a-z][a-z0-9]*(_[a-z0-9]+)*$`)

// reservedFieldNames are either supplied by model.Base or are SQL keywords that
// would need dialect-specific quoting, which the generated migrations avoid.
var reservedFieldNames = map[string]bool{
	"id": true, "created_at": true, "updated_at": true, "deleted_at": true,
	"order": true, "group": true, "key": true, "desc": true, "asc": true,
	"select": true, "from": true, "where": true, "table": true, "index": true,
	"limit": true, "offset": true, "user": true, "default": true, "check": true,
	"column": true, "primary": true, "references": true,
}

// defaultFields keeps `make:module Name` useful without --fields.
const defaultFields = "name:string:required"

func parseFields(spec string) ([]field, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		spec = defaultFields
	}

	var fields []field
	seen := map[string]bool{}
	goNames := map[string]bool{"Base": true, "TableName": true, "ID": true, "CreatedAt": true, "UpdatedAt": true, "DeletedAt": true}
	for raw := range strings.SplitSeq(spec, ",") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		parts := strings.Split(raw, ":")
		if len(parts) < 2 || len(parts) > 3 {
			return nil, fmt.Errorf("字段 %q 格式应为 name:type 或 name:type:required", raw)
		}
		name, kind := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		if !fieldNamePattern.MatchString(name) {
			return nil, fmt.Errorf("字段名 %q 必须是小写蛇形命名，例如 due_at", name)
		}
		if reservedFieldNames[name] {
			return nil, fmt.Errorf("字段名 %q 是保留字：id 与时间戳由框架提供，SQL 关键字需要方言相关的转义", name)
		}
		if seen[name] {
			return nil, fmt.Errorf("字段名 %q 重复", name)
		}
		goName := toPascal(name)
		if goNames[goName] {
			return nil, fmt.Errorf("字段 %q 转换后的 Go 名称 %q 与其他字段或模型成员冲突", name, goName)
		}
		if !fieldTypes[kind] {
			return nil, fmt.Errorf("字段 %q 的类型 %q 不支持，可选 string、text、int、bool、time", name, kind)
		}
		required := false
		if len(parts) == 3 {
			if modifier := strings.TrimSpace(parts[2]); modifier != "required" {
				return nil, fmt.Errorf("字段 %q 的修饰符 %q 不支持，只能是 required", name, modifier)
			}
			required = true
		}
		// validator's `required` means "not the zero value": a required bool
		// would reject false and a required int would reject 0, both valid.
		if required && (kind == "bool" || kind == "int") {
			return nil, fmt.Errorf("%s 字段 %q 不能标记 required：零值（false 或 0）本身是合法值，需要约束请在 service 中校验", kind, name)
		}
		seen[name] = true
		goNames[goName] = true
		fields = append(fields, field{Name: name, GoName: goName, Type: kind, Required: required})
	}
	if len(fields) == 0 {
		return nil, fmt.Errorf("至少需要一个字段")
	}
	return fields, nil
}

func (f field) IsString() bool { return f.Type == "string" || f.Type == "text" }
func (f field) IsTime() bool   { return f.Type == "time" }

// ModelType is the Go type on the model and in create inputs.
func (f field) ModelType() string {
	switch f.Type {
	case "int":
		return "int"
	case "bool":
		return "bool"
	case "time":
		return "*time.Time"
	default:
		return "string"
	}
}

// InputType is the create request and service input type. A time travels as
// a string in the same layout responses use, so a value read from the API can
// be sent back unchanged; the service parses it.
func (f field) InputType() string {
	if f.Type == "time" {
		return "string"
	}
	return f.ModelType()
}

// PatchType is the update input type: a pointer, so "not sent" differs from
// "set to the zero value".
func (f field) PatchType() string {
	return "*" + f.InputType()
}

// TimeVar names the parsed *time.Time local in generated create code.
func (f field) TimeVar() string {
	first, rest, _ := strings.Cut(toSnake(f.GoName), "_")
	return first + toPascal(rest) + "Time"
}

// TSType is the field's type in the generated frontend API module.
func (f field) TSType() string {
	switch f.Type {
	case "int":
		return "number"
	case "bool":
		return "boolean"
	default:
		return "string"
	}
}

// FormType is the resource-page form control for the field.
func (f field) FormType() string {
	switch f.Type {
	case "text":
		return "textarea"
	case "int":
		return "number"
	case "bool":
		return "switch"
	case "time":
		return "datetime"
	default:
		return ""
	}
}

func (f field) ResponseType() string {
	if f.Type == "time" {
		return "string"
	}
	return f.ModelType()
}

// ResponseTags omits an unset time so clients see an absent key, not "".
func (f field) ResponseTags() string {
	if f.IsTime() {
		return fmt.Sprintf(`json:"%s,omitempty"`, f.Name)
	}
	return fmt.Sprintf(`json:"%s"`, f.Name)
}

// ModelTags carries no GORM default: with one, GORM leaves a zero value out of
// the INSERT and the column default takes its place.
func (f field) ModelTags() string {
	switch f.Type {
	case "string":
		return fmt.Sprintf(`gorm:"size:255;not null" json:"%s"`, f.Name)
	case "text":
		return fmt.Sprintf(`gorm:"type:text;not null" json:"%s"`, f.Name)
	case "int", "bool":
		return fmt.Sprintf(`gorm:"not null" json:"%s"`, f.Name)
	default:
		return fmt.Sprintf(`json:"%s,omitempty"`, f.Name)
	}
}

func (f field) CreateBinding() string {
	var rules []string
	if f.Required {
		rules = append(rules, "required")
	} else {
		rules = append(rules, "omitempty")
	}
	if f.Type == "string" {
		rules = append(rules, "max=255")
	}
	return strings.Join(rules, ",")
}

func (f field) PatchBinding() string {
	if f.Type == "string" {
		return "omitempty,max=255"
	}
	return "omitempty"
}

func (f field) PostgresColumn() string {
	switch f.Type {
	case "string":
		return "VARCHAR(255) NOT NULL DEFAULT ''"
	case "text":
		return "TEXT NOT NULL DEFAULT ''"
	case "int":
		return "INTEGER NOT NULL DEFAULT 0"
	case "bool":
		return "BOOLEAN NOT NULL DEFAULT FALSE"
	default:
		return "TIMESTAMPTZ NULL"
	}
}

func (f field) MySQLColumn() string {
	switch f.Type {
	case "string":
		return "VARCHAR(255) NOT NULL DEFAULT ''"
	case "text":
		// MySQL rejects a literal default on TEXT columns.
		return "TEXT NOT NULL"
	case "int":
		return "INT NOT NULL DEFAULT 0"
	case "bool":
		return "TINYINT(1) NOT NULL DEFAULT 0"
	default:
		return "DATETIME(6) NULL"
	}
}

// SampleValue and UpdatedValue feed the generated service test.
func (f field) SampleValue() string {
	switch f.Type {
	case "string", "text":
		return fmt.Sprintf("%q", "sample "+f.Name)
	case "int":
		return "42"
	case "bool":
		return "true"
	default:
		return `"2026-10-01 10:00:00"`
	}
}

func (f field) UpdatedValue() string {
	switch f.Type {
	case "string", "text":
		return fmt.Sprintf("%q", "updated "+f.Name)
	case "int":
		return "7"
	case "bool":
		return "false"
	default:
		return `"2026-10-02 11:30:00"`
	}
}
