package model

import (
	"context"
	stderrors "errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/zhimma/grove/pkg/errx"
)

type User struct {
	Base
	Name        string     `gorm:"size:120;not null" json:"name"`
	Email       string     `gorm:"size:160;not null" json:"email"`
	Phone       string     `gorm:"size:32;not null;default:'';index" json:"phone"`
	Avatar      string     `gorm:"size:255;not null;default:''" json:"avatar"`
	Status      int        `gorm:"not null;index" json:"status"`
	Remark      string     `gorm:"size:500;not null;default:''" json:"remark"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
}

func (User) TableName() string {
	return "users"
}

const (
	UserStatusDisabled = 0
	UserStatusActive   = 1
)

func (u User) CanAccess() bool {
	return u.Status == UserStatusActive
}

func FindUserByID(ctx context.Context, db *gorm.DB, userID string) (*User, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, errx.Unauthorized().WithMessage("缺少用户身份信息")
	}

	if db == nil {
		return nil, errx.ServiceUnavailable().WithMessage("默认数据库未配置")
	}

	var user User
	if err := db.WithContext(ctx).Where("id = ?", userID).First(&user).Error; err != nil {
		if stderrors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errx.NotFound().WithMessage("用户不存在")
		}
		return nil, errx.Internal().WithCause(err)
	}
	if !user.CanAccess() {
		return nil, errx.Forbidden().WithCode("user_disabled").WithMessage("用户已停用")
	}

	return &user, nil
}
