package errors

import "net/http"

type HTTPError struct {
	HTTPStatus int
	Message    string
	Code       string
	Data       map[string]interface{}
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

func (e *HTTPError) Clone() *HTTPError {
	if e == nil {
		return nil
	}
	cloned := *e
	if e.Data != nil {
		cloned.Data = make(map[string]interface{}, len(e.Data))
		for key, value := range e.Data {
			cloned.Data[key] = value
		}
	}
	return &cloned
}

func (e *HTTPError) WithMessage(message string) *HTTPError {
	cloned := e.Clone()
	cloned.Message = message
	return cloned
}

func (e *HTTPError) WithCode(code string) *HTTPError {
	cloned := e.Clone()
	cloned.Code = code
	return cloned
}

func (e *HTTPError) WithHTTPStatus(httpStatus int) *HTTPError {
	cloned := e.Clone()
	cloned.HTTPStatus = httpStatus
	return cloned
}

func (e *HTTPError) WithData(data map[string]interface{}) *HTTPError {
	cloned := e.Clone()
	cloned.Data = data
	return cloned
}

func (e *HTTPError) WithCause(err error) *HTTPError {
	cloned := e.Clone()
	cloned.Cause = err
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

func ServiceUnavailable() *HTTPError {
	return New(http.StatusServiceUnavailable, "service_unavailable", "服务暂不可用")
}

func Internal() *HTTPError {
	return New(http.StatusInternalServerError, "internal_error", "系统繁忙，请稍后再试")
}

// Common error constants for convenience
var (
	ErrDatabase       = Internal().WithCode("database_error")
	ErrRecordNotFound = NotFound().WithCode("record_not_found")
	ErrNotFound       = NotFound()
)

func Normalize(err error) *HTTPError {
	if err == nil {
		return nil
	}
	if httpErr, ok := err.(*HTTPError); ok {
		return httpErr
	}
	return Internal().WithCause(err)
}
