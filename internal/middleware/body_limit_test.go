package middleware

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestBodyLimitRejectsKnownOversizedBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(BodyLimit(4))
	engine.POST("/", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString("12345"))
	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d: %s", resp.Code, resp.Body.String())
	}
	var payload struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Data["error_code"] != "request_body_too_large" {
		t.Fatalf("unexpected error payload: %#v", payload.Data)
	}
}

func TestBodyLimitWrapsUnknownLengthBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(BodyLimit(4))
	engine.POST("/", func(c *gin.Context) {
		_, err := io.ReadAll(c.Request.Body)
		var maxErr *http.MaxBytesError
		if !errors.As(err, &maxErr) {
			t.Fatalf("expected MaxBytesError, got %v", err)
		}
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString("12345"))
	req.ContentLength = -1
	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusNoContent {
		t.Fatalf("expected handler to observe wrapped body, got %d", resp.Code)
	}
}
