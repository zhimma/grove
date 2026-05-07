package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	pkgerrors "github.com/zhimma/grove/pkg/errors"
	"github.com/zhimma/grove/pkg/reqctx"
)

func TestFailIncludesErrorCodeInData(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/roles", nil)
	reqctx.SetRequestID(c, "req-1")

	Fail(c, pkgerrors.Conflict().WithCode("role_code_exists").WithMessage("角色编码已存在"))

	if recorder.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d", recorder.Code)
	}

	var payload Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if payload.Code != -1 {
		t.Fatalf("expected code -1, got %d", payload.Code)
	}
	if payload.Message != "角色编码已存在" {
		t.Fatalf("unexpected message: %s", payload.Message)
	}

	data, ok := payload.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected object data, got %#v", payload.Data)
	}
	if got := data["error_code"]; got != "role_code_exists" {
		t.Fatalf("unexpected error_code: %#v", got)
	}
}

func TestFailPreservesValidationErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/roles", nil)
	reqctx.SetRequestID(c, "req-2")

	Fail(c, pkgerrors.InvalidParams().WithHTTPStatus(http.StatusUnprocessableEntity).WithMessage("请求参数校验失败").WithData(map[string]interface{}{
		"errors": map[string][]string{
			"code": {"角色编码不能为空"},
		},
	}))

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected status 422, got %d", recorder.Code)
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	data := payload["data"].(map[string]any)
	if got := data["error_code"]; got != "invalid_params" {
		t.Fatalf("unexpected error_code: %#v", got)
	}
	errorsMap := data["errors"].(map[string]any)
	if _, ok := errorsMap["code"]; !ok {
		t.Fatalf("expected field errors, got %#v", errorsMap)
	}
}
