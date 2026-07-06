package response

import (
	"fmt"
	"maps"
	"net/http"

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

func Success(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Response{
		Code:      0,
		Message:   "ok",
		Data:      data,
		RequestID: request.GetRequestID(c),
	})
}

// Fail 接受 *errx.HTTPError 或 error;nil 表示不写出。
// 不再接受 string,避免把硬编码字符串被静默吞成 400。
func Fail(c *gin.Context, err error) {
	httpErr := normalize(err)
	if httpErr == nil {
		return
	}

	message := responseMessage(httpErr)
	request.SetErrorMeta(c, request.ErrorMeta{
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
		RequestID: request.GetRequestID(c),
	}
	if data := errorData(httpErr, request.GetRequestMeta(c).Debug); len(data) > 0 {
		resp.Data = data
	}

	c.JSON(httpErr.HTTPStatus, resp)
}

func responseMessage(httpErr *errx.HTTPError) string {
	if httpErr == nil {
		return ""
	}
	if httpErr.HTTPStatus == http.StatusInternalServerError || httpErr.Code == "internal_error" {
		return errx.Internal().Message
	}
	if httpErr.Message != "" {
		return httpErr.Message
	}
	return http.StatusText(httpErr.HTTPStatus)
}

func errorData(httpErr *errx.HTTPError, debug bool) map[string]any {
	if httpErr == nil {
		return nil
	}
	var data map[string]any
	if httpErr.Data != nil {
		data = make(map[string]any, len(httpErr.Data)+1)
		maps.Copy(data, httpErr.Data)
	}
	if httpErr.Code != "" {
		if data == nil {
			data = map[string]any{}
		}
		if _, exists := data["error_code"]; !exists {
			data["error_code"] = httpErr.Code
		}
	}
	if debug && httpErr.Cause != nil {
		if data == nil {
			data = map[string]any{}
		}
		data["debug"] = map[string]any{
			"error": httpErr.Cause.Error(),
			"type":  fmt.Sprintf("%T", httpErr.Cause),
		}
	}
	return data
}

func logFailure(c *gin.Context, httpErr *errx.HTTPError, message string) {
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
	identity := request.GetIdentity(c)
	event.
		Str("request_id", request.GetRequestID(c)).
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

func failureLogLevel(httpErr *errx.HTTPError) zerolog.Level {
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

// normalize 把 error 收敛成 *errx.HTTPError。
// *errx.HTTPError 原样返回;其它 error 走 errx.Normalize 包成 500。
// 不再接受 string 和 default 分支——业务层写错类型时由编译期拒绝。
func normalize(err error) *errx.HTTPError {
	if err == nil {
		return nil
	}
	return errx.Normalize(err)
}
