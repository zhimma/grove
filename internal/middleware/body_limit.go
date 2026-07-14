package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/zhimma/grove/pkg/errx"
	"github.com/zhimma/grove/pkg/response"
)

func BodyLimit(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if maxBytes <= 0 || c.Request == nil || c.Request.Body == nil {
			c.Next()
			return
		}
		if c.Request.ContentLength > maxBytes {
			response.Fail(c, errx.RequestBodyTooLarge(maxBytes))
			c.Abort()
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		c.Next()
	}
}
