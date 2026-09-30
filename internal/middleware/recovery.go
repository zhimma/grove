package middleware

import (
	"errors"
	"net"
	"runtime/debug"
	"syscall"

	"github.com/gin-gonic/gin"

	"github.com/zhimma/grove/pkg/errx"
	"github.com/zhimma/grove/pkg/logger"
	"github.com/zhimma/grove/pkg/request"
	"github.com/zhimma/grove/pkg/response"
)

func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			recovered := recover()
			if recovered == nil {
				return
			}
			c.Abort()
			log := logger.FromContext(c.Request.Context())
			if logger.RIDFromContext(c.Request.Context()) == "" {
				log = log.With().Str("request_id", request.RequestID(c)).Logger()
			}
			if err, ok := recovered.(error); ok && isBrokenConnection(err) {
				// 连接已断开时保留错误记录，不再写响应，也不输出原始请求。
				_ = c.Error(err)
				log.Warn().Err(err).Msg("客户端连接已断开")
				return
			}
			log.Error().Interface("panic", recovered).Bytes("stack", debug.Stack()).Msg("异常已恢复")
			response.Fail(c, errx.Internal())
		}()
		c.Next()
	}
}

func isBrokenConnection(err error) bool {
	var networkErr *net.OpError
	return errors.As(err, &networkErr) && (errors.Is(networkErr, syscall.EPIPE) || errors.Is(networkErr, syscall.ECONNRESET))
}
