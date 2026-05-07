package model

type ConsoleLoginLog struct {
	Base
	AdminID       string        `gorm:"size:26;index" json:"admin_id"`
	Account       string        `gorm:"size:120;index;not null" json:"account"`
	Success       bool          `gorm:"not null;default:false" json:"success"`
	FailureReason string        `gorm:"size:255" json:"failure_reason"`
	RequestID     string        `gorm:"size:120;index" json:"request_id"`
	ClientIP      string        `gorm:"size:64" json:"client_ip"`
	UserAgent     string        `gorm:"size:500" json:"user_agent"`
	Operator      *ConsoleAdmin `gorm:"foreignKey:AdminID" json:"operator,omitempty"`
}

func (ConsoleLoginLog) TableName() string {
	return "console_login_logs"
}
