package middleware

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"github.com/zhimma/grove/pkg/logger"
	"github.com/zhimma/grove/pkg/reqctx"
)

func AccessLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		status := c.Writer.Status()
		level := zerolog.InfoLevel
		if status >= http.StatusInternalServerError {
			level = zerolog.ErrorLevel
		} else if status >= http.StatusBadRequest {
			level = zerolog.WarnLevel
		}
		meta := reqctx.GetErrorMeta(c)
		identity := reqctx.GetIdentity(c)
		log := logger.Logger()
		log.WithLevel(level).
			Str("request_id", reqctx.GetRequestID(c)).
			Str("method", c.Request.Method).
			Str("path", c.Request.URL.Path).
			Str("route", c.FullPath()).
			Int("status", status).
			Str("error_code", meta.Code).
			Str("message", meta.Message).
			Str("admin_id", identity.AdminID).
			Str("user_id", identity.UserID).
			Dur("duration", time.Since(start)).
			Msg("请求已完成")
	}
}
