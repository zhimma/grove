package middleware

import (
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"

	"github.com/zhimma/grove/pkg/logger"
	"github.com/zhimma/grove/pkg/request"
)

func RequestMeta(serviceName string, debug ...bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		appDebug := true
		if len(debug) > 0 {
			appDebug = debug[0]
		}
		request.SetRequestMeta(c, request.RequestMeta{
			RequestID: request.RequestID(c),
			App:       serviceName,
			Debug:     appDebug,
			Method:    c.Request.Method,
			Path:      c.Request.URL.Path,
			Route:     c.FullPath(),
			ClientIP:  c.ClientIP(),
			UserAgent: c.Request.UserAgent(),
		})
		ctx := c.Request.Context()
		log := logger.FromContext(ctx)
		fields := log.With().Str("request_id", request.RequestID(c))
		if span := trace.SpanContextFromContext(ctx); span.IsValid() {
			fields = fields.Str("trace_id", span.TraceID().String()).Str("span_id", span.SpanID().String())
		}
		ctx = logger.WithContext(ctx, fields.Logger())
		ctx = logger.WithRID(ctx, request.RequestID(c))
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
