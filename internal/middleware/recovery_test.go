package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/zhimma/grove/pkg/request"
)

func TestRecoveryUsesUnifiedErrorResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		request.SetRequestID(c, "req-panic")
		c.Next()
	})
	engine.Use(Recovery())
	engine.GET("/panic", func(c *gin.Context) {
		panic("boom")
	})

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", recorder.Code)
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if got := int(payload["code"].(float64)); got != -1 {
		t.Fatalf("unexpected code: %d", got)
	}
	if got := payload["message"]; got != "系统繁忙，请稍后再试" {
		t.Fatalf("unexpected message: %#v", got)
	}
	data := payload["data"].(map[string]any)
	if got := data["error_code"]; got != "internal_error" {
		t.Fatalf("unexpected error_code: %#v", got)
	}
	if got := payload["request_id"]; got != "req-panic" {
		t.Fatalf("unexpected request_id: %#v", got)
	}
}
