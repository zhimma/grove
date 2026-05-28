package middleware

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/zhimma/grove/internal/model"
	"github.com/zhimma/grove/pkg/logger"
	"github.com/zhimma/grove/pkg/request"
)

func AuditOperation(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		startedAt := time.Now()
		c.Next()

		if db == nil || !shouldAuditOperation(c) {
			return
		}

		meta := request.GetRequestMeta(c)
		identity := request.GetIdentity(c)
		route := meta.Route
		if strings.TrimSpace(route) == "" {
			route = c.FullPath()
		}
		if strings.TrimSpace(route) == "" {
			route = c.Request.URL.Path
		}

		status := c.Writer.Status()
		errorMessage := http.StatusText(status)
		if status >= http.StatusBadRequest {
			if errMeta := request.GetErrorMeta(c); strings.TrimSpace(errMeta.Message) != "" {
				errorMessage = errMeta.Message
			}
		}

		record := model.ConsoleOperationLog{
			AdminID:      identity.AdminID,
			Method:       c.Request.Method,
			Path:         c.Request.URL.Path,
			Route:        route,
			Module:       detectLogModule(route),
			Action:       buildAuditAction(c.Request.Method, route),
			RequestID:    meta.RequestID,
			StatusCode:   status,
			Success:      status < http.StatusBadRequest,
			ErrorMessage: errorMessage,
			DurationMS:   time.Since(startedAt).Milliseconds(),
			ClientIP:     meta.ClientIP,
			UserAgent:    truncateString(meta.UserAgent, 500),
			RequestQuery: truncateString(c.Request.URL.RawQuery, 2000),
		}
		if auditMeta := request.GetAuditMeta(c); strings.TrimSpace(auditMeta.TargetType) != "" || strings.TrimSpace(auditMeta.TargetID) != "" || len(auditMeta.Detail) > 0 {
			record.TargetType = strings.TrimSpace(auditMeta.TargetType)
			record.TargetID = strings.TrimSpace(auditMeta.TargetID)
			if payload, err := json.Marshal(auditMeta.Detail); err == nil {
				record.DetailJSON = truncateString(string(payload), 8000)
			}
		}
		if err := db.WithContext(c.Request.Context()).Create(&record).Error; err != nil {
			logger.Error().
				Err(err).
				Str("module", "console_audit").
				Str("path", c.Request.URL.Path).
				Msg("控制台操作日志写入失败")
		}
	}
}

func shouldAuditOperation(c *gin.Context) bool {
	if c == nil || c.Request == nil {
		return false
	}
	switch strings.ToUpper(strings.TrimSpace(c.Request.Method)) {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return !strings.EqualFold(c.Request.URL.Path, "/console/v1/auth/login")
	default:
		return false
	}
}

func detectLogModule(route string) string {
	trimmed := strings.TrimPrefix(strings.TrimSpace(route), "/console/v1/")
	if trimmed == "" || trimmed == route {
		return "system"
	}
	parts := strings.Split(trimmed, "/")
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		return "system"
	}
	return strings.TrimSpace(parts[0])
}

func buildAuditAction(method, route string) string {
	trimmedRoute := strings.TrimPrefix(strings.TrimSpace(route), "/console/v1/")
	if trimmedRoute == "" {
		trimmedRoute = "/"
	}
	return strings.ToUpper(strings.TrimSpace(method)) + " " + trimmedRoute
}

func truncateString(value string, maxLen int) string {
	if maxLen <= 0 || len(value) <= maxLen {
		return value
	}
	return value[:maxLen]
}
