package validation

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/zhimma/grove/pkg/errx"
)

type customValidatePayload struct {
	Name string `json:"name" binding:"required" label:"角色名称"`
}

func (p customValidatePayload) Validate() error {
	return Require(false, "角色名称与业务规则不匹配")
}

func TestBindJSONUsesLabelForValidationMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type createRoleRequest struct {
		Name string `json:"name" binding:"required" label:"角色名称"`
		Code string `json:"code" binding:"required" label:"角色编码"`
	}

	req := httptest.NewRequest(http.MethodPost, "/roles", bytes.NewBufferString(`{"name":"运营"}`))
	req.Header.Set("Content-Type", "application/json")

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req

	var payload createRoleRequest
	err := BindJSON(c, &payload)
	if err == nil {
		t.Fatal("expected validation error")
	}

	httpErr, ok := err.(*errx.HTTPError)
	if !ok {
		t.Fatalf("expected *errx.HTTPError, got %T", err)
	}
	if httpErr.HTTPStatus != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", httpErr.HTTPStatus)
	}

	errs, ok := httpErr.Data["errors"].(map[string][]string)
	if !ok {
		t.Fatalf("expected validation errors payload, got %#v", httpErr.Data["errors"])
	}

	if got := errs["code"]; len(got) != 1 || got[0] != "角色编码不能为空" {
		t.Fatalf("unexpected code errors: %#v", got)
	}
}

func TestBindJSONUsesLabelTag(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type listRequest struct {
		Page int `json:"page" binding:"required,min=2" label:"页码"`
	}

	req := httptest.NewRequest(http.MethodPost, "/list", bytes.NewBufferString(`{"page":1}`))
	req.Header.Set("Content-Type", "application/json")

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req

	var payload listRequest
	err := BindJSON(c, &payload)
	if err == nil {
		t.Fatal("expected validation error")
	}

	httpErr := err.(*errx.HTTPError)
	if httpErr.HTTPStatus != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", httpErr.HTTPStatus)
	}
	errs := httpErr.Data["errors"].(map[string][]string)
	if got := errs["page"]; len(got) != 1 || got[0] != "页码不能小于2" {
		t.Fatalf("unexpected page errors: %#v", got)
	}
}

func TestBindQueryUsesLabelForTypeError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type listRequest struct {
		Page int `form:"page" binding:"omitempty,min=1" label:"页码"`
	}

	req := httptest.NewRequest(http.MethodGet, "/roles?page=abc", nil)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req

	var payload listRequest
	err := BindQuery(c, &payload)
	if err == nil {
		t.Fatal("expected validation error")
	}

	httpErr := err.(*errx.HTTPError)
	if httpErr.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", httpErr.HTTPStatus)
	}
	errs := httpErr.Data["errors"].(map[string][]string)
	if got := errs["page"]; len(got) != 1 || got[0] != "页码格式不正确" {
		t.Fatalf("unexpected page errors: %#v", got)
	}
}

func TestBindJSONUsesBadRequestForJSONSyntaxError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type createRoleRequest struct {
		Name string `json:"name" binding:"required" label:"角色名称"`
	}

	req := httptest.NewRequest(http.MethodPost, "/roles", bytes.NewBufferString(`{"name":`))
	req.Header.Set("Content-Type", "application/json")

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req

	var payload createRoleRequest
	err := BindJSON(c, &payload)
	if err == nil {
		t.Fatal("expected validation error")
	}

	httpErr := err.(*errx.HTTPError)
	if httpErr.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", httpErr.HTTPStatus)
	}

	errs := httpErr.Data["errors"].(map[string][]string)
	if got := errs["_error"]; len(got) != 1 || got[0] != "请求体格式不正确" {
		t.Fatalf("unexpected syntax errors: %#v", got)
	}
}

func TestBindJSONUsesValidationStatusForCustomValidate(t *testing.T) {
	gin.SetMode(gin.TestMode)

	req := httptest.NewRequest(http.MethodPost, "/roles", bytes.NewBufferString(`{"name":"运营"}`))
	req.Header.Set("Content-Type", "application/json")

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req

	var payload customValidatePayload
	err := BindJSON(c, &payload)
	if err == nil {
		t.Fatal("expected validation error")
	}

	httpErr := err.(*errx.HTTPError)
	if httpErr.HTTPStatus != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", httpErr.HTTPStatus)
	}

	errs := httpErr.Data["errors"].(map[string][]string)
	if got := errs["_error"]; len(got) != 1 || got[0] != "角色名称与业务规则不匹配" {
		t.Fatalf("unexpected custom validation errors: %#v", got)
	}
}
