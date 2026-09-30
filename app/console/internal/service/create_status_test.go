package service

import (
	"context"
	"net/http"
	"testing"

	"github.com/zhimma/grove/internal/model"
	"github.com/zhimma/grove/internal/testkit"
	"github.com/zhimma/grove/pkg/database"
	"github.com/zhimma/grove/pkg/errx"
	"github.com/zhimma/grove/pkg/pagination"
)

func TestCreateRolePreservesExplicitStatus(t *testing.T) {
	for _, test := range []struct {
		name   string
		status *int
		want   int
	}{
		{name: "default", want: model.ConsoleRoleStatusActive},
		{name: "disabled", status: ptrInt(model.ConsoleRoleStatusDisabled), want: model.ConsoleRoleStatusDisabled},
		{name: "active", status: ptrInt(model.ConsoleRoleStatusActive), want: model.ConsoleRoleStatusActive},
	} {
		t.Run(test.name, func(t *testing.T) {
			db := testkit.OpenDB(t, &model.ConsoleRole{})
			svc := NewRoleService(database.NewConnectionsFromDBs(db, nil), nil, nil, pagination.Policy{})
			role, err := svc.CreateRole(context.Background(), CreateRoleInput{Name: "测试角色", Code: "test", Status: test.status})
			if err != nil {
				t.Fatal(err)
			}
			var stored model.ConsoleRole
			if err := db.First(&stored, "id = ?", role.ID).Error; err != nil {
				t.Fatal(err)
			}
			if role.Status != test.want || stored.Status != test.want {
				t.Fatalf("returned=%d stored=%d, want %d", role.Status, stored.Status, test.want)
			}
			if _, err := svc.CreateRole(context.Background(), CreateRoleInput{Name: "非法状态", Code: "invalid", Status: ptrInt(9)}); errx.EffectiveStatus(errx.Normalize(err)) != http.StatusUnprocessableEntity {
				t.Fatalf("invalid status error: %v", err)
			}
		})
	}
}

func TestCreateAdminPreservesExplicitStatus(t *testing.T) {
	for _, test := range []struct {
		name   string
		status *int
		want   int
	}{
		{name: "default", want: model.ConsoleAdminStatusActive},
		{name: "disabled", status: ptrInt(model.ConsoleAdminStatusDisabled), want: model.ConsoleAdminStatusDisabled},
		{name: "active", status: ptrInt(model.ConsoleAdminStatusActive), want: model.ConsoleAdminStatusActive},
		{name: "locked", status: ptrInt(model.ConsoleAdminStatusLocked), want: model.ConsoleAdminStatusLocked},
	} {
		t.Run(test.name, func(t *testing.T) {
			dbs, db, roleID, _ := openAdminRBACTestContext(t)
			svc := &AdminService{dbs: dbs, roleBindings: &failingAdminRoleBindings{}}
			admin, err := svc.CreateAdmin(context.Background(), CreateAdminInput{
				Account: "operator", Password: "password123", RoleID: roleID, Status: test.status,
			})
			if err != nil {
				t.Fatal(err)
			}
			var stored model.ConsoleAdmin
			if err := db.First(&stored, "id = ?", admin.ID).Error; err != nil {
				t.Fatal(err)
			}
			if admin.Status != test.want || stored.Status != test.want {
				t.Fatalf("returned=%d stored=%d, want %d", admin.Status, stored.Status, test.want)
			}
			if _, err := svc.CreateAdmin(context.Background(), CreateAdminInput{
				Account: "invalid", Password: "password123", RoleID: roleID, Status: ptrInt(9),
			}); errx.EffectiveStatus(errx.Normalize(err)) != http.StatusUnprocessableEntity {
				t.Fatalf("invalid status error: %v", err)
			}
		})
	}
}

func TestCreateConfigPreservesReadOnly(t *testing.T) {
	svc, _ := newSystemConfigSecretService(t, false)
	ctx := context.Background()
	config, err := svc.CreateConfig(ctx, CreateSystemConfigInput{
		ConfigGroup: "test", ConfigKey: "read_only", Value: "original", IsEditable: ptrBool(false),
	})
	if err != nil {
		t.Fatal(err)
	}
	if config.IsEditable {
		t.Fatal("explicit read-only config became editable")
	}
	if _, err := svc.UpdateConfigByID(ctx, UpdateSystemConfigByIDInput{ID: config.ID, Value: "changed"}); errx.EffectiveStatus(errx.Normalize(err)) != http.StatusForbidden {
		t.Fatalf("read-only update error: %v", err)
	}
	for _, flag := range []*bool{nil, ptrBool(true)} {
		key := "default_editable"
		if flag != nil {
			key = "explicit_editable"
		}
		config, err := svc.CreateConfig(ctx, CreateSystemConfigInput{ConfigGroup: "test", ConfigKey: key, IsEditable: flag})
		if err != nil {
			t.Fatal(err)
		}
		if !config.IsEditable {
			t.Fatalf("%s is not editable", key)
		}
	}
}

func ptrBool(value bool) *bool { return &value }
