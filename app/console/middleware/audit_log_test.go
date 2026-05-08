package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/zhimma/grove/internal/model"
	"github.com/zhimma/grove/pkg/errors"
	"github.com/zhimma/grove/pkg/reqctx"
	"github.com/zhimma/grove/pkg/response"
)

func TestAuditOperationUsesErrorMetaMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/audit.db"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.ConsoleOperationLog{}); err != nil {
		t.Fatalf("migrate operation logs: %v", err)
	}

	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		reqctx.SetRequestID(c, "req-audit")
		reqctx.SetRequestMeta(c, reqctx.RequestMeta{
			RequestID: "req-audit",
			Method:    c.Request.Method,
			Path:      c.Request.URL.Path,
			Route:     "/console/v1/admins/:id",
			ClientIP:  c.ClientIP(),
			UserAgent: c.Request.UserAgent(),
		})
		c.Next()
	})
	engine.Use(AuditOperation(db))
	engine.DELETE("/console/v1/admins/:id", func(c *gin.Context) {
		response.Fail(c, errors.Forbidden().WithMessage("不能删除超级管理员").WithCode("super_admin_forbidden"))
	})

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/console/v1/admins/1", nil)
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", recorder.Code)
	}

	var log model.ConsoleOperationLog
	if err := db.First(&log).Error; err != nil {
		t.Fatalf("read operation log: %v", err)
	}
	if log.ErrorMessage != "不能删除超级管理员" {
		t.Fatalf("expected business error message, got %q", log.ErrorMessage)
	}
}
