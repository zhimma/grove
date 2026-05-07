package middleware

import (
	"github.com/gin-gonic/gin"

	"github.com/zhimma/grove/pkg/reqctx"
)

func RequestMeta(serviceName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		reqctx.SetRequestMeta(c, reqctx.RequestMeta{
			RequestID: reqctx.GetRequestID(c),
			App:       serviceName,
			Method:    c.Request.Method,
			Path:      c.Request.URL.Path,
			Route:     c.FullPath(),
			ClientIP:  c.ClientIP(),
			UserAgent: c.Request.UserAgent(),
		})
		c.Next()
	}
}
