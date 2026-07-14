package observability

import (
	"context"
	"errors"
	"net"
	"strings"

	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type RedisHook struct {
	tracer trace.Tracer
}

var _ redis.Hook = RedisHook{}

func (r *Runtime) NewRedisHook() redis.Hook {
	return RedisHook{tracer: r.Tracer("github.com/zhimma/grove/redis")}
}

func (h RedisHook) DialHook(next redis.DialHook) redis.DialHook {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		ctx, span := h.tracer.Start(ctx, "redis.dial", trace.WithSpanKind(trace.SpanKindClient))
		span.SetAttributes(attribute.String("network.transport", network), attribute.String("server.address", addr))
		conn, err := next(ctx, network, addr)
		recordClientError(span, err)
		span.End()
		return conn, err
	}
}

func (h RedisHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		name := "unknown"
		if cmd != nil && strings.TrimSpace(cmd.Name()) != "" {
			name = strings.ToLower(strings.TrimSpace(cmd.Name()))
		}
		ctx, span := h.tracer.Start(ctx, "redis."+name, trace.WithSpanKind(trace.SpanKindClient))
		span.SetAttributes(attribute.String("db.system", "redis"), attribute.String("db.operation.name", name))
		err := next(ctx, cmd)
		if !errors.Is(err, redis.Nil) {
			recordClientError(span, err)
		}
		span.End()
		return err
	}
}

func (h RedisHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		ctx, span := h.tracer.Start(ctx, "redis.pipeline", trace.WithSpanKind(trace.SpanKindClient))
		span.SetAttributes(attribute.String("db.system", "redis"), attribute.Int("db.operation.batch.size", len(cmds)))
		err := next(ctx, cmds)
		if !errors.Is(err, redis.Nil) {
			recordClientError(span, err)
		}
		span.End()
		return err
	}
}

func recordClientError(span trace.Span, err error) {
	if err == nil || span == nil {
		return
	}
	span.RecordError(err)
	span.SetStatus(codes.Error, "dependency operation failed")
}
