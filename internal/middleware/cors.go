package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/zhimma/grove/internal/config"
)

func CORS(cfg config.CORSConfig) gin.HandlerFunc {
	allowOrigins := strings.Join(cfg.AllowedOrigins, ", ")
	allowMethods := strings.Join(cfg.AllowedMethods, ", ")
	allowHeaders := strings.Join(cfg.AllowedHeaders, ", ")

	return func(c *gin.Context) {
		if allowOrigins != "" {
			c.Writer.Header().Set("Access-Control-Allow-Origin", allowOrigins)
		}
		if allowMethods != "" {
			c.Writer.Header().Set("Access-Control-Allow-Methods", allowMethods)
		}
		if allowHeaders != "" {
			c.Writer.Header().Set("Access-Control-Allow-Headers", allowHeaders)
		}
		if cfg.AllowCredentials {
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		}
		if cfg.MaxAge > 0 {
			c.Writer.Header().Set("Access-Control-Max-Age", "600")
		}
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}
