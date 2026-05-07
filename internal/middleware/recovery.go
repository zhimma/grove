package middleware

import (
	"github.com/gin-gonic/gin"

	pkgerrors "github.com/zhimma/grove/pkg/errors"
	"github.com/zhimma/grove/pkg/logger"
	"github.com/zhimma/grove/pkg/reqctx"
	"github.com/zhimma/grove/pkg/response"
)

func Recovery() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		logger.Error().
			Interface("panic", recovered).
			Str("request_id", reqctx.GetRequestID(c)).
			Msg("异常已恢复")

		response.Fail(c, pkgerrors.Internal())
		c.Abort()
	})
}
