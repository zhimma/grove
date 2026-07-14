package model

import "time"

type ConsoleSession struct {
	AuditBase
	AdminID          string        `gorm:"size:64;not null;index" json:"admin_id"`
	Admin            *ConsoleAdmin `gorm:"foreignKey:AdminID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"admin,omitempty"`
	RefreshTokenHash string        `gorm:"type:char(64);not null;uniqueIndex" json:"-"`
	DeviceName       string        `gorm:"size:120;not null;default:''" json:"device_name"`
	ClientIP         string        `gorm:"size:64;not null;default:''" json:"client_ip"`
	UserAgent        string        `gorm:"size:500;not null;default:''" json:"user_agent"`
	LastActiveAt     time.Time     `gorm:"not null;index" json:"last_active_at"`
	ExpiresAt        time.Time     `gorm:"not null;index" json:"expires_at"`
	RevokedAt        *time.Time    `gorm:"index" json:"revoked_at,omitempty"`
	RevokeReason     string        `gorm:"size:255;not null;default:''" json:"revoke_reason"`
}

func (ConsoleSession) TableName() string {
	return "console_sessions"
}

func (s ConsoleSession) ActiveAt(now time.Time) bool {
	return s.RevokedAt == nil && s.ExpiresAt.After(now)
}
