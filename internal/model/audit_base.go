package model

import (
	"time"

	"gorm.io/gorm"

	"github.com/zhimma/grove/pkg/ulid"
)

type AuditBase struct {
	ID        string    `gorm:"primaryKey;type:varchar(26)" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (b *AuditBase) BeforeCreate(tx *gorm.DB) error {
	if b.ID == "" {
		b.ID = ulid.New()
	}
	return nil
}
