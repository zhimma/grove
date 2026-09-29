package service

import (
	"context"
	"net/http"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/zhimma/grove/internal/model"
	"github.com/zhimma/grove/pkg/database"
	"github.com/zhimma/grove/pkg/errx"
	"github.com/zhimma/grove/pkg/pagination"
)

func TestUserServiceCRUDAndStatus(t *testing.T) {
	dbs, db := openUserServiceDB(t)
	service := NewUserService(dbs, pagination.Policy{Default: 2, Max: 10})
	ctx := context.Background()

	created, err := service.CreateUser(ctx, CreateUserInput{
		Name:  "张三",
		Email: "zhangsan@example.com",
		Phone: "13800138000",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if created.ID == "" || created.Status != model.UserStatusActive {
		t.Fatalf("unexpected created user: %#v", created)
	}

	if _, err := service.CreateUser(ctx, CreateUserInput{Name: "Duplicate", Email: "zhangsan@example.com"}); errx.Normalize(err).Code != "user_email_exists" {
		t.Fatalf("duplicate email must be rejected: %v", err)
	}

	updatedName := "李四"
	updated, err := service.UpdateUser(ctx, UpdateUserInput{UserID: created.ID, Name: &updatedName})
	if err != nil {
		t.Fatalf("update user: %v", err)
	}
	if updated.Name != updatedName {
		t.Fatalf("updated name = %q, expected %q", updated.Name, updatedName)
	}

	disabled, err := service.UpdateUserStatus(ctx, UpdateUserStatusInput{UserID: created.ID, Status: model.UserStatusDisabled})
	if err != nil {
		t.Fatalf("disable user: %v", err)
	}
	if disabled.Status != model.UserStatusDisabled {
		t.Fatalf("user status = %d, expected disabled", disabled.Status)
	}

	list, err := service.ListUsers(ctx, ListUsersInput{Status: ptrInt(model.UserStatusDisabled)})
	if err != nil {
		t.Fatalf("list disabled users: %v", err)
	}
	if list.Meta.Total != 1 || len(list.List) != 1 || list.List[0].ID != created.ID {
		t.Fatalf("unexpected disabled user list: %#v", list)
	}

	if err := service.DeleteUser(ctx, DeleteUserInput{UserID: created.ID}); err != nil {
		t.Fatalf("delete user: %v", err)
	}
	if _, err := service.GetUser(ctx, GetUserInput{UserID: created.ID}); errx.Normalize(err).HTTPStatus != http.StatusNotFound {
		t.Fatalf("deleted user must be hidden: %v", err)
	}
	var total int64
	if err := db.Unscoped().Model(&model.User{}).Where("id = ?", created.ID).Count(&total).Error; err != nil || total != 1 {
		t.Fatalf("soft deleted user must remain auditable: total=%d err=%v", total, err)
	}
}

func TestUserServiceRejectsInvalidInput(t *testing.T) {
	dbs, _ := openUserServiceDB(t)
	service := NewUserService(dbs, pagination.Policy{})

	for _, input := range []CreateUserInput{
		{Email: "missing-name@example.com"},
		{Name: "Missing email"},
		{Name: "Bad email", Email: "not-an-email"},
	} {
		_, err := service.CreateUser(context.Background(), input)
		if err == nil || errx.Normalize(err) == nil || errx.Normalize(err).HTTPStatus != http.StatusUnprocessableEntity {
			t.Fatalf("invalid input must return 422: input=%#v err=%v", input, err)
		}
	}
}

func openUserServiceDB(t *testing.T) (database.Connections, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/users.db"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}); err != nil {
		t.Fatalf("migrate users: %v", err)
	}
	return database.NewConnectionsFromDBs(db, nil), db
}

func ptrInt(value int) *int {
	return &value
}
