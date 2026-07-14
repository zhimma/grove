package observability

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"
)

func TestRuntimeGinMiddlewarePropagatesTraceAndExportsLowCardinalityMetrics(t *testing.T) {
	gin.SetMode(gin.TestMode)
	runtime, err := New(context.Background(), Config{
		ServiceName:      "api",
		Environment:      "test",
		Enabled:          true,
		MetricsEnabled:   true,
		MetricsPath:      "/metrics",
		TraceSampleRatio: 1,
	})
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}
	t.Cleanup(func() { _ = runtime.Close() })

	var traceID string
	engine := gin.New()
	engine.Use(runtime.GinMiddleware())
	engine.GET("/users/:id", func(c *gin.Context) {
		traceID = trace.SpanContextFromContext(c.Request.Context()).TraceID().String()
		c.Status(http.StatusInternalServerError)
	})

	req := httptest.NewRequest(http.MethodGet, "/users/secret-user-id", nil)
	req.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, req)
	if traceID != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Fatalf("trace propagation failed: %q", traceID)
	}

	metricsResp := httptest.NewRecorder()
	runtime.MetricsHandler().ServeHTTP(metricsResp, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := metricsResp.Body.String()
	if !strings.Contains(body, `route="/users/:id"`) || !strings.Contains(body, `status="500"`) {
		t.Fatalf("missing HTTP metric labels: %s", body)
	}
	if strings.Contains(body, "secret-user-id") {
		t.Fatalf("metrics must not contain raw URL paths: %s", body)
	}
}

func TestRuntimeGinMiddlewareTracesMetricsPathWhenMetricsAreDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	runtime, err := New(context.Background(), Config{
		ServiceName:      "api",
		Environment:      "test",
		Enabled:          true,
		MetricsEnabled:   false,
		MetricsPath:      "/internal/metrics",
		TraceSampleRatio: 1,
	})
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}
	t.Cleanup(func() { _ = runtime.Close() })

	var traced bool
	engine := gin.New()
	engine.Use(runtime.GinMiddleware())
	engine.GET("/internal/metrics", func(c *gin.Context) {
		traced = trace.SpanContextFromContext(c.Request.Context()).IsValid()
		c.Status(http.StatusNoContent)
	})
	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/internal/metrics", nil))
	if !traced {
		t.Fatal("metrics path must remain traced when metrics export is disabled")
	}
}

func TestInstrumentGORMRecordsOperationWithoutSQL(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSpanProcessor(recorder),
	)
	t.Cleanup(func() { _ = tracerProvider.Shutdown(context.Background()) })
	runtime := &Runtime{tracerProvider: tracerProvider}

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := runtime.InstrumentGORM("default", db); err != nil {
		t.Fatalf("instrument gorm: %v", err)
	}
	type item struct {
		ID   int
		Name string
	}
	if err := db.AutoMigrate(&item{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := db.Create(&item{Name: "secret-value"}).Error; err != nil {
		t.Fatalf("create: %v", err)
	}

	spans := recorder.Ended()
	if len(spans) == 0 {
		t.Fatal("expected gorm spans")
	}
	for _, span := range spans {
		for _, attr := range span.Attributes() {
			if strings.Contains(string(attr.Key), "statement") || strings.Contains(attr.Value.Emit(), "secret-value") {
				t.Fatalf("gorm span leaked SQL data: %#v", span.Attributes())
			}
		}
	}
}

func TestObserveDBPoolExportsConnectionStats(t *testing.T) {
	runtime, err := New(context.Background(), Config{
		ServiceName:      "api",
		Environment:      "test",
		Enabled:          true,
		MetricsEnabled:   true,
		MetricsPath:      "/metrics",
		TraceSampleRatio: 1,
	})
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}
	t.Cleanup(func() { _ = runtime.Close() })
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	if err := runtime.ObserveDBPool("default", sqlDB); err != nil {
		t.Fatalf("observe pool: %v", err)
	}

	resp := httptest.NewRecorder()
	runtime.MetricsHandler().ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if body := resp.Body.String(); !strings.Contains(body, `database="default"`) || !strings.Contains(body, "grove_db_pool_open") {
		t.Fatalf("missing DB pool metrics: %s", body)
	}
}

func TestRedisHookRecordsCommandNameWithoutArguments(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSpanProcessor(recorder),
	)
	t.Cleanup(func() { _ = tracerProvider.Shutdown(context.Background()) })
	hook := RedisHook{tracer: tracerProvider.Tracer("test")}
	cmd := redis.NewStatusCmd(context.Background(), "set", "token", "secret-value")
	if err := hook.ProcessHook(func(context.Context, redis.Cmder) error { return nil })(context.Background(), cmd); err != nil {
		t.Fatal(err)
	}

	spans := recorder.Ended()
	if len(spans) != 1 || spans[0].Name() != "redis.set" {
		t.Fatalf("unexpected redis spans: %#v", spans)
	}
	for _, attr := range spans[0].Attributes() {
		if strings.Contains(attr.Value.Emit(), "secret-value") || strings.Contains(attr.Value.Emit(), "token") {
			t.Fatalf("redis span leaked command arguments: %#v", spans[0].Attributes())
		}
	}
}

func TestRecordClientErrorMarksSpan(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	tracerProvider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	_, span := tracerProvider.Tracer("test").Start(context.Background(), "dependency")
	recordClientError(span, errors.New("failed"))
	span.SetAttributes(attribute.String("safe", "value"))
	span.End()
	if len(recorder.Ended()) != 1 {
		t.Fatal("expected ended span")
	}
}
