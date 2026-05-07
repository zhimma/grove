package middleware

import (
	"time"

	"github.com/gin-gonic/gin"

	"github.com/zhimma/grove/pkg/logger"
	"github.com/zhimma/grove/pkg/reqctx"
)

func AccessLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		logger.Info().
			Str("request_id", reqctx.GetRequestID(c)).
			Str("method", c.Request.Method).
			Str("path", c.Request.URL.Path).
			Int("status", c.Writer.Status()).
			Dur("duration", time.Since(start)).
			Msg("请求已完成")
	}
}
