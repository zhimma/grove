package middleware

import (
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/zhimma/grove/pkg/request"
)

var safeRequestID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,119}$`)

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := strings.TrimSpace(c.GetHeader("X-Request-Id"))
		if !safeRequestID.MatchString(requestID) {
			requestID = uuid.NewString()
		}
		c.Writer.Header().Set("X-Request-Id", requestID)
		request.SetRequestID(c, requestID)
		c.Next()
	}
}
