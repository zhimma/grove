package service

import (
	"context"
	"errors"
	"testing"

	"gorm.io/gorm"

	"github.com/zhimma/grove/internal/model"
	"github.com/zhimma/grove/internal/testkit"
	"github.com/zhimma/grove/pkg/database"
	"github.com/zhimma/grove/pkg/errx"
	"github.com/zhimma/grove/pkg/pagination"
	"github.com/zhimma/grove/pkg/transaction"
)

func TestServicesJoinCallerTransaction(t *testing.T) {
	for _, rollback := range []bool{false, true} {
		name := "commit"
		if rollback {
			name = "rollback"
		}
		t.Run(name, func(t *testing.T) {
			db := testkit.OpenDB(t, &model.User{}, &model.Article{}, &model.SystemConfig{})
			dbs := database.NewConnectionsFromDBs(db, nil)
			users := NewUserService(dbs, pagination.Policy{})
			articles := NewArticleService(dbs, pagination.Policy{})
			configs := NewSystemConfigService(dbs, nil, pagination.Policy{})
			abort := errors.New("abort workflow")
			err := db.Transaction(func(tx *gorm.DB) error {
				ctx := transaction.WithDB(context.Background(), tx)
				if _, err := users.CreateUser(ctx, CreateUserInput{Name: "测试用户", Email: "transaction@example.com"}); err != nil {
					return err
				}
				if _, err := articles.CreateArticle(ctx, CreateArticleInput{Title: "事务测试", Slug: "transaction", Content: "正文"}); err != nil {
					return err
				}
				if _, err := configs.CreateConfig(ctx, CreateSystemConfigInput{ConfigGroup: "test", ConfigKey: "transaction", Value: "value"}); err != nil {
					return err
				}
				list, err := users.ListUsers(ctx, ListUsersInput{})
				if err != nil {
					return err
				}
				if list.Meta.Total != 1 {
					t.Fatalf("service cannot read its uncommitted write: total=%d", list.Meta.Total)
				}
				if rollback {
					return abort
				}
				return nil
			})
			if rollback && !errors.Is(err, abort) || !rollback && err != nil {
				t.Fatalf("transaction: %v", err)
			}
			want := int64(1)
			if rollback {
				want = 0
			}
			for _, record := range []any{&model.User{}, &model.Article{}, &model.SystemConfig{}} {
				var count int64
				if err := db.Model(record).Count(&count).Error; err != nil {
					t.Fatal(err)
				}
				if count != want {
					t.Fatalf("%T count=%d, want %d", record, count, want)
				}
			}
		})
	}
}

func TestPermissionSideEffectsRejectCallerTransaction(t *testing.T) {
	dbs, db, roleID, _ := openAdminRBACTestContext(t)
	bindings := &failingAdminRoleBindings{}
	admins := &AdminService{dbs: dbs, roleBindings: bindings}
	roles := NewRoleService(dbs, nil, nil, pagination.Policy{})
	for name, run := range map[string]func(context.Context) error{
		"create admin": func(ctx context.Context) error {
			_, err := admins.CreateAdmin(ctx, CreateAdminInput{Account: "new-admin", Password: "password123", RoleID: roleID})
			return err
		},
		"delete admin": func(ctx context.Context) error { return admins.DeleteAdmin(ctx, DeleteAdminInput{AdminID: "unused"}) },
		"delete role":  func(ctx context.Context) error { return roles.DeleteRole(ctx, DeleteRoleInput{RoleID: roleID}) },
		"set permissions": func(ctx context.Context) error {
			return roles.SetRolePermissions(ctx, SetRolePermissionsInput{RoleID: roleID})
		},
		"login": func(ctx context.Context) error {
			_, err := NewAuthService(dbs, nil, nil).Login(ctx, LoginInput{Account: "operator", Password: "password123"})
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			if err := db.Transaction(func(tx *gorm.DB) error {
				err := run(transaction.WithDB(context.Background(), tx))
				if errx.EffectiveCode(errx.Normalize(err)) != "transaction_not_supported" {
					t.Fatalf("unexpected error: %v", err)
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		})
	}
	if len(bindings.calls) != 0 {
		t.Fatalf("permissions changed before rejecting the transaction: %v", bindings.calls)
	}
}
