package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/zhimma/grove/pkg/request"
)

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-Id")
		if requestID == "" {
			requestID = uuid.NewString()
		}
		c.Writer.Header().Set("X-Request-Id", requestID)
		request.SetRequestID(c, requestID)
		c.Next()
	}
}
