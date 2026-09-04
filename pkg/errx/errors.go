package errx

import (
	stderrors "errors"
	"net/http"
	"strings"
)

type HTTPError struct {
	HTTPStatus int
	Message    string
	Code       string
	Data       map[string]any
	Cause      error
}

func (e *HTTPError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	return http.StatusText(e.HTTPStatus)
}

// Unwrap exposes the internal cause to errors.Is/errors.As callers while the
// response package remains responsible for deciding whether that cause may be
// shown to a client. Keeping the cause in the error chain also means service
// code can use standard Go error matching without depending on HTTPError.
func (e *HTTPError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func (e *HTTPError) Clone() *HTTPError {
	if e == nil {
		return nil
	}
	cloned := *e
	if e.Data != nil {
		cloned.Data = make(map[string]any, len(e.Data))
		for key, value := range e.Data {
			cloned.Data[key] = value
		}
	}
	return &cloned
}

func (e *HTTPError) WithMessage(message string) *HTTPError {
	cloned := e.Clone()
	if cloned == nil {
		return nil
	}
	cloned.Message = message
	return cloned
}

func (e *HTTPError) WithCode(code string) *HTTPError {
	cloned := e.Clone()
	if cloned == nil {
		return nil
	}
	cloned.Code = code
	return cloned
}

func (e *HTTPError) WithHTTPStatus(httpStatus int) *HTTPError {
	cloned := e.Clone()
	if cloned == nil {
		return nil
	}
	cloned.HTTPStatus = httpStatus
	return cloned
}

func (e *HTTPError) WithData(data map[string]any) *HTTPError {
	cloned := e.Clone()
	if cloned == nil {
		return nil
	}
	cloned.Data = cloneData(data)
	return cloned
}

func (e *HTTPError) WithCause(err error) *HTTPError {
	cloned := e.Clone()
	if cloned == nil {
		return nil
	}
	cloned.Cause = err
	return cloned
}

// WithDataValue is a small convenience for adding one response-safe field
// without mutating the source error's data map. It is intentionally limited to
// a single level; nested values are treated as caller-owned payloads.
func (e *HTTPError) WithDataValue(key string, value any) *HTTPError {
	cloned := e.Clone()
	if cloned == nil {
		return nil
	}
	if cloned.Data == nil {
		cloned.Data = make(map[string]any, 1)
	}
	cloned.Data[key] = value
	return cloned
}

func cloneData(data map[string]any) map[string]any {
	if data == nil {
		return nil
	}
	cloned := make(map[string]any, len(data))
	for key, value := range data {
		cloned[key] = value
	}
	return cloned
}

func New(httpStatus int, code, message string) *HTTPError {
	return &HTTPError{
		HTTPStatus: httpStatus,
		Code:       code,
		Message:    message,
	}
}

func InvalidParams() *HTTPError {
	return New(http.StatusBadRequest, "invalid_params", "请求参数格式不正确")
}

func Unauthorized() *HTTPError {
	return New(http.StatusUnauthorized, "unauthorized", "未登录或登录已失效")
}

func Forbidden() *HTTPError {
	return New(http.StatusForbidden, "forbidden", "无权限访问")
}

func NotFound() *HTTPError {
	return New(http.StatusNotFound, "not_found", "资源不存在")
}

func Conflict() *HTTPError {
	return New(http.StatusConflict, "conflict", "数据冲突")
}

func TooManyRequests() *HTTPError {
	return New(http.StatusTooManyRequests, "too_many_requests", "请求过于频繁")
}

func RequestBodyTooLarge(maxBytes int64) *HTTPError {
	data := map[string]any{}
	if maxBytes > 0 {
		data["max_bytes"] = maxBytes
	}
	return New(http.StatusRequestEntityTooLarge, "request_body_too_large", "请求体超过大小限制").WithData(data)
}

func ServiceUnavailable() *HTTPError {
	return New(http.StatusServiceUnavailable, "service_unavailable", "服务暂不可用")
}

func Internal() *HTTPError {
	return New(http.StatusInternalServerError, "internal_error", "系统繁忙，请稍后再试")
}

// This package deliberately exports no sentinel error values. The constructors
// (NotFound, Internal, ...) return a fresh *HTTPError on every call, and
// *HTTPError has no Is method, so a package-level `var ErrNotFound = NotFound()`
// would never match under errors.Is — it would compile, read correctly, and
// silently evaluate to false. Add an Is method first if sentinels are ever
// needed.

func Normalize(err error) *HTTPError {
	if err == nil {
		return nil
	}
	var httpErr *HTTPError
	if stderrors.As(err, &httpErr) {
		// An interface can contain a typed nil *HTTPError. Treat it as an
		// unknown failure instead of returning nil and silently dropping the
		// response.
		if httpErr != nil {
			return httpErr
		}
	}
	return Internal().WithCause(err)
}

// CodeForStatus returns the framework's stable fallback error code for a
// status when a custom HTTPError did not provide one. Constructors already set
// their own codes; this helper closes the contract for errors created with New
// or by third-party services.
func CodeForStatus(status int) string {
	switch status {
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return "invalid_params"
	case http.StatusUnauthorized:
		return "unauthorized"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusConflict:
		return "conflict"
	case http.StatusRequestEntityTooLarge:
		return "request_body_too_large"
	case http.StatusTooManyRequests:
		return "too_many_requests"
	case http.StatusServiceUnavailable:
		return "service_unavailable"
	case http.StatusInternalServerError:
		return "internal_error"
	default:
		return "http_error"
	}
}

// EffectiveCode trims accidental whitespace and returns a non-empty stable
// code for every HTTPError. It does not mutate the error, so callers may safely
// reuse package-level sentinel values.
func EffectiveCode(err *HTTPError) string {
	if err == nil {
		return ""
	}
	if code := strings.TrimSpace(err.Code); code != "" {
		return code
	}
	return CodeForStatus(effectiveStatus(err.HTTPStatus))
}

// EffectiveStatus keeps malformed custom status values from reaching
// net/http, which otherwise emits an invalid response or panics in tests.
func EffectiveStatus(err *HTTPError) int {
	if err == nil {
		return http.StatusInternalServerError
	}
	return effectiveStatus(err.HTTPStatus)
}

func effectiveStatus(status int) int {
	if status < 100 || status > 599 {
		return http.StatusInternalServerError
	}
	return status
}

// BadRequest and UnprocessableEntity are explicit aliases for callers that
// prefer status-oriented names. InvalidParams remains the compatibility
// constructor used by existing handlers.
func BadRequest() *HTTPError {
	return InvalidParams()
}

func UnprocessableEntity() *HTTPError {
	return InvalidParams().WithHTTPStatus(http.StatusUnprocessableEntity).WithMessage("请求参数校验失败")
}
