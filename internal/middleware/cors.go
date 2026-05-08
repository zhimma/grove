package middleware

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/zhimma/grove/internal/config"
)

func CORS(cfg config.CORSConfig) gin.HandlerFunc {
	allowAllOrigins := containsCORSValue(cfg.AllowedOrigins, "*")
	allowMethods := strings.Join(cfg.AllowedMethods, ", ")
	allowHeaders := strings.Join(cfg.AllowedHeaders, ", ")

	return func(c *gin.Context) {
		origin := strings.TrimSpace(c.GetHeader("Origin"))
		allowedOrigin := ""
		switch {
		case origin != "" && allowAllOrigins && !cfg.AllowCredentials:
			allowedOrigin = "*"
		case origin != "" && originAllowed(origin, cfg.AllowedOrigins):
			allowedOrigin = origin
		case origin == "" && allowAllOrigins && !cfg.AllowCredentials:
			allowedOrigin = "*"
		}
		if allowedOrigin != "" {
			c.Writer.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
			c.Writer.Header().Add("Vary", "Origin")
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
			c.Writer.Header().Set("Access-Control-Max-Age", strconv.Itoa(cfg.MaxAge))
		}
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}

func originAllowed(origin string, allowedOrigins []string) bool {
	origin = strings.TrimSpace(origin)
	if origin == "" {
		return false
	}
	for _, allowed := range allowedOrigins {
		if strings.TrimSpace(allowed) == origin {
			return true
		}
	}
	return false
}

func containsCORSValue(values []string, target string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == target {
			return true
		}
	}
	return false
}
