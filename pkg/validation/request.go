package validation

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"

	pkgerrors "github.com/zhimma/grove/pkg/errors"
)

const validationMessage = "请求参数校验失败"
const invalidParamsMessage = "请求参数格式不正确"

func BindJSON(c *gin.Context, target any) error {
	if err := c.ShouldBindJSON(target); err != nil {
		if _, ok := err.(validator.ValidationErrors); ok {
			return newValidationError(c, err, target, "json")
		}
		return newBindingError(c, err, target, "json")
	}
	return runRequestHooks(target)
}

func BindQuery(c *gin.Context, target any) error {
	if err := c.ShouldBindQuery(target); err != nil {
		if _, ok := err.(validator.ValidationErrors); ok {
			return newValidationError(c, err, target, "query")
		}
		return newBindingError(c, err, target, "query")
	}
	return runRequestHooks(target)
}

func BindURI(c *gin.Context, target any) error {
	if err := c.ShouldBindUri(target); err != nil {
		if _, ok := err.(validator.ValidationErrors); ok {
			return newValidationError(c, err, target, "uri")
		}
		return newBindingError(c, err, target, "uri")
	}
	return runRequestHooks(target)
}

func Require(condition bool, message string) error {
	if condition {
		return nil
	}
	return fmt.Errorf("%s", message)
}

func runRequestHooks(target any) error {
	if validatable, ok := target.(interface{ Validate() error }); ok {
		if err := validatable.Validate(); err != nil {
			return pkgerrors.InvalidParams().
				WithHTTPStatus(http.StatusUnprocessableEntity).
				WithMessage(validationMessage).
				WithData(map[string]interface{}{
					"errors": map[string][]string{
						"_error": {err.Error()},
					},
				})
		}
	}
	return nil
}

func newValidationError(c *gin.Context, err error, target any, source string) error {
	return pkgerrors.InvalidParams().
		WithHTTPStatus(http.StatusUnprocessableEntity).
		WithMessage(validationMessage).
		WithData(map[string]interface{}{
			"errors": formatErrors(c, err, target, source),
		})
}

func newBindingError(c *gin.Context, err error, target any, source string) error {
	return pkgerrors.InvalidParams().WithMessage(invalidParamsMessage).WithData(map[string]interface{}{
		"errors": formatErrors(c, err, target, source),
	})
}

func formatErrors(c *gin.Context, err error, target any, source string) map[string][]string {
	if err == nil {
		return nil
	}

	if validationErrors, ok := err.(validator.ValidationErrors); ok {
		return formatValidationErrors(validationErrors, target)
	}

	var unmarshalTypeErr *json.UnmarshalTypeError
	if errors.As(err, &unmarshalTypeErr) {
		meta := resolveFieldMeta(target, unmarshalTypeErr.Field)
		return map[string][]string{
			meta.Key: {fmt.Sprintf("%s格式不正确", meta.Label)},
		}
	}

	var syntaxErr *json.SyntaxError
	if errors.As(err, &syntaxErr) || errors.Is(err, io.ErrUnexpectedEOF) {
		return map[string][]string{
			"_error": {"请求体格式不正确"},
		}
	}

	var numErr *strconv.NumError
	if errors.As(err, &numErr) {
		if meta, ok := resolveTypeErrorField(c, target, source); ok {
			return map[string][]string{
				meta.Key: {fmt.Sprintf("%s格式不正确", meta.Label)},
			}
		}
		return map[string][]string{
			"_error": {"请求参数格式不正确"},
		}
	}

	return map[string][]string{
		"_error": {"请求参数格式不正确"},
	}
}

func resolveTypeErrorField(c *gin.Context, target any, source string) (fieldMeta, bool) {
	structType := indirectStructType(target)
	if structType == nil || c == nil {
		return fieldMeta{}, false
	}

	t := *structType
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tagName := sourceTagName(source)
		tagValue := firstTagValue(field.Tag.Get(tagName))
		if tagValue == "" || tagValue == "-" {
			continue
		}

		rawValue, ok := readRequestValue(c, source, tagValue)
		if !ok || strings.TrimSpace(rawValue) == "" {
			continue
		}

		if fieldValueHasTypeError(field.Type, rawValue) {
			return resolveFieldMeta(target, field.Name), true
		}
	}

	return fieldMeta{}, false
}

func sourceTagName(source string) string {
	switch source {
	case "uri":
		return "uri"
	case "query":
		return "form"
	default:
		return "json"
	}
}

func readRequestValue(c *gin.Context, source, key string) (string, bool) {
	switch source {
	case "uri":
		for _, param := range c.Params {
			if param.Key == key {
				return param.Value, true
			}
		}
		return "", false
	case "query":
		values, ok := c.Request.URL.Query()[key]
		if !ok || len(values) == 0 {
			return "", false
		}
		return values[0], true
	default:
		return "", false
	}
}

func fieldValueHasTypeError(fieldType reflect.Type, rawValue string) bool {
	for fieldType.Kind() == reflect.Ptr {
		fieldType = fieldType.Elem()
	}

	switch fieldType.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		_, err := strconv.ParseInt(rawValue, 10, 64)
		return err != nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		_, err := strconv.ParseUint(rawValue, 10, 64)
		return err != nil
	case reflect.Float32, reflect.Float64:
		_, err := strconv.ParseFloat(rawValue, 64)
		return err != nil
	case reflect.Bool:
		_, err := strconv.ParseBool(rawValue)
		return err != nil
	default:
		return false
	}
}

func formatValidationErrors(validationErrors validator.ValidationErrors, target any) map[string][]string {
	errorsMap := make(map[string][]string, len(validationErrors))
	for _, fieldErr := range validationErrors {
		meta := resolveFieldMeta(target, fieldErr.StructField())
		errorsMap[meta.Key] = append(errorsMap[meta.Key], formatValidationMessage(target, meta, fieldErr))
	}
	if len(errorsMap) == 0 {
		return map[string][]string{
			"_error": {"请求参数格式不正确"},
		}
	}
	return errorsMap
}

func formatValidationMessage(target any, meta fieldMeta, fieldErr validator.FieldError) string {
	label := meta.Label
	tag := fieldErr.Tag()
	param := fieldErr.Param()

	switch tag {
	case "required":
		return fmt.Sprintf("%s不能为空", label)
	case "email":
		return fmt.Sprintf("%s格式不正确", label)
	case "min":
		if isLengthKind(fieldErr.Kind()) {
			return fmt.Sprintf("%s长度不能少于%s位", label, param)
		}
		return fmt.Sprintf("%s不能小于%s", label, param)
	case "max":
		if isLengthKind(fieldErr.Kind()) {
			return fmt.Sprintf("%s长度不能超过%s位", label, param)
		}
		return fmt.Sprintf("%s不能大于%s", label, param)
	case "len":
		if isLengthKind(fieldErr.Kind()) {
			return fmt.Sprintf("%s长度必须为%s位", label, param)
		}
		return fmt.Sprintf("%s必须等于%s", label, param)
	case "gte":
		return fmt.Sprintf("%s必须大于或等于%s", label, param)
	case "lte":
		return fmt.Sprintf("%s必须小于或等于%s", label, param)
	case "gt":
		return fmt.Sprintf("%s必须大于%s", label, param)
	case "lt":
		return fmt.Sprintf("%s必须小于%s", label, param)
	case "oneof":
		return fmt.Sprintf("%s的取值不合法", label)
	case "eqfield":
		other := resolveFieldMeta(target, param)
		return fmt.Sprintf("%s必须与%s一致", label, other.Label)
	default:
		return fmt.Sprintf("%s不合法", label)
	}
}

type fieldMeta struct {
	Key   string
	Label string
}

func resolveFieldMeta(target any, fieldRef string) fieldMeta {
	fieldRef = lastFieldSegment(fieldRef)
	if fieldRef == "" {
		return fieldMeta{
			Key:   "_error",
			Label: "参数",
		}
	}

	meta := fieldMeta{
		Key:   lowerFirst(fieldRef),
		Label: lowerFirst(fieldRef),
	}

	structType := indirectStructType(target)
	if structType == nil {
		return meta
	}

	field, ok := findFieldMetaTarget(*structType, fieldRef)
	if !ok {
		return meta
	}

	if key := firstNonEmptyTagValue(field, "json", "form", "uri"); key != "" && key != "-" {
		meta.Key = key
	}

	label := strings.TrimSpace(field.Tag.Get("label"))
	if label == "" {
		label = strings.TrimSpace(field.Tag.Get("lebel"))
	}
	if label != "" {
		meta.Label = label
		return meta
	}

	if meta.Key != "" && meta.Key != "_" {
		meta.Label = meta.Key
	}
	return meta
}

func indirectStructType(target any) *reflect.Type {
	if target == nil {
		return nil
	}

	t := reflect.TypeOf(target)
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil
	}
	return &t
}

func findFieldMetaTarget(t reflect.Type, fieldRef string) (reflect.StructField, bool) {
	if field, ok := t.FieldByName(fieldRef); ok {
		return field, true
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		if strings.EqualFold(field.Name, fieldRef) {
			return field, true
		}

		for _, tagName := range []string{"json", "form", "uri"} {
			tagValue := firstTagValue(field.Tag.Get(tagName))
			if tagValue == "" || tagValue == "-" {
				continue
			}
			if tagValue == fieldRef || strings.EqualFold(tagValue, fieldRef) {
				return field, true
			}
		}
	}

	return reflect.StructField{}, false
}

func firstNonEmptyTagValue(field reflect.StructField, tagNames ...string) string {
	for _, tagName := range tagNames {
		if value := firstTagValue(field.Tag.Get(tagName)); value != "" {
			return value
		}
	}
	return ""
}

func firstTagValue(tag string) string {
	if tag == "" {
		return ""
	}

	parts := strings.Split(tag, ",")
	if len(parts) == 0 {
		return ""
	}

	return strings.TrimSpace(parts[0])
}

func lastFieldSegment(field string) string {
	field = strings.TrimSpace(field)
	if field == "" {
		return ""
	}
	if index := strings.LastIndex(field, "."); index >= 0 {
		return field[index+1:]
	}
	return field
}

func lowerFirst(value string) string {
	if value == "" {
		return ""
	}
	return strings.ToLower(value[:1]) + value[1:]
}

func isLengthKind(kind reflect.Kind) bool {
	switch kind {
	case reflect.String, reflect.Array, reflect.Slice, reflect.Map:
		return true
	default:
		return false
	}
}
