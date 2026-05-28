package response

import (
	"encoding/json"
	stderrors "errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/zhimma/grove/pkg/errx"
	"github.com/zhimma/grove/pkg/request"
)

func TestFailIncludesErrorCodeInData(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/roles", nil)
	request.SetRequestID(c, "req-1")

	Fail(c, errx.Conflict().WithCode("role_code_exists").WithMessage("角色编码已存在"))

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
	request.SetRequestID(c, "req-2")

	Fail(c, errx.InvalidParams().WithHTTPStatus(http.StatusUnprocessableEntity).WithMessage("请求参数校验失败").WithData(map[string]interface{}{
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

func TestFailHidesInternalCauseWhenDebugDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/admins", nil)
	request.SetRequestID(c, "req-3")
	request.SetRequestMeta(c, request.RequestMeta{RequestID: "req-3", Debug: false})

	Fail(c, errx.Internal().WithMessage("database exploded").WithCause(stderrors.New("pq: duplicate key")))

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", recorder.Code)
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got := payload["message"]; got != "系统繁忙，请稍后再试" {
		t.Fatalf("expected generic message, got %#v", got)
	}
	data := payload["data"].(map[string]any)
	if _, exists := data["debug"]; exists {
		t.Fatalf("expected debug data to be hidden, got %#v", data["debug"])
	}

	meta := request.GetErrorMeta(c)
	if meta.HTTPStatus != http.StatusInternalServerError {
		t.Fatalf("unexpected error meta status: %#v", meta)
	}
	if meta.Code != "internal_error" || meta.Message != "系统繁忙，请稍后再试" {
		t.Fatalf("unexpected error meta: %#v", meta)
	}
	if !meta.InternalError || !meta.HasCause {
		t.Fatalf("expected internal cause meta, got %#v", meta)
	}
}

func TestFailIncludesDebugCauseWhenDebugEnabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/admins", nil)
	request.SetRequestID(c, "req-4")
	request.SetRequestMeta(c, request.RequestMeta{RequestID: "req-4", Debug: true})

	Fail(c, stderrors.New("driver: connection refused"))

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", recorder.Code)
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got := payload["message"]; got != "系统繁忙，请稍后再试" {
		t.Fatalf("expected generic message, got %#v", got)
	}
	data := payload["data"].(map[string]any)
	debug := data["debug"].(map[string]any)
	if got := debug["error"]; got != "driver: connection refused" {
		t.Fatalf("unexpected debug error: %#v", got)
	}
	if got := debug["type"]; got != "*errors.errorString" {
		t.Fatalf("unexpected debug type: %#v", got)
	}
}
