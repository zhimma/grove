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

func TestBindQueryUsesLabelFromEmbeddedStruct(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type paginationQuery struct {
		Page int `form:"page" binding:"omitempty,min=1" label:"页码"`
	}
	type listRequest struct {
		paginationQuery
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

func TestBindJSONUsesPayloadTooLargeForMaxBytesError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type payload struct {
		Name string `json:"name"`
	}
	req := httptest.NewRequest(http.MethodPost, "/roles", bytes.NewBufferString(`{"name":"too large"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	req.Body = http.MaxBytesReader(recorder, req.Body, 4)

	c, _ := gin.CreateTestContext(recorder)
	c.Request = req
	var input payload
	err := BindJSON(c, &input)
	if err == nil {
		t.Fatal("expected payload-too-large error")
	}
	httpErr := errx.Normalize(err)
	if httpErr.HTTPStatus != http.StatusRequestEntityTooLarge || httpErr.Code != "request_body_too_large" {
		t.Fatalf("unexpected error: %#v", httpErr)
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

func TestBindJSONStrictRejectsUnknownField(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type payload struct {
		Name string `json:"name" label:"名称"`
	}
	req := httptest.NewRequest(http.MethodPost, "/roles", bytes.NewBufferString(`{"name":"运营","unexpected":true}`))
	req.Header.Set("Content-Type", "application/json")
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req

	var input payload
	err := BindJSONStrict(c, &input)
	if err == nil {
		t.Fatal("expected unknown-field validation error")
	}
	httpErr := errx.Normalize(err)
	if httpErr.HTTPStatus != http.StatusBadRequest || httpErr.Code != "invalid_params" {
		t.Fatalf("unexpected unknown-field error: %#v", httpErr)
	}
	errs, ok := httpErr.Data["errors"].(map[string][]string)
	if !ok || len(errs["_error"]) != 1 || errs["_error"][0] == "" {
		t.Fatalf("expected stable unknown-field message, got %#v", httpErr.Data["errors"])
	}
}

func TestBindJSONAllowsUnknownFieldWhenExplicitlyRequested(t *testing.T) {
	gin.SetMode(gin.TestMode)
	type payload struct {
		Name string `json:"name" label:"名称"`
	}
	req := httptest.NewRequest(http.MethodPost, "/roles", bytes.NewBufferString(`{"name":"运营","legacy":true}`))
	req.Header.Set("Content-Type", "application/json")
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req
	var input payload
	if err := BindJSONAllowUnknown(c, &input); err != nil {
		t.Fatalf("explicit compatibility binding failed: %v", err)
	}
	if input.Name != "运营" {
		t.Fatalf("unexpected decoded payload: %#v", input)
	}
}

func TestBindJSONStrictRejectsTrailingValue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	type payload struct {
		Name string `json:"name" label:"名称"`
	}
	req := httptest.NewRequest(http.MethodPost, "/roles", bytes.NewBufferString(`{"name":"运营"}{"name":"重复"}`))
	req.Header.Set("Content-Type", "application/json")
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req
	var input payload
	err := BindJSONStrict(c, &input)
	if err == nil || errx.Normalize(err).HTTPStatus != http.StatusBadRequest {
		t.Fatalf("expected trailing JSON 400, got %v", err)
	}
}

func TestBindURIUsesValidationStatusAndFieldLabel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	type path struct {
		ID int `uri:"id" binding:"required" label:"资源ID"`
	}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/roles/not-an-int", nil)
	c.Params = gin.Params{{Key: "id", Value: "not-an-int"}}
	var input path
	err := BindURI(c, &input)
	if err == nil {
		t.Fatal("expected URI type error")
	}
	httpErr := errx.Normalize(err)
	if httpErr.HTTPStatus != http.StatusBadRequest || httpErr.Code != "invalid_params" {
		t.Fatalf("unexpected URI error: %#v", httpErr)
	}
	errs := httpErr.Data["errors"].(map[string][]string)
	if got := errs["id"]; len(got) != 1 || got[0] != "资源ID格式不正确" {
		t.Fatalf("unexpected URI field errors: %#v", errs)
	}
}
