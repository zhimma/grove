package handler

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/zhimma/grove/pkg/request"
)

func setAuditMeta(c *gin.Context, targetType, targetID string, detail map[string]any) {
	if c == nil {
		return
	}
	request.SetAuditMeta(c, request.AuditMeta{
		TargetType: strings.TrimSpace(targetType),
		TargetID:   strings.TrimSpace(targetID),
		Detail:     detail,
	})
}
