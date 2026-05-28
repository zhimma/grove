package model

import (
	"time"

	"gorm.io/gorm"

	"github.com/zhimma/grove/pkg/ulid"
)

type Base struct {
	ID        string     `gorm:"primaryKey;type:varchar(26)" json:"id"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `gorm:"index" json:"deleted_at,omitempty"`
}

func (b *Base) BeforeCreate(tx *gorm.DB) error {
	if b.ID == "" {
		b.ID = ulid.New()
	}
	return nil
}
