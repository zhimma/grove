package response

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	pkgerrors "github.com/zhimma/grove/pkg/errors"
	"github.com/zhimma/grove/pkg/logger"
	"github.com/zhimma/grove/pkg/reqctx"
)

type Response struct {
	Code      int         `json:"code"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data,omitempty"`
	RequestID string      `json:"request_id,omitempty"`
}

func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:      0,
		Message:   "ok",
		Data:      data,
		RequestID: reqctx.GetRequestID(c),
	})
}

func Fail(c *gin.Context, input interface{}) {
	httpErr := normalize(input)
	if httpErr == nil {
		return
	}

	message := responseMessage(httpErr)
	reqctx.SetErrorMeta(c, reqctx.ErrorMeta{
		HTTPStatus:    httpErr.HTTPStatus,
		Code:          httpErr.Code,
		Message:       message,
		InternalError: httpErr.HTTPStatus >= http.StatusInternalServerError,
		HasCause:      httpErr.Cause != nil,
	})
	logFailure(c, httpErr, message)

	resp := Response{
		Code:      -1,
		Message:   message,
		RequestID: reqctx.GetRequestID(c),
	}
	if data := buildErrorData(httpErr, reqctx.GetRequestMeta(c).Debug); len(data) > 0 {
		resp.Data = data
	}

	c.JSON(httpErr.HTTPStatus, resp)
}

func responseMessage(httpErr *pkgerrors.HTTPError) string {
	if httpErr == nil {
		return ""
	}
	if httpErr.HTTPStatus == http.StatusInternalServerError || httpErr.Code == "internal_error" {
		return pkgerrors.Internal().Message
	}
	if httpErr.Message != "" {
		return httpErr.Message
	}
	return http.StatusText(httpErr.HTTPStatus)
}

func buildErrorData(httpErr *pkgerrors.HTTPError, debug bool) map[string]interface{} {
	if httpErr == nil {
		return nil
	}

	var data map[string]interface{}
	if httpErr.Data != nil {
		data = make(map[string]interface{}, len(httpErr.Data)+1)
		for key, value := range httpErr.Data {
			data[key] = value
		}
	}

	if httpErr.Code != "" {
		if data == nil {
			data = make(map[string]interface{}, 1)
		}
		if _, exists := data["error_code"]; !exists {
			data["error_code"] = httpErr.Code
		}
	}
	if debug && httpErr.Cause != nil {
		if data == nil {
			data = make(map[string]interface{}, 1)
		}
		data["debug"] = map[string]interface{}{
			"error": httpErr.Cause.Error(),
			"type":  fmt.Sprintf("%T", httpErr.Cause),
		}
	}

	return data
}

func logFailure(c *gin.Context, httpErr *pkgerrors.HTTPError, message string) {
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
	identity := reqctx.GetIdentity(c)
	event.
		Str("request_id", reqctx.GetRequestID(c)).
		Str("method", c.Request.Method).
		Str("path", c.Request.URL.Path).
		Str("route", c.FullPath()).
		Int("status", httpErr.HTTPStatus).
		Str("error_code", httpErr.Code).
		Str("message", message).
		Str("admin_id", identity.AdminID).
		Str("user_id", identity.UserID).
		Msg("请求处理失败")
}

func failureLogLevel(httpErr *pkgerrors.HTTPError) zerolog.Level {
	if httpErr.HTTPStatus >= http.StatusInternalServerError {
		return zerolog.ErrorLevel
	}
	switch httpErr.HTTPStatus {
	case http.StatusUnauthorized, http.StatusForbidden, http.StatusTooManyRequests:
		return zerolog.WarnLevel
	default:
		return zerolog.NoLevel
	}
}

func normalize(input interface{}) *pkgerrors.HTTPError {
	switch value := input.(type) {
	case nil:
		return nil
	case *pkgerrors.HTTPError:
		return value
	case error:
		return pkgerrors.Normalize(value)
	case string:
		return pkgerrors.InvalidParams().WithMessage(value)
	default:
		return pkgerrors.Internal()
	}
}
