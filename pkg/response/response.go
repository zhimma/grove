package response

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"github.com/zhimma/grove/pkg/errx"
	"github.com/zhimma/grove/pkg/logger"
	"github.com/zhimma/grove/pkg/request"
)

type Response struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	Data      any    `json:"data,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

// Success writes a successful response with the canonical 200 envelope.
func Success(c *gin.Context, data any) {
	write(c, http.StatusOK, Response{
		Code:      0,
		Message:   "ok",
		Data:      data,
		RequestID: requestID(c),
	})
}

// Created is the success counterpart for resource-creation endpoints. The
// envelope remains identical to Success so clients only need to inspect HTTP
// status when they care about creation semantics.
func Created(c *gin.Context, data any) {
	write(c, http.StatusCreated, Response{
		Code:      0,
		Message:   "ok",
		Data:      data,
		RequestID: requestID(c),
	})
}

// NoContent emits a standards-compliant 204 response. A 204 response cannot
// contain the JSON envelope; the request ID is therefore carried by the
// X-Request-Id header when available.
func NoContent(c *gin.Context) {
	if c == nil {
		return
	}
	setRequestIDHeader(c, requestID(c))
	c.Status(http.StatusNoContent)
	// Gin delays WriteHeader until the first write. Force the empty status so
	// httptest and real net/http clients observe 204 rather than the recorder's
	// default 200.
	c.Writer.WriteHeaderNow()
}

func Fail(c *gin.Context, err error) {
	httpErr := errx.Normalize(err)
	if httpErr == nil {
		return
	}

	status := errx.EffectiveStatus(httpErr)
	code := errx.EffectiveCode(httpErr)
	message := responseMessage(httpErr, status)
	request.SetErrorMeta(c, request.ErrorMeta{
		HTTPStatus:    status,
		Code:          code,
		Message:       message,
		InternalError: status >= http.StatusInternalServerError,
		HasCause:      httpErr.Cause != nil,
	})
	logFailure(c, httpErr, status, code, message)

	resp := Response{
		Code:      -1,
		Message:   message,
		RequestID: requestID(c),
	}
	if data := buildErrorData(httpErr, status, code, request.RequestMetaOf(c).Debug); len(data) > 0 {
		resp.Data = data
	}

	write(c, status, resp)
}

func responseMessage(httpErr *errx.HTTPError, status int) string {
	if httpErr == nil {
		return ""
	}
	// InternalError is deliberately generic. Availability and gateway errors
	// retain their safe, caller-provided messages (for example, "权限控制器未
	// 配置") so operators and clients can distinguish a 503 from a 500.
	if status == http.StatusInternalServerError || strings.TrimSpace(httpErr.Code) == "internal_error" {
		return errx.Internal().Message
	}
	if httpErr.Message != "" {
		return httpErr.Message
	}
	if statusText := http.StatusText(status); statusText != "" {
		return statusText
	}
	return "请求处理失败"
}

func buildErrorData(httpErr *errx.HTTPError, status int, code string, debug bool) map[string]any {
	if httpErr == nil {
		return nil
	}

	var data map[string]any
	if httpErr.Data != nil {
		data = make(map[string]any, len(httpErr.Data)+1)
		for key, value := range httpErr.Data {
			data[key] = value
		}
	}
	// `debug` is reserved for the response layer. Prevent a service payload
	// from smuggling diagnostic details into production responses.
	if data != nil && !debug {
		delete(data, "debug")
	}

	if code != "" {
		if data == nil {
			data = make(map[string]any, 1)
		}
		// The envelope's machine-readable code is authoritative. Do not let a
		// stale value embedded in Data disagree with HTTPError.Code.
		data["error_code"] = code
	}
	// Causes are diagnostic server details. Even when debug is enabled, do not
	// expose causes attached to client-facing 4xx responses (they may contain
	// token/parser or storage details). Internal failures are still inspectable
	// locally through the explicit debug envelope.
	if debug && status >= http.StatusInternalServerError && httpErr.Cause != nil {
		if data == nil {
			data = make(map[string]any, 1)
		}
		data["debug"] = map[string]any{
			"error": httpErr.Cause.Error(),
			"type":  fmt.Sprintf("%T", httpErr.Cause),
		}
	}

	return data
}

func logFailure(c *gin.Context, httpErr *errx.HTTPError, status int, code, message string) {
	if c == nil || httpErr == nil {
		return
	}
	level := failureLogLevel(httpErr)
	if level == zerolog.NoLevel {
		return
	}

	log := logger.Logger()
	event := log.WithLevel(level)
	if httpErr.Cause != nil {
		event = event.Err(httpErr.Cause).Str("cause", httpErr.Cause.Error())
	}
	identity := request.IdentityOf(c)
	method, path := "", ""
	if c.Request != nil {
		method = c.Request.Method
		if c.Request.URL != nil {
			path = c.Request.URL.Path
		}
	}
	event.
		Str("request_id", requestID(c)).
		Str("method", method).
		Str("path", path).
		Str("route", c.FullPath()).
		Int("status", status).
		Str("error_code", code).
		Str("message", message).
		Str("admin_id", identity.AdminID).
		Str("user_id", identity.UserID).
		Msg("请求处理失败")
}

func failureLogLevel(httpErr *errx.HTTPError) zerolog.Level {
	status := errx.EffectiveStatus(httpErr)
	if status >= http.StatusInternalServerError {
		return zerolog.ErrorLevel
	}
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden, http.StatusTooManyRequests:
		return zerolog.WarnLevel
	default:
		return zerolog.NoLevel
	}
}

func write(c *gin.Context, status int, payload Response) {
	if c == nil {
		return
	}
	setRequestIDHeader(c, payload.RequestID)
	if status == http.StatusNoContent {
		c.Status(status)
		return
	}
	c.JSON(status, payload)
}

func requestID(c *gin.Context) string {
	if c == nil {
		return ""
	}
	if id := strings.TrimSpace(request.RequestID(c)); id != "" {
		return id
	}
	// Unit handlers and middleware-adjacent code may call response helpers
	// before RequestID has run. Preserve a validated upstream ID when present;
	// the normal RequestID middleware remains the source of generated IDs.
	return strings.TrimSpace(c.GetHeader("X-Request-Id"))
}

func setRequestIDHeader(c *gin.Context, id string) {
	if c == nil || strings.TrimSpace(id) == "" {
		return
	}
	c.Header("X-Request-Id", id)
}
