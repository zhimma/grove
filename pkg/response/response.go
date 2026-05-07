package response

import (
	"net/http"

	"github.com/gin-gonic/gin"

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

	if httpErr.Cause != nil {
		logger.Error().
			Err(httpErr.Cause).
			Str("request_id", reqctx.GetRequestID(c)).
			Str("path", c.Request.URL.Path).
			Msg("请求处理失败")
	}

	resp := Response{
		Code:      -1,
		Message:   httpErr.Message,
		RequestID: reqctx.GetRequestID(c),
	}
	if data := buildErrorData(httpErr); len(data) > 0 {
		resp.Data = data
	}

	c.JSON(httpErr.HTTPStatus, resp)
}

func buildErrorData(httpErr *pkgerrors.HTTPError) map[string]interface{} {
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

	return data
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
