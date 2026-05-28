package middleware

import (
	"runtime/debug"

	"github.com/gin-gonic/gin"

	"github.com/zhimma/grove/pkg/errx"
	"github.com/zhimma/grove/pkg/logger"
	"github.com/zhimma/grove/pkg/request"
	"github.com/zhimma/grove/pkg/response"
)

func Recovery() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		logger.Error().
			Interface("panic", recovered).
			Str("request_id", request.GetRequestID(c)).
			Bytes("stack", debug.Stack()).
			Msg("异常已恢复")

		response.Fail(c, errx.Internal())
		c.Abort()
	})
}
