package job

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/hibiken/asynq"
)

type Client struct {
	client *asynq.Client
}

type Server struct {
	server *asynq.Server
	mux    *asynq.ServeMux
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
	return &Client{client: asynq.NewClient(opt)}
}

func (c *Client) Enqueue(ctx context.Context, taskType string, payload interface{}, opts ...asynq.Option) (string, error) {
	if c == nil || c.client == nil {
		return "", errors.New("任务客户端未初始化")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	info, err := c.client.EnqueueContext(ctx, asynq.NewTask(taskType, body), opts...)
	if err != nil {
		return "", err
	}
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
	return &Server{
		server: asynq.NewServer(opt, asynq.Config{
			Concurrency: cfg.Concurrency,
			Queues:      cfg.Queues,
		}),
		mux: asynq.NewServeMux(),
	}
}

func (s *Server) Register(taskType string, handler func(context.Context, *asynq.Task) error) {
	if s == nil || s.mux == nil {
		return
	}
	s.mux.HandleFunc(taskType, handler)
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

func ParsePayload(task *asynq.Task, target interface{}) error {
	return json.Unmarshal(task.Payload(), target)
}
