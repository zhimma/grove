package middleware

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"syscall"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/trace"

	"github.com/zhimma/grove/pkg/logger"
)

func captureRuntimeLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	previous, level := logger.Logger(), zerolog.GlobalLevel()
	var output bytes.Buffer
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	logger.InitForTest(zerolog.New(&output))
	t.Cleanup(func() {
		logger.InitForTest(previous)
		zerolog.SetGlobalLevel(level)
	})
	return &output
}

func TestRequestLoggerCarriesRequestAndTraceIDs(t *testing.T) {
	output := captureRuntimeLog(t)
	span := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: trace.TraceID{1, 2, 3}, SpanID: trace.SpanID{4, 5, 6},
	})
	engine := gin.New()
	engine.Use(RequestID(), RequestMeta("console", false))
	engine.GET("/test", func(c *gin.Context) {
		log := logger.FromContext(c.Request.Context())
		log.Info().Msg("业务操作已完成")
		if got := logger.RIDFromContext(c.Request.Context()); got != c.GetHeader("X-Request-Id") {
			t.Errorf("context request ID=%q", got)
		}
		c.Status(http.StatusOK)
	})
	for _, id := range []string{"request-one", "request-two"} {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Request-Id", id)
		req = req.WithContext(trace.ContextWithSpanContext(req.Context(), span))
		engine.ServeHTTP(httptest.NewRecorder(), req)
		var event map[string]any
		if err := json.Unmarshal(output.Bytes(), &event); err != nil {
			t.Fatal(err)
		}
		if event["request_id"] != id || event["trace_id"] != span.TraceID().String() || event["span_id"] != span.SpanID().String() {
			t.Fatalf("missing or stale context fields: %#v", event)
		}
		output.Reset()
	}
	logger.Info().Msg("进程日志")
	if strings.Contains(output.String(), "request_id") {
		t.Fatal("request fields escaped into the global logger")
	}
}

func TestRecoveryDoesNotDumpRequestSecrets(t *testing.T) {
	for _, mode := range []string{gin.DebugMode, gin.ReleaseMode} {
		for _, panicValue := range []any{
			"unexpected failure",
			&net.OpError{Op: "write", Net: "tcp", Err: &os.SyscallError{Syscall: "write", Err: syscall.EPIPE}},
			&net.OpError{Op: "read", Net: "tcp", Err: &os.SyscallError{Syscall: "read", Err: syscall.ECONNRESET}},
		} {
			t.Run(mode+"/"+strings.ReplaceAll(panicType(panicValue), "/", "-"), func(t *testing.T) {
				output := captureRuntimeLog(t)
				previousMode, previousWriter := gin.Mode(), gin.DefaultErrorWriter
				var ginOutput bytes.Buffer
				gin.SetMode(mode)
				gin.DefaultErrorWriter = &ginOutput
				t.Cleanup(func() { gin.SetMode(previousMode); gin.DefaultErrorWriter = previousWriter })
				engine := gin.New()
				engine.Use(RequestID(), RequestMeta("console", false), Recovery())
				engine.GET("/panic", func(*gin.Context) { panic(panicValue) })
				req := httptest.NewRequest(http.MethodGet, "/panic?token=query-secret", nil)
				req.Header.Set("Cookie", "session=cookie-secret")
				req.Header.Set("X-Api-Key", "api-key-secret")
				req.Header.Set("Authorization", "Bearer bearer-secret")
				engine.ServeHTTP(httptest.NewRecorder(), req)
				for _, secret := range []string{"query-secret", "cookie-secret", "api-key-secret", "bearer-secret"} {
					if strings.Contains(output.String()+ginOutput.String(), secret) {
						t.Fatalf("request secret reached recovery logs: %s", secret)
					}
				}
				if output.Len() == 0 || ginOutput.Len() != 0 {
					t.Fatalf("runtime logs=%d bytes, uncontrolled Gin logs=%d bytes", output.Len(), ginOutput.Len())
				}
			})
		}
	}
}

func panicType(value any) string {
	if err, ok := value.(error); ok {
		return err.Error()
	}
	return "panic"
}
