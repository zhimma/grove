package observability

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

func (r *Runtime) GinMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if r == nil || (r.metricsHandler != nil && c.Request.URL.Path == r.metricsPath) {
			c.Next()
			return
		}

		ctx := r.propagator.Extract(c.Request.Context(), propagation.HeaderCarrier(c.Request.Header))
		ctx, span := r.Tracer("github.com/zhimma/grove/http").Start(
			ctx,
			c.Request.Method,
			trace.WithSpanKind(trace.SpanKindServer),
		)
		c.Request = c.Request.WithContext(ctx)
		startedAt := time.Now()
		c.Next()

		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		status := c.Writer.Status()
		span.SetName(c.Request.Method + " " + route)
		span.SetAttributes(
			attribute.String("http.request.method", c.Request.Method),
			attribute.String("http.route", route),
			attribute.Int("http.response.status_code", status),
		)
		if status >= http.StatusInternalServerError {
			span.SetStatus(codes.Error, http.StatusText(status))
		}

		attrs := metric.WithAttributes(
			attribute.String("service", r.serviceName),
			attribute.String("method", c.Request.Method),
			attribute.String("route", route),
			attribute.String("status", strconv.Itoa(status)),
		)
		r.httpRequests.Add(ctx, 1, attrs)
		r.httpDuration.Record(ctx, time.Since(startedAt).Seconds(), attrs)
		if status >= http.StatusBadRequest {
			r.httpErrors.Add(ctx, 1, attrs)
		}
		span.End()
	}
}
