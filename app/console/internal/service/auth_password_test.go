package service

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/zhimma/grove/internal/model"
	"github.com/zhimma/grove/pkg/database"
)

func TestChangePasswordClearsMustChangePassword(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/auth-password.db"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.ConsoleAdmin{}); err != nil {
		t.Fatalf("migrate console admin: %v", err)
	}

	oldHash, err := bcrypt.GenerateFromPassword([]byte("old-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash old password: %v", err)
	}
	admin := model.ConsoleAdmin{
		Base:               model.Base{ID: "console-admin-root"},
		Account:            "root",
		Password:           string(oldHash),
		MustChangePassword: true,
		Status:             model.ConsoleAdminStatusActive,
	}
	if err := db.Create(&admin).Error; err != nil {
		t.Fatalf("create admin: %v", err)
	}

	repo := database.NewRepoWithConnections(db, nil)
	service := NewAuthService(repo, nil, nil)
	if err := service.ChangePassword(context.Background(), ChangePasswordInput{
		AdminID:     admin.ID,
		OldPassword: "old-password",
		NewPassword: "new-password",
	}); err != nil {
		t.Fatalf("change password: %v", err)
	}

	var updated model.ConsoleAdmin
	if err := db.First(&updated, "id = ?", admin.ID).Error; err != nil {
		t.Fatalf("load updated admin: %v", err)
	}
	if updated.MustChangePassword {
		t.Fatal("must_change_password should be cleared after self-service password change")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(updated.Password), []byte("new-password")); err != nil {
		t.Fatalf("new password was not persisted: %v", err)
	}
}
