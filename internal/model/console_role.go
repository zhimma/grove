package model

import (
	"gorm.io/gorm"

	"github.com/zhimma/grove/internal/datatype"
)

type ConsoleRole struct {
	Base
	Name        string               `gorm:"size:120;not null" json:"name"`
	Code        string               `gorm:"size:120;uniqueIndex;not null" json:"code"`
	DisplayName string               `gorm:"size:120" json:"display_name"`
	Description string               `gorm:"size:255" json:"description"`
	MenuKeys    datatype.StringArray `gorm:"type:jsonb;default:'[]'" json:"menu_keys"`
	IsSuper     bool                 `gorm:"not null;default:false" json:"is_super"`
	Status      int                  `gorm:"not null;default:1" json:"status"`
	Sort        int                  `gorm:"not null;default:0" json:"sort"`
}

func (ConsoleRole) TableName() string {
	return "console_roles"
}

func (r *ConsoleRole) BeforeCreate(tx *gorm.DB) error {
	if err := r.Base.BeforeCreate(tx); err != nil {
		return err
	}
	if r.DisplayName == "" {
		r.DisplayName = r.Name
	}
	return nil
}

func (r *ConsoleRole) IsActive() bool {
	if r == nil {
		return false
	}
	return r.Status == ConsoleRoleStatusActive
}

// GetMenuKeys 获取菜单键列表
func (r *ConsoleRole) GetMenuKeys() []string {
	if r == nil {
		return []string{}
	}
	return r.MenuKeys
}

const (
	ConsoleRoleStatusDisabled = 0
	ConsoleRoleStatusActive   = 1
)
