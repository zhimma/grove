package model

import (
	"time"

	"gorm.io/gorm"
)

type ConsoleAdmin struct {
	Base
	Account       string       `gorm:"size:120;uniqueIndex;not null" json:"account"`
	Username      string       `gorm:"size:120" json:"username"`
	Email         string       `gorm:"size:160" json:"email"`
	Phone         string       `gorm:"size:32" json:"phone"`
	Password      string       `gorm:"size:255;not null" json:"-"`
	RealName      string       `gorm:"size:120" json:"real_name"`
	DisplayName   string       `gorm:"size:120" json:"display_name"`
	Avatar        string       `gorm:"size:255" json:"avatar"`
	RoleID        string       `gorm:"size:26;index" json:"role_id"`
	Status        int          `gorm:"not null;default:1" json:"status"`
	EmailVerified bool         `gorm:"not null;default:false" json:"email_verified"`
	PhoneVerified bool         `gorm:"not null;default:false" json:"phone_verified"`
	LastLoginAt   *time.Time   `json:"last_login_at,omitempty"`
	LastLoginIP   string       `gorm:"size:64" json:"last_login_ip"`
	LoginCount    int          `gorm:"not null;default:0" json:"login_count"`
	Remark        string       `gorm:"size:500" json:"remark"`
	Role          *ConsoleRole `gorm:"foreignKey:RoleID" json:"role,omitempty"`
}

func (ConsoleAdmin) TableName() string {
	return "console_admins"
}

func (a ConsoleAdmin) CanLogin() bool {
	return a.Status == ConsoleAdminStatusActive
}

func (a ConsoleAdmin) HasSuperAccess() bool {
	return a.Role != nil && a.Role.IsSuper
}

func (a *ConsoleAdmin) BeforeCreate(tx *gorm.DB) error {
	if err := a.Base.BeforeCreate(tx); err != nil {
		return err
	}
	if a.DisplayName == "" {
		switch {
		case a.RealName != "":
			a.DisplayName = a.RealName
		case a.Username != "":
			a.DisplayName = a.Username
		default:
			a.DisplayName = a.Account
		}
	}
	return nil
}

func (a ConsoleAdmin) GetDisplayName() string {
	if a.DisplayName != "" {
		return a.DisplayName
	}
	if a.RealName != "" {
		return a.RealName
	}
	if a.Username != "" {
		return a.Username
	}
	return a.Account
}

const (
	ConsoleAdminStatusDisabled = 0
	ConsoleAdminStatusActive   = 1
	ConsoleAdminStatusLocked   = 2
)
