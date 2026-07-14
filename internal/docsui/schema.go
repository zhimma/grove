package docsui

import (
	"reflect"
	"strconv"
	"strings"
	"time"
)

var timeType = reflect.TypeOf(time.Time{})

func SchemaFor(value any) Schema {
	if value == nil {
		return Schema{}
	}
	return schemaForType(reflect.TypeOf(value), map[reflect.Type]bool{})
}

func ParametersFor(value any, tagName, location string) []Parameter {
	if value == nil {
		return nil
	}
	typeOf := reflect.TypeOf(value)
	for typeOf.Kind() == reflect.Pointer {
		typeOf = typeOf.Elem()
	}
	if typeOf.Kind() != reflect.Struct {
		return nil
	}

	parameters := make([]Parameter, 0)
	appendParameters(&parameters, typeOf, strings.TrimSpace(tagName), strings.TrimSpace(location))
	return parameters
}

func schemaForType(typeOf reflect.Type, visiting map[reflect.Type]bool) Schema {
	for typeOf.Kind() == reflect.Pointer {
		typeOf = typeOf.Elem()
	}
	if typeOf == timeType {
		return Schema{Type: "string", Format: "date-time"}
	}

	switch typeOf.Kind() {
	case reflect.Bool:
		return Schema{Type: "boolean"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32:
		return Schema{Type: "integer", Format: "int32"}
	case reflect.Int64:
		return Schema{Type: "integer", Format: "int64"}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32:
		return Schema{Type: "integer", Format: "int32"}
	case reflect.Uint64:
		return Schema{Type: "integer", Format: "int64"}
	case reflect.Float32:
		return Schema{Type: "number", Format: "float"}
	case reflect.Float64:
		return Schema{Type: "number", Format: "double"}
	case reflect.String:
		return Schema{Type: "string"}
	case reflect.Slice, reflect.Array:
		item := schemaForType(typeOf.Elem(), visiting)
		return Schema{Type: "array", Items: &item}
	case reflect.Map:
		if typeOf.Key().Kind() != reflect.String {
			return Schema{Type: "object", AdditionalProperties: true}
		}
		if typeOf.Elem().Kind() == reflect.Interface {
			return Schema{Type: "object", AdditionalProperties: true}
		}
		valueSchema := schemaForType(typeOf.Elem(), visiting)
		if isEmptySchema(valueSchema) {
			return Schema{Type: "object", AdditionalProperties: true}
		}
		return Schema{Type: "object", AdditionalProperties: valueSchema}
	case reflect.Interface:
		return Schema{}
	case reflect.Struct:
		if visiting[typeOf] {
			return Schema{Type: "object"}
		}
		visiting[typeOf] = true
		defer delete(visiting, typeOf)

		schema := Schema{Type: "object", Properties: map[string]Schema{}}
		for i := 0; i < typeOf.NumField(); i++ {
			field := typeOf.Field(i)
			if !field.IsExported() {
				continue
			}
			name, options := fieldName(field, "json")
			if name == "-" {
				continue
			}
			if field.Anonymous && name == "" {
				embedded := schemaForType(field.Type, visiting)
				for propertyName, property := range embedded.Properties {
					schema.Properties[propertyName] = property
				}
				schema.Required = append(schema.Required, embedded.Required...)
				continue
			}
			if name == "" {
				name = field.Name
			}
			property := schemaForType(field.Type, visiting)
			property.Description = strings.TrimSpace(field.Tag.Get("label"))
			applyBinding(&property, field.Tag.Get("binding"))
			schema.Properties[name] = property
			if hasBindingRule(field.Tag.Get("binding"), "required") && !options["omitempty"] {
				schema.Required = append(schema.Required, name)
			}
		}
		if len(schema.Properties) == 0 {
			schema.Properties = nil
		}
		return schema
	default:
		return Schema{}
	}
}

func appendParameters(parameters *[]Parameter, typeOf reflect.Type, tagName, location string) {
	for i := 0; i < typeOf.NumField(); i++ {
		field := typeOf.Field(i)
		if !field.IsExported() {
			continue
		}
		name, _ := fieldName(field, tagName)
		if field.Anonymous && name == "" {
			embedded := field.Type
			for embedded.Kind() == reflect.Pointer {
				embedded = embedded.Elem()
			}
			if embedded.Kind() == reflect.Struct {
				appendParameters(parameters, embedded, tagName, location)
			}
			continue
		}
		if name == "" || name == "-" {
			continue
		}
		schema := schemaForType(field.Type, map[reflect.Type]bool{})
		applyBinding(&schema, field.Tag.Get("binding"))
		*parameters = append(*parameters, Parameter{
			Name:        name,
			In:          location,
			Description: strings.TrimSpace(field.Tag.Get("label")),
			Required:    location == "path" || hasBindingRule(field.Tag.Get("binding"), "required"),
			Schema:      schema,
		})
	}
}

func fieldName(field reflect.StructField, tagName string) (string, map[string]bool) {
	options := map[string]bool{}
	tag := strings.TrimSpace(field.Tag.Get(tagName))
	if tag == "" {
		return "", options
	}
	parts := strings.Split(tag, ",")
	for _, option := range parts[1:] {
		options[strings.TrimSpace(option)] = true
	}
	return strings.TrimSpace(parts[0]), options
}

func applyBinding(schema *Schema, binding string) {
	if schema == nil {
		return
	}
	for _, rule := range strings.Split(binding, ",") {
		rule = strings.TrimSpace(rule)
		if rule == "" || rule == "omitempty" || rule == "required" {
			continue
		}
		name, raw, hasValue := strings.Cut(rule, "=")
		if !hasValue {
			continue
		}
		switch strings.TrimSpace(name) {
		case "min":
			applyMin(schema, raw)
		case "max":
			applyMax(schema, raw)
		case "oneof":
			for _, value := range strings.Fields(raw) {
				schema.Enum = append(schema.Enum, value)
			}
		}
	}
}

func applyMin(schema *Schema, raw string) {
	if schema.Type == "string" {
		if value, err := strconv.Atoi(raw); err == nil {
			schema.MinLength = &value
		}
		return
	}
	if value, err := strconv.ParseFloat(raw, 64); err == nil {
		schema.Minimum = &value
	}
}

func applyMax(schema *Schema, raw string) {
	if schema.Type == "string" {
		if value, err := strconv.Atoi(raw); err == nil {
			schema.MaxLength = &value
		}
		return
	}
	if value, err := strconv.ParseFloat(raw, 64); err == nil {
		schema.Maximum = &value
	}
}

func hasBindingRule(binding, expected string) bool {
	for _, rule := range strings.Split(binding, ",") {
		if strings.TrimSpace(rule) == expected {
			return true
		}
	}
	return false
}
