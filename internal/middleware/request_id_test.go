package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestRequestIDAcceptsSafeValueAndReplacesInvalidInput(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(RequestID())
	engine.GET("/", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	valid := httptest.NewRequest(http.MethodGet, "/", nil)
	valid.Header.Set("X-Request-Id", "client.req-123:abc")
	validResp := httptest.NewRecorder()
	engine.ServeHTTP(validResp, valid)
	if got := validResp.Header().Get("X-Request-Id"); got != "client.req-123:abc" {
		t.Fatalf("safe request id changed to %q", got)
	}

	for _, invalid := range []string{"bad\nrequest", strings.Repeat("a", 121), "含中文"} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Request-Id", invalid)
		resp := httptest.NewRecorder()
		engine.ServeHTTP(resp, req)
		generated := resp.Header().Get("X-Request-Id")
		if _, err := uuid.Parse(generated); err != nil {
			t.Fatalf("invalid request id %q was not replaced with UUID: %q", invalid, generated)
		}
	}
}
