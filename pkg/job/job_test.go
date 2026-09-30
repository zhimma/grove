package job

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/zhimma/grove/internal/observability"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

const testTaskType = "test.message"

type testPayload struct {
	Message string `json:"message"`
}

func TestClientEnqueueRequiresInitializedClient(t *testing.T) {
	var client *Client
	if _, err := client.Enqueue(context.Background(), testTaskType, testPayload{Message: "hello"}); err == nil {
		t.Fatal("expected enqueue error for nil client")
	}
}

func TestServerRegisterAllowsNilServerState(t *testing.T) {
	server := NewServer(RedisConfig{}, ServerConfig{})
	if server == nil || server.mux == nil {
		t.Fatal("expected initialized server mux")
	}

	if err := server.Register(testTaskType, func(context.Context, *asynq.Task) error {
		return nil
	}); err != nil {
		t.Fatalf("register task: %v", err)
	}
}

func TestServerRegisterReturnsErrorForNilServer(t *testing.T) {
	var server *Server
	if err := server.Register(testTaskType, func(context.Context, *asynq.Task) error {
		return nil
	}); err == nil {
		t.Fatal("expected nil server register error")
	}
}

func TestParsePayloadDecodesJSON(t *testing.T) {
	task := asynq.NewTask(testTaskType, []byte(`{"message":"hello","_grove_trace":{"traceparent":"ignored-by-old-decoder"}}`))

	var payload testPayload
	if err := ParsePayload(task, &payload); err != nil {
		t.Fatalf("parse payload failed: %v", err)
	}
	if payload.Message != "hello" {
		t.Fatalf("expected message hello, got %q", payload.Message)
	}
}

func TestTraceContextInjectionPreservesJSONPayloadCompatibility(t *testing.T) {
	traceID, _ := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	spanID, _ := trace.SpanIDFromHex("00f067aa0ba902b7")
	ctx := trace.ContextWithSpanContext(context.Background(), trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
	}))
	propagator := propagation.TraceContext{}
	body := injectTraceContext(ctx, propagator, []byte(`{"message":"hello"}`))

	var payload testPayload
	if err := json.Unmarshal(body, &payload); err != nil || payload.Message != "hello" {
		t.Fatalf("business payload changed: payload=%#v err=%v body=%s", payload, err, body)
	}
	extracted := extractTraceContext(context.Background(), propagator, body)
	if got := trace.SpanContextFromContext(extracted).TraceID(); got != traceID {
		t.Fatalf("trace ID = %s, want %s", got, traceID)
	}
}

func TestTraceContextInjectionLeavesNonObjectPayloadUnchanged(t *testing.T) {
	body := []byte(`"hello"`)
	if got := injectTraceContext(context.Background(), propagation.TraceContext{}, body); string(got) != string(body) {
		t.Fatalf("non-object payload changed: %s", got)
	}
}

func TestServerTelemetryContinuesInjectedTrace(t *testing.T) {
	tracerProvider := sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.AlwaysSample()))
	t.Cleanup(func() { _ = tracerProvider.Shutdown(context.Background()) })
	otel.SetTracerProvider(tracerProvider)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	traceID, _ := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	spanID, _ := trace.SpanIDFromHex("00f067aa0ba902b7")
	parent := trace.ContextWithSpanContext(context.Background(), trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
	}))
	body := injectTraceContext(parent, propagation.TraceContext{}, []byte(`{"message":"hello"}`))
	server := NewServer(RedisConfig{}, ServerConfig{})
	var handledTraceID trace.TraceID
	if err := server.Register("test:trace", func(ctx context.Context, _ *asynq.Task) error {
		handledTraceID = trace.SpanContextFromContext(ctx).TraceID()
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := server.mux.ProcessTask(context.Background(), asynq.NewTask("test:trace", body)); err != nil {
		t.Fatal(err)
	}
	if handledTraceID != traceID {
		t.Fatalf("consumer trace ID = %s, want %s", handledTraceID, traceID)
	}
}

func TestServerTelemetryExportsJobResultMetrics(t *testing.T) {
	runtime, err := observability.New(context.Background(), observability.Config{
		ServiceName:      "worker",
		Environment:      "test",
		Enabled:          true,
		MetricsEnabled:   true,
		MetricsPath:      "/metrics",
		TraceSampleRatio: 1,
	})
	if err != nil {
		t.Fatalf("new observability runtime: %v", err)
	}
	t.Cleanup(func() { _ = runtime.Close() })

	server := NewServer(RedisConfig{}, ServerConfig{})
	if err := server.Register("test:success", func(context.Context, *asynq.Task) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if err := server.mux.ProcessTask(context.Background(), asynq.NewTask("test:success", []byte(`{}`))); err != nil {
		t.Fatal(err)
	}

	resp := httptest.NewRecorder()
	runtime.MetricsHandler().ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := resp.Body.String()
	if !strings.Contains(body, `task="test:success"`) || !strings.Contains(body, `result="success"`) {
		t.Fatalf("missing job metrics: %s", body)
	}
}
