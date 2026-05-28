package model

import (
	"context"
	stderrors "errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/zhimma/grove/pkg/errx"
)

type User struct {
	Base
	Name  string `gorm:"size:120;not null" json:"name"`
	Email string `gorm:"size:160;uniqueIndex;not null" json:"email"`
}

func (User) TableName() string {
	return "users"
}

func FindUserByID(ctx context.Context, db *gorm.DB, userID string) (*User, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, errx.Unauthorized().WithMessage("缺少用户身份信息")
	}

	if db == nil {
		return &User{
			Base:  Base{ID: userID},
			Name:  "API User",
			Email: fmt.Sprintf("%s@example.com", userID),
		}, nil
	}

	var user User
	if err := db.WithContext(ctx).Where("id = ?", userID).First(&user).Error; err != nil {
		if stderrors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errx.NotFound().WithMessage("用户不存在")
		}
		return nil, errx.Internal().WithCause(err)
	}

	return &user, nil
}
