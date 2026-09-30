package validation

import (
	"reflect"
	"strconv"
	"strings"
)

func validationFieldPath(target any, namespace string) string {
	if root := indirectStructType(target); root != nil {
		return strings.TrimPrefix(namespace, (*root).Name()+".")
	}
	return namespace
}

// 路径中的数组下标和 map 键属于字段位置，不能与同名叶子字段合并。
func resolveFieldMeta(target any, path, source string) fieldMeta {
	path = strings.TrimSpace(path)
	if path == "" {
		return fieldMeta{Key: "_error", Label: "参数"}
	}
	var current reflect.Type
	if root := indirectStructType(target); root != nil {
		current = *root
	}
	var keys []string
	label := path
	for _, part := range splitFieldPath(path) {
		name, indexes := part, ""
		if index := strings.IndexByte(part, '['); index >= 0 {
			name, indexes = part[:index], part[index:]
		}
		current = indirectType(current)
		// JSON 解码错误使用点分隔的集合索引，校验器使用方括号；统一输出位置。
		if current != nil && len(keys) > 0 && indexes == "" {
			indexed := current.Kind() == reflect.Map
			if current.Kind() == reflect.Array || current.Kind() == reflect.Slice {
				index, err := strconv.Atoi(name)
				indexed = err == nil && index >= 0
			}
			if indexed {
				keys[len(keys)-1] += "[" + name + "]"
				current = current.Elem()
				continue
			}
		}
		// JSON 类型错误只提供字段路径，集合下标可能缺失；仍保留元素字段的标签。
		for current != nil && (current.Kind() == reflect.Slice || current.Kind() == reflect.Array || current.Kind() == reflect.Map) {
			current = indirectType(current.Elem())
		}
		var field reflect.StructField
		var found bool
		if current != nil && current.Kind() == reflect.Struct {
			field, found = findFieldMetaTarget(current, name)
		}
		wireName := lowerFirst(name)
		inline := false
		if found {
			tag := firstTagValue(field.Tag.Get(sourceTagName(source)))
			if tag != "" && tag != "-" {
				wireName = tag
			}
			current = indirectType(field.Type)
			inline = field.Anonymous && tag == "" && current.Kind() == reflect.Struct
			label = strings.TrimSpace(field.Tag.Get("label"))
			if label == "" {
				label = wireName
			}
		} else {
			current = nil
			label = wireName
		}
		if !inline {
			keys = append(keys, wireName+indexes)
		}
		for remaining := indexes; remaining != ""; {
			end := strings.IndexByte(remaining, ']')
			if end < 0 {
				break
			}
			current = indirectType(current)
			if current != nil {
				switch current.Kind() {
				case reflect.Slice, reflect.Array, reflect.Map:
					current = current.Elem()
				default:
					current = nil
				}
			}
			remaining = remaining[end+1:]
		}
	}
	if len(keys) == 0 {
		return fieldMeta{Key: lowerFirst(path), Label: label}
	}
	return fieldMeta{Key: strings.Join(keys, "."), Label: label}
}

func indirectType(value reflect.Type) reflect.Type {
	for value != nil && value.Kind() == reflect.Pointer {
		value = value.Elem()
	}
	return value
}

func splitFieldPath(path string) []string {
	var parts []string
	start, depth := 0, 0
	for index, char := range path {
		switch char {
		case '[':
			depth++
		case ']':
			if depth > 0 {
				depth--
			}
		case '.':
			if depth == 0 {
				parts = append(parts, path[start:index])
				start = index + 1
			}
		}
	}
	return append(parts, path[start:])
}
