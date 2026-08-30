package middleware

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/zhimma/grove/internal/model"
	"github.com/zhimma/grove/pkg/logger"
	"github.com/zhimma/grove/pkg/request"
)

const (
	// Audit records are operational data, not a request-body archive. Keep the
	// limits small enough to bound database growth and avoid accidentally
	// persisting an uploaded/request payload.
	maxAuditQueryLength       = 2000
	maxAuditDetailLength      = 8000
	maxAuditDetailValueLength = 2000
	maxAuditPathLength        = 255
	maxAuditRouteLength       = 255
	maxAuditActionLength      = 180
	maxAuditErrorLength       = 500
	maxAuditRequestIDLength   = 120
	maxAuditClientIPLength    = 64
	maxAuditUserAgentLength   = 500
	maxAuditTargetTypeLength  = 120
	maxAuditTargetIDLength    = 64
	auditRedactedValue        = "REDACTED"
)

var sensitiveAuditKeys = map[string]struct{}{
	"authorization":     {},
	"cookie":            {},
	"set_cookie":        {},
	"password":          {},
	"passwd":            {},
	"pass":              {},
	"pwd":               {},
	"secret":            {},
	"token":             {},
	"access_token":      {},
	"refresh_token":     {},
	"id_token":          {},
	"api_key":           {},
	"apikey":            {},
	"client_secret":     {},
	"private_key":       {},
	"signature":         {},
	"sig":               {},
	"csrf":              {},
	"csrf_token":        {},
	"verification_code": {},
	"otp":               {},
	"code":              {},
}

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
			AdminID:      strings.TrimSpace(identity.AdminID),
			Method:       c.Request.Method,
			Path:         truncateString(c.Request.URL.Path, maxAuditPathLength),
			Route:        truncateString(route, maxAuditRouteLength),
			Module:       detectLogModule(route),
			Action:       truncateString(buildAuditAction(c.Request.Method, route), maxAuditActionLength),
			RequestID:    truncateString(meta.RequestID, maxAuditRequestIDLength),
			StatusCode:   status,
			Success:      status < http.StatusBadRequest,
			ErrorMessage: truncateString(errorMessage, maxAuditErrorLength),
			DurationMS:   time.Since(startedAt).Milliseconds(),
			ClientIP:     truncateString(meta.ClientIP, maxAuditClientIPLength),
			UserAgent:    truncateString(meta.UserAgent, maxAuditUserAgentLength),
			RequestQuery: auditRequestQuery(route, c.Request.URL.RawQuery),
		}
		if auditMeta := request.GetAuditMeta(c); strings.TrimSpace(auditMeta.TargetType) != "" || strings.TrimSpace(auditMeta.TargetID) != "" || len(auditMeta.Detail) > 0 {
			record.TargetType = truncateString(strings.TrimSpace(auditMeta.TargetType), maxAuditTargetTypeLength)
			record.TargetID = truncateString(strings.TrimSpace(auditMeta.TargetID), maxAuditTargetIDLength)
			if detailJSON, ok := marshalAuditDetail(redactAuditDetail(auditMeta.Detail)); ok {
				record.DetailJSON = detailJSON
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

func auditRequestQuery(route, rawQuery string) string {
	if detectLogModule(route) == "system-configs" {
		return ""
	}
	return redactAuditQuery(rawQuery)
}

// redactAuditQuery keeps the query shape useful for troubleshooting while
// replacing values whose keys commonly carry credentials or one-time secrets.
// Parse before truncating so a secret near the end of a long query cannot leak
// merely because it was outside the retained prefix.
func redactAuditQuery(rawQuery string) string {
	rawQuery = strings.TrimSpace(rawQuery)
	if rawQuery == "" {
		return ""
	}
	values, err := url.ParseQuery(rawQuery)
	if err == nil {
		for key, items := range values {
			if isSensitiveAuditKey(key) {
				for i := range items {
					items[i] = auditRedactedValue
				}
				values[key] = items
				continue
			}
			for i := range items {
				items[i] = truncateString(items[i], maxAuditDetailValueLength)
			}
			values[key] = items
		}
		return truncateString(values.Encode(), maxAuditQueryLength)
	}

	// A malformed escape sequence should not make us fall back to storing the
	// original query. Redact each pair conservatively and retain only its safe
	// syntax.
	parts := strings.Split(rawQuery, "&")
	for i, part := range parts {
		key, value, hasValue := strings.Cut(part, "=")
		decodedKey, decodeErr := url.QueryUnescape(key)
		if decodeErr != nil {
			decodedKey = key
		}
		if isSensitiveAuditKey(decodedKey) {
			parts[i] = key + "=" + auditRedactedValue
			continue
		}
		if hasValue {
			parts[i] = key + "=" + truncateString(value, maxAuditDetailValueLength)
		}
	}
	return truncateString(strings.Join(parts, "&"), maxAuditQueryLength)
}

func redactAuditDetail(detail map[string]any) map[string]any {
	if detail == nil {
		return nil
	}
	redacted, ok := redactAuditValue(detail).(map[string]any)
	if !ok {
		return map[string]any{}
	}
	return redacted
}

// marshalAuditDetail keeps the persisted value valid JSON even when the
// redacted payload exceeds its storage budget. Byte-truncating a serialized
// object can produce invalid JSON and turn the detail endpoint into a raw
// string leak; a small marker preserves the bounded, structured contract.
func marshalAuditDetail(detail map[string]any) (string, bool) {
	payload, err := json.Marshal(detail)
	if err != nil {
		return "", false
	}
	if len(payload) <= maxAuditDetailLength {
		return string(payload), true
	}
	truncated, err := json.Marshal(map[string]bool{"_truncated": true})
	if err != nil {
		return "", false
	}
	return string(truncated), true
}

func redactAuditValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, item := range typed {
			if isSensitiveAuditKey(key) {
				result[key] = auditRedactedValue
				continue
			}
			result[key] = redactAuditValue(item)
		}
		return result
	case map[string]string:
		result := make(map[string]string, len(typed))
		for key, item := range typed {
			if isSensitiveAuditKey(key) {
				result[key] = auditRedactedValue
				continue
			}
			result[key] = truncateString(item, maxAuditDetailValueLength)
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for i, item := range typed {
			result[i] = redactAuditValue(item)
		}
		return result
	case []string:
		result := make([]string, len(typed))
		for i, item := range typed {
			result[i] = truncateString(item, maxAuditDetailValueLength)
		}
		return result
	case string:
		return truncateString(typed, maxAuditDetailValueLength)
	default:
		return value
	}
}

func isSensitiveAuditKey(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	normalized = strings.NewReplacer("-", "_", ".", "_", "[", "_", "]", "_").Replace(normalized)
	normalized = strings.Trim(normalized, "_ ")
	if _, ok := sensitiveAuditKeys[normalized]; ok {
		return true
	}
	for _, part := range strings.FieldsFunc(normalized, func(r rune) bool {
		return r == '_' || r == ' ' || r == ':'
	}) {
		if _, ok := sensitiveAuditKeys[part]; ok {
			return true
		}
	}
	return false
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
