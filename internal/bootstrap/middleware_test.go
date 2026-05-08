package bootstrap

import (
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
	if len(middlewares) != 4 {
		t.Fatalf("expected 4 default middlewares, got %d", len(middlewares))
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
