package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	consoleservice "github.com/zhimma/grove/app/console/internal/service"
	"github.com/zhimma/grove/internal/model"
	"github.com/zhimma/grove/pkg/validation"
)

func TestEmbeddedListQueryBinding(t *testing.T) {
	gin.SetMode(gin.TestMode)
	req := httptest.NewRequest(http.MethodGet,
		"/admins?page=2&page_size=25&offset=3&limit=4&list_all=true&keyword=ops&order_by=created_at+desc&created_from=2026-01-01&created_to=2026-01-31&role_id=role-1&status=1",
		nil,
	)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req

	var query ListAdminsRequest
	if err := validation.BindQuery(c, &query); err != nil {
		t.Fatalf("bind query: %v", err)
	}
	if query.Page != 2 || query.PageSize != 25 || query.Offset != 3 || query.Limit != 4 || !query.ListAll {
		t.Fatalf("unexpected pagination query: %#v", query.Request)
	}
	if query.Keyword != "ops" || len(query.OrderBy) != 1 || query.OrderBy[0] != "created_at desc" {
		t.Fatalf("unexpected list query: %#v", query.ListQuery)
	}
	if query.CreatedFrom != "2026-01-01" || query.CreatedTo != "2026-01-31" || query.RoleID != "role-1" || query.Status == nil || *query.Status != 1 {
		t.Fatalf("unexpected filters: %#v", query)
	}
}

func TestNewAdminResponseMapsListAndDetailFields(t *testing.T) {
	createdAt := time.Date(2026, 7, 14, 10, 20, 30, 0, time.Local)
	updatedAt := createdAt.Add(time.Hour)
	admin := &model.ConsoleAdmin{
		Base:               model.Base{ID: "admin-1", CreatedAt: createdAt, UpdatedAt: updatedAt},
		Account:            "root",
		DisplayName:        "Root Admin",
		RoleID:             "role-1",
		Status:             model.ConsoleAdminStatusActive,
		EmailVerified:      true,
		MustChangePassword: true,
		Role: &model.ConsoleRole{
			Base:        model.Base{ID: "role-1"},
			Name:        "root",
			DisplayName: "超级管理员",
			Code:        "root",
			IsSuper:     true,
		},
	}

	got := newAdminResponse(admin)
	if got.ID != admin.ID || got.DisplayName != "Root Admin" || got.RoleName != "root" || got.Role == nil || got.Role.DisplayName != "超级管理员" {
		t.Fatalf("unexpected admin response: %#v", got)
	}
	if !got.IsSuper || got.StatusText != "启用" || got.CreatedAt != "2026-07-14 10:20:30" || got.UpdatedAt != "2026-07-14 11:20:30" {
		t.Fatalf("unexpected derived admin fields: %#v", got)
	}
}

func TestResponseConvertersKeepHTTPModelsSeparate(t *testing.T) {
	role := newRoleResponse(consoleservice.Role{
		ID:        "role-1",
		Name:      "operator",
		Status:    model.ConsoleRoleStatusDisabled,
		CreatedAt: "2026-07-14 10:00:00",
		UpdatedAt: "2026-07-14 11:00:00",
	})
	if role.StatusText != "禁用" || role.UpdatedAt == "" {
		t.Fatalf("unexpected role response: %#v", role)
	}

	config := newSystemConfigItem(model.SystemConfig{
		Base:        model.Base{ID: "config-1"},
		ConfigGroup: "app",
		ConfigKey:   "name",
		Value:       "grove",
	})
	if config.ID != "config-1" || config.ConfigKey != "name" || config.Value != "grove" {
		t.Fatalf("unexpected system config response: %#v", config)
	}
}
