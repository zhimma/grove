package bootstrap

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"github.com/zhimma/grove/internal/config"
	"github.com/zhimma/grove/pkg/logger"
)

func TestGlobalAllowsNilConfig(t *testing.T) {
	loader := NewMiddlewareLoader(nil, "api")

	middlewares := loader.Global()
	if len(middlewares) != 6 {
		t.Fatalf("expected 6 default middlewares, got %d", len(middlewares))
	}
}

func TestGlobalMiddlewareSetsSecurityHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	loader := NewMiddlewareLoader(&config.Config{
		Security: config.SecurityConfig{HSTSEnabled: true},
	}, "api")
	engine.Use(loader.Global()...)
	engine.GET("/health", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	engine.ServeHTTP(recorder, req)

	want := map[string]string{
		"X-Content-Type-Options":    "nosniff",
		"X-Frame-Options":           "DENY",
		"Referrer-Policy":           "strict-origin-when-cross-origin",
		"Permissions-Policy":        "camera=(), microphone=(), geolocation=()",
		"Strict-Transport-Security": "max-age=31536000; includeSubDomains",
	}
	for name, value := range want {
		if got := recorder.Header().Get(name); got != value {
			t.Errorf("expected %s %q, got %q", name, value, got)
		}
	}
}

func TestGlobalBodyLimitPreservesCORSAndAccessLog(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logBuffer := &testLogWriter{}
	previous := logger.Logger()
	logger.InitForTest(zerolog.New(logBuffer))
	t.Cleanup(func() { logger.InitForTest(previous) })

	engine := gin.New()
	loader := NewMiddlewareLoader(&config.Config{
		App:    config.AppConfig{Debug: false},
		Server: config.ServerConfig{MaxBodyBytes: 4},
		CORS: config.CORSConfig{
			Enabled:        true,
			AllowedOrigins: []string{"https://console.example.com"},
			AllowedMethods: []string{"POST"},
			AllowedHeaders: []string{"Content-Type"},
		},
	}, "console")
	engine.Use(loader.Global()...)
	engine.POST("/upload", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/upload", bytes.NewBufferString("12345"))
	req.Header.Set("Origin", "https://console.example.com")
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d", recorder.Code)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "https://console.example.com" {
		t.Fatalf("expected CORS header on rejected body, got %q", got)
	}
	if logBuffer.CountMessage("请求已完成") != 1 {
		t.Fatalf("oversized request must be logged: %s", logBuffer.String())
	}
}

func TestGlobalMiddlewareSetsDebugAndLogsRecoveredPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logBuffer := &testLogWriter{}
	previous := logger.Logger()
	logger.InitForTest(zerolog.New(logBuffer))
	t.Cleanup(func() {
		logger.InitForTest(previous)
	})

	engine := gin.New()
	loader := NewMiddlewareLoader(&config.Config{
		App:  config.AppConfig{Debug: true},
		CORS: config.CORSConfig{Enabled: false},
	}, "api")
	engine.Use(loader.Global()...)
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
	data := payload["data"].(map[string]any)
	if _, exists := data["debug"]; exists {
		t.Fatalf("panic response must not expose debug data, got %#v", data["debug"])
	}
	if logBuffer.CountMessage("请求已完成") != 1 {
		t.Fatalf("expected access log after recovered panic, got logs: %s", logBuffer.String())
	}
}

type testLogWriter struct {
	lines []string
}

func (w *testLogWriter) Write(p []byte) (int, error) {
	w.lines = append(w.lines, string(p))
	return len(p), nil
}

func (w *testLogWriter) String() string {
	var out string
	for _, line := range w.lines {
		out += line
	}
	return out
}

func (w *testLogWriter) CountMessage(message string) int {
	count := 0
	for _, line := range w.lines {
		var payload map[string]any
		if err := json.Unmarshal([]byte(line), &payload); err != nil {
			continue
		}
		if payload["message"] == message {
			count++
		}
	}
	return count
}
