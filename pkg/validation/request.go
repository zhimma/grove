package validation

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"

	"github.com/zhimma/grove/pkg/errx"
)

const validationMessage = "请求参数校验失败"
const invalidParamsMessage = "请求参数格式不正确"

var unknownJSONFieldPattern = regexp.MustCompile(`unknown field "([^"]+)"`)

var errTrailingJSON = errors.New("request contains more than one JSON value")

func BindJSON(c *gin.Context, target any) error {
	// Keep the historical permissive behavior for compatibility endpoints.
	// Public API handlers should call BindJSONStrict; callers that need the
	// legacy behavior can use the explicit BindJSONAllowUnknown alias.
	return bindJSON(c, target, false)
}

// BindJSONStrict rejects unknown object fields and trailing JSON values before
// running the same validator and request hooks as BindJSON. This is the
// default contract for newly added public API endpoints.
func BindJSONStrict(c *gin.Context, target any) error {
	return bindJSON(c, target, true)
}

// BindJSONAllowUnknown documents an intentional compatibility exception. It
// is equivalent to the legacy BindJSON behavior and should not be used for
// newly introduced public endpoints.
func BindJSONAllowUnknown(c *gin.Context, target any) error {
	return bindJSON(c, target, false)
}

func bindJSON(c *gin.Context, target any, disallowUnknown bool) error {
	if c == nil || c.Request == nil || c.Request.Body == nil {
		return newBindingError(c, errors.New("invalid request"), target, "json")
	}

	decoder := json.NewDecoder(c.Request.Body)
	if disallowUnknown {
		decoder.DisallowUnknownFields()
	}

	if err := decoder.Decode(target); err != nil {
		if isValidationError(err) {
			return newValidationError(c, err, target, "json")
		}
		return newBindingError(c, err, target, "json")
	}

	// A request is one JSON document. Gin's default binder stops after the
	// first value, which could otherwise accept `{...}{...}` accidentally.
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			err = errTrailingJSON
		}
		return newBindingError(c, err, target, "json")
	}

	if binding.Validator != nil {
		if err := binding.Validator.ValidateStruct(target); err != nil {
			if isValidationError(err) {
				return newValidationError(c, err, target, "json")
			}
			return newBindingError(c, err, target, "json")
		}
	}
	return runRequestHooks(target)
}

func BindQuery(c *gin.Context, target any) error {
	if c == nil || c.Request == nil {
		return newBindingError(c, errors.New("invalid request"), target, "query")
	}
	if err := c.ShouldBindQuery(target); err != nil {
		if isValidationError(err) {
			return newValidationError(c, err, target, "query")
		}
		return newBindingError(c, err, target, "query")
	}
	return runRequestHooks(target)
}

func BindURI(c *gin.Context, target any) error {
	if c == nil || c.Request == nil {
		return newBindingError(c, errors.New("invalid request"), target, "uri")
	}
	if err := c.ShouldBindUri(target); err != nil {
		if isValidationError(err) {
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
			var httpErr *errx.HTTPError
			if errors.As(err, &httpErr) && httpErr != nil {
				return httpErr
			}
			return errx.InvalidParams().
				WithHTTPStatus(http.StatusUnprocessableEntity).
				WithMessage(validationMessage).
				WithData(map[string]any{
					"errors": map[string][]string{
						"_error": {err.Error()},
					},
				})
		}
	}
	return nil
}

func newValidationError(c *gin.Context, err error, target any, source string) error {
	return errx.InvalidParams().
		WithHTTPStatus(http.StatusUnprocessableEntity).
		WithMessage(validationMessage).
		WithData(map[string]any{
			"errors": formatErrors(c, err, target, source),
		})
}

func newBindingError(c *gin.Context, err error, target any, source string) error {
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		return errx.RequestBodyTooLarge(maxBytesErr.Limit)
	}
	return errx.InvalidParams().WithMessage(invalidParamsMessage).WithData(map[string]any{
		"errors": formatErrors(c, err, target, source),
	})
}

func formatErrors(c *gin.Context, err error, target any, source string) map[string][]string {
	if err == nil {
		return nil
	}

	var validationErrors validator.ValidationErrors
	if errors.As(err, &validationErrors) {
		return formatValidationErrors(validationErrors, target, source)
	}

	var unmarshalTypeErr *json.UnmarshalTypeError
	if errors.As(err, &unmarshalTypeErr) {
		meta := resolveFieldMeta(target, unmarshalTypeErr.Field, source)
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

	if matches := unknownJSONFieldPattern.FindStringSubmatch(err.Error()); len(matches) == 2 {
		return map[string][]string{
			"_error": {fmt.Sprintf("请求包含不支持的字段%s", matches[1])},
		}
	}
	if errors.Is(err, errTrailingJSON) {
		return map[string][]string{
			"_error": {"请求体只能包含一个 JSON 对象"},
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
	if structType == nil || c == nil || c.Request == nil {
		return fieldMeta{}, false
	}
	return resolveTypeErrorFieldInStruct(c, target, *structType, source)
}

func resolveTypeErrorFieldInStruct(c *gin.Context, target any, t reflect.Type, source string) (fieldMeta, bool) {
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.Anonymous {
			fieldType := field.Type
			for fieldType.Kind() == reflect.Pointer {
				fieldType = fieldType.Elem()
			}
			if fieldType.Kind() == reflect.Struct {
				if meta, ok := resolveTypeErrorFieldInStruct(c, target, fieldType, source); ok {
					return meta, true
				}
			}
		}
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
			return resolveFieldMeta(target, field.Name, source), true
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
	for fieldType.Kind() == reflect.Pointer {
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

func formatValidationErrors(validationErrors validator.ValidationErrors, target any, source string) map[string][]string {
	errorsMap := make(map[string][]string, len(validationErrors))
	for _, fieldErr := range validationErrors {
		path := validationFieldPath(target, fieldErr.StructNamespace())
		meta := resolveFieldMeta(target, path, source)
		errorsMap[meta.Key] = append(errorsMap[meta.Key], formatValidationMessage(target, meta, fieldErr, source))
	}
	if len(errorsMap) == 0 {
		return map[string][]string{
			"_error": {"请求参数格式不正确"},
		}
	}
	return errorsMap
}

func isValidationError(err error) bool {
	if err == nil {
		return false
	}
	var validationErrors validator.ValidationErrors
	return errors.As(err, &validationErrors)
}

func formatValidationMessage(target any, meta fieldMeta, fieldErr validator.FieldError, source string) string {
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
		parts := splitFieldPath(validationFieldPath(target, fieldErr.StructNamespace()))
		if len(parts) > 1 {
			param = strings.Join(parts[:len(parts)-1], ".") + "." + param
		}
		other := resolveFieldMeta(target, param, source)
		return fmt.Sprintf("%s必须与%s一致", label, other.Label)
	default:
		return fmt.Sprintf("%s不合法", label)
	}
}

type fieldMeta struct {
	Key   string
	Label string
}

func indirectStructType(target any) *reflect.Type {
	if target == nil {
		return nil
	}

	t := reflect.TypeOf(target)
	for t.Kind() == reflect.Pointer {
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
