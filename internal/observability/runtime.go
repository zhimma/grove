package observability

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	otelprometheus "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

type Config struct {
	ServiceName      string
	Environment      string
	Enabled          bool
	MetricsEnabled   bool
	MetricsPath      string
	TraceSampleRatio float64
	OTLPTraceURL     string
	OTLPInsecure     bool
}

const meterName = "github.com/zhimma/grove/internal/observability"

type Runtime struct {
	serviceName    string
	metricsPath    string
	propagator     propagation.TextMapPropagator
	tracerProvider *sdktrace.TracerProvider
	meterProvider  *sdkmetric.MeterProvider
	metricsHandler http.Handler

	httpRequests metric.Int64Counter
	httpErrors   metric.Int64Counter
	httpDuration metric.Float64Histogram
	dbPool       dbPoolInstruments

	mu            sync.Mutex
	registrations []metric.Registration
	closeOnce     sync.Once
	closeErr      error
}

func New(ctx context.Context, cfg Config) (*Runtime, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	serviceName := strings.TrimSpace(cfg.ServiceName)
	if serviceName == "" {
		serviceName = "grove"
	}
	metricsPath := strings.TrimSpace(cfg.MetricsPath)
	if metricsPath == "" {
		metricsPath = "/metrics"
	}
	ratio := cfg.TraceSampleRatio
	if ratio < 0 || ratio > 1 {
		return nil, fmt.Errorf("trace sample ratio must be between 0 and 1")
	}

	res, err := resource.Merge(resource.Default(), resource.NewSchemaless(
		attribute.String("service.name", serviceName),
		attribute.String("deployment.environment.name", strings.TrimSpace(cfg.Environment)),
	))
	if err != nil {
		return nil, fmt.Errorf("create OpenTelemetry resource: %w", err)
	}

	traceOptions := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratio))),
	}
	if endpoint := strings.TrimSpace(cfg.OTLPTraceURL); endpoint != "" {
		exporterOptions := []otlptracehttp.Option{otlptracehttp.WithEndpointURL(endpoint)}
		if cfg.OTLPInsecure {
			exporterOptions = append(exporterOptions, otlptracehttp.WithInsecure())
		}
		exporter, err := otlptracehttp.New(ctx, exporterOptions...)
		if err != nil {
			return nil, fmt.Errorf("create OTLP trace exporter: %w", err)
		}
		traceOptions = append(traceOptions, sdktrace.WithBatcher(exporter))
	}
	tracerProvider := sdktrace.NewTracerProvider(traceOptions...)

	var (
		meterOptions = []sdkmetric.Option{
			sdkmetric.WithResource(res),
			sdkmetric.WithView(sdkmetric.NewView(
				sdkmetric.Instrument{Name: "http.server.duration"},
				sdkmetric.Stream{Aggregation: sdkmetric.AggregationExplicitBucketHistogram{
					Boundaries: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
				}},
			)),
			sdkmetric.WithView(sdkmetric.NewView(
				sdkmetric.Instrument{Name: "job.duration"},
				sdkmetric.Stream{Aggregation: sdkmetric.AggregationExplicitBucketHistogram{
					Boundaries: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60, 300},
				}},
			)),
		}
		metricsHandler http.Handler
	)
	if cfg.MetricsEnabled {
		registry := prometheus.NewRegistry()
		exporter, err := otelprometheus.New(
			otelprometheus.WithRegisterer(registry),
			otelprometheus.WithNamespace("grove"),
		)
		if err != nil {
			_ = tracerProvider.Shutdown(ctx)
			return nil, fmt.Errorf("create Prometheus exporter: %w", err)
		}
		meterOptions = append(meterOptions, sdkmetric.WithReader(exporter))
		metricsHandler = promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	}
	meterProvider := sdkmetric.NewMeterProvider(meterOptions...)
	propagator := propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})

	runtime := &Runtime{
		serviceName:    serviceName,
		metricsPath:    metricsPath,
		propagator:     propagator,
		tracerProvider: tracerProvider,
		meterProvider:  meterProvider,
		metricsHandler: metricsHandler,
	}
	if err := runtime.initInstruments(); err != nil {
		_ = runtime.Close()
		return nil, err
	}

	otel.SetTracerProvider(tracerProvider)
	otel.SetMeterProvider(meterProvider)
	otel.SetTextMapPropagator(propagator)
	return runtime, nil
}

func (r *Runtime) initInstruments() error {
	meter := r.meterProvider.Meter(meterName)
	var err error
	r.httpRequests, err = meter.Int64Counter("http.server.requests", metric.WithDescription("HTTP server requests"))
	if err != nil {
		return err
	}
	r.httpErrors, err = meter.Int64Counter("http.server.errors", metric.WithDescription("HTTP server responses with status >= 400"))
	if err != nil {
		return err
	}
	r.httpDuration, err = meter.Float64Histogram("http.server.duration", metric.WithUnit("s"), metric.WithDescription("HTTP server request duration"))
	if err != nil {
		return err
	}
	return r.dbPool.init(meter)
}

func (r *Runtime) Tracer(name string) trace.Tracer {
	if r == nil || r.tracerProvider == nil {
		return otel.Tracer(name)
	}
	return r.tracerProvider.Tracer(name)
}

func (r *Runtime) MeterProvider() metric.MeterProvider {
	if r == nil || r.meterProvider == nil {
		return otel.GetMeterProvider()
	}
	return r.meterProvider
}

func (r *Runtime) TracerProvider() trace.TracerProvider {
	if r == nil || r.tracerProvider == nil {
		return otel.GetTracerProvider()
	}
	return r.tracerProvider
}

func (r *Runtime) Propagator() propagation.TextMapPropagator {
	if r == nil || r.propagator == nil {
		return otel.GetTextMapPropagator()
	}
	return r.propagator
}

func (r *Runtime) MetricsHandler() http.Handler {
	if r == nil {
		return nil
	}
	return r.metricsHandler
}

func (r *Runtime) MetricsPath() string {
	if r == nil {
		return ""
	}
	return r.metricsPath
}

func (r *Runtime) Close() error {
	if r == nil {
		return nil
	}
	r.closeOnce.Do(func() {
		r.mu.Lock()
		registrations := append([]metric.Registration(nil), r.registrations...)
		r.registrations = nil
		r.mu.Unlock()
		for _, registration := range registrations {
			if err := registration.Unregister(); err != nil {
				r.closeErr = errors.Join(r.closeErr, err)
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if otel.GetMeterProvider() == r.meterProvider {
			otel.SetMeterProvider(metricnoop.NewMeterProvider())
		}
		if otel.GetTracerProvider() == r.tracerProvider {
			otel.SetTracerProvider(tracenoop.NewTracerProvider())
		}
		if r.meterProvider != nil {
			r.closeErr = errors.Join(r.closeErr, r.meterProvider.Shutdown(ctx))
		}
		if r.tracerProvider != nil {
			r.closeErr = errors.Join(r.closeErr, r.tracerProvider.Shutdown(ctx))
		}
	})
	return r.closeErr
}
