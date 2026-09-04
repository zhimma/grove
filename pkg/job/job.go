package job

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

const tracePayloadKey = "_grove_trace"

type Client struct {
	client     *asynq.Client
	tracer     trace.Tracer
	propagator propagation.TextMapPropagator
}

type Server struct {
	server     *asynq.Server
	mux        *asynq.ServeMux
	tracer     trace.Tracer
	propagator propagation.TextMapPropagator
	executions metric.Int64Counter
	duration   metric.Float64Histogram
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type ServerConfig struct {
	Concurrency int
	Queues      map[string]int
}

func RedisOpt(cfg RedisConfig) asynq.RedisClientOpt {
	return asynq.RedisClientOpt{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	}
}

func NewClient(cfg RedisConfig) *Client {
	opt := RedisOpt(cfg)
	return &Client{
		client:     asynq.NewClient(opt),
		tracer:     otel.Tracer("github.com/zhimma/grove/job/client"),
		propagator: otel.GetTextMapPropagator(),
	}
}

func (c *Client) Enqueue(ctx context.Context, taskType string, payload any, opts ...asynq.Option) (string, error) {
	if c == nil || c.client == nil {
		return "", errors.New("任务客户端未初始化")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, span := c.tracer.Start(ctx, "asynq.enqueue "+taskType, trace.WithSpanKind(trace.SpanKindProducer))
	span.SetAttributes(attribute.String("messaging.system", "asynq"), attribute.String("messaging.destination.name", taskType))
	defer span.End()

	body, err := json.Marshal(payload)
	if err != nil {
		recordTelemetryError(span, err)
		return "", err
	}
	body = injectTraceContext(ctx, c.propagator, body)
	info, err := c.client.EnqueueContext(ctx, asynq.NewTask(taskType, body), opts...)
	if err != nil {
		recordTelemetryError(span, err)
		return "", err
	}
	span.SetAttributes(attribute.String("messaging.message.id", info.ID))
	return info.ID, nil
}

func (c *Client) Close() error {
	if c == nil || c.client == nil {
		return nil
	}
	return c.client.Close()
}

func NewServer(redisCfg RedisConfig, cfg ServerConfig) *Server {
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 10
	}
	if len(cfg.Queues) == 0 {
		cfg.Queues = map[string]int{
			"default": 1,
		}
	}

	opt := RedisOpt(redisCfg)
	meter := otel.Meter("github.com/zhimma/grove/job/server")
	executions, _ := meter.Int64Counter("job.executions", metric.WithDescription("Background job executions"))
	duration, _ := meter.Float64Histogram("job.duration", metric.WithUnit("s"), metric.WithDescription("Background job execution duration"))
	server := &Server{
		server: asynq.NewServer(opt, asynq.Config{
			Concurrency: cfg.Concurrency,
			Queues:      cfg.Queues,
		}),
		mux:        asynq.NewServeMux(),
		tracer:     otel.Tracer("github.com/zhimma/grove/job/server"),
		propagator: otel.GetTextMapPropagator(),
		executions: executions,
		duration:   duration,
	}
	server.mux.Use(server.telemetryMiddleware())
	return server
}

func (s *Server) Register(taskType string, handler func(context.Context, *asynq.Task) error) error {
	if s == nil || s.mux == nil {
		return errors.New("任务服务未初始化")
	}
	if taskType == "" {
		return errors.New("任务类型不能为空")
	}
	if handler == nil {
		return errors.New("任务处理器不能为空")
	}
	s.mux.HandleFunc(taskType, handler)
	return nil
}

func (s *Server) Run() error {
	if s == nil || s.server == nil || s.mux == nil {
		return errors.New("任务服务未初始化")
	}
	return s.server.Run(s.mux)
}

func (s *Server) Shutdown() {
	if s != nil && s.server != nil {
		s.server.Shutdown()
	}
}

func ParsePayload(task *asynq.Task, target any) error {
	return json.Unmarshal(task.Payload(), target)
}

func (s *Server) telemetryMiddleware() asynq.MiddlewareFunc {
	return func(next asynq.Handler) asynq.Handler {
		return asynq.HandlerFunc(func(ctx context.Context, task *asynq.Task) error {
			if ctx == nil {
				ctx = context.Background()
			}
			ctx = extractTraceContext(ctx, s.propagator, task.Payload())
			ctx, span := s.tracer.Start(ctx, "asynq.process "+task.Type(), trace.WithSpanKind(trace.SpanKindConsumer))
			span.SetAttributes(attribute.String("messaging.system", "asynq"), attribute.String("messaging.destination.name", task.Type()))
			startedAt := time.Now()
			err := next.ProcessTask(ctx, task)
			result := "success"
			if err != nil {
				result = "error"
				recordTelemetryError(span, err)
			}
			attrs := metric.WithAttributes(attribute.String("task", task.Type()), attribute.String("result", result))
			s.executions.Add(ctx, 1, attrs)
			s.duration.Record(ctx, time.Since(startedAt).Seconds(), attrs)
			span.End()
			return err
		})
	}
}

func injectTraceContext(ctx context.Context, propagator propagation.TextMapPropagator, body []byte) []byte {
	carrier := propagation.MapCarrier{}
	propagator.Inject(ctx, carrier)
	if len(carrier) == 0 {
		return body
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(body, &object); err != nil || object == nil {
		return body
	}
	tracePayload, err := json.Marshal(map[string]string(carrier))
	if err != nil {
		return body
	}
	object[tracePayloadKey] = tracePayload
	tracedBody, err := json.Marshal(object)
	if err != nil {
		return body
	}
	return tracedBody
}

func extractTraceContext(ctx context.Context, propagator propagation.TextMapPropagator, body []byte) context.Context {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(body, &object); err != nil || object == nil {
		return ctx
	}
	raw, ok := object[tracePayloadKey]
	if !ok {
		return ctx
	}
	carrier := propagation.MapCarrier{}
	if err := json.Unmarshal(raw, &carrier); err != nil {
		return ctx
	}
	return propagator.Extract(ctx, carrier)
}

func recordTelemetryError(span trace.Span, err error) {
	if span == nil || err == nil {
		return
	}
	span.RecordError(err)
	span.SetStatus(codes.Error, strings.TrimSpace(err.Error()))
}
