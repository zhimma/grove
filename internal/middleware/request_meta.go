package middleware

import (
	"github.com/gin-gonic/gin"

	"github.com/zhimma/grove/pkg/reqctx"
)

func RequestMeta(serviceName string, debug ...bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		appDebug := true
		if len(debug) > 0 {
			appDebug = debug[0]
		}
		reqctx.SetRequestMeta(c, reqctx.RequestMeta{
			RequestID: reqctx.GetRequestID(c),
			App:       serviceName,
			Debug:     appDebug,
			Method:    c.Request.Method,
			Path:      c.Request.URL.Path,
			Route:     c.FullPath(),
			ClientIP:  c.ClientIP(),
			UserAgent: c.Request.UserAgent(),
		})
		c.Next()
	}
}
