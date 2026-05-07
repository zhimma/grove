package job

import (
	"context"
	"testing"

	"github.com/hibiken/asynq"
)

func TestClientEnqueueRequiresInitializedClient(t *testing.T) {
	var client *Client
	if _, err := client.Enqueue(context.Background(), TaskEcho, EchoPayload{Message: "hello"}); err == nil {
		t.Fatal("expected enqueue error for nil client")
	}
}

func TestServerRegisterAllowsNilServerState(t *testing.T) {
	server := NewServer(RedisConfig{}, ServerConfig{})
	if server == nil || server.mux == nil {
		t.Fatal("expected initialized server mux")
	}

	server.Register(TaskEcho, func(context.Context, *asynq.Task) error {
		return nil
	})
}

func TestParsePayloadDecodesJSON(t *testing.T) {
	task := asynq.NewTask(TaskEcho, []byte(`{"message":"hello"}`))

	var payload EchoPayload
	if err := ParsePayload(task, &payload); err != nil {
		t.Fatalf("parse payload failed: %v", err)
	}
	if payload.Message != "hello" {
		t.Fatalf("expected message hello, got %q", payload.Message)
	}
}
