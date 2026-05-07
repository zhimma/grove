package model

type ConsoleOperationLog struct {
	Base
	AdminID      string        `gorm:"size:26;index" json:"admin_id"`
	Method       string        `gorm:"size:12;index;not null" json:"method"`
	Path         string        `gorm:"size:255;index;not null" json:"path"`
	Route        string        `gorm:"size:255;index;not null" json:"route"`
	Module       string        `gorm:"size:120;index;not null" json:"module"`
	Action       string        `gorm:"size:180;not null" json:"action"`
	TargetType   string        `gorm:"size:120;index" json:"target_type"`
	TargetID     string        `gorm:"size:64;index" json:"target_id"`
	RequestID    string        `gorm:"size:120;index" json:"request_id"`
	StatusCode   int           `gorm:"not null;default:200" json:"status_code"`
	Success      bool          `gorm:"not null;default:true" json:"success"`
	ErrorMessage string        `gorm:"size:500" json:"error_message"`
	DurationMS   int64         `gorm:"not null;default:0" json:"duration_ms"`
	ClientIP     string        `gorm:"size:64" json:"client_ip"`
	UserAgent    string        `gorm:"size:500" json:"user_agent"`
	RequestQuery string        `gorm:"type:text" json:"request_query"`
	DetailJSON   string        `gorm:"type:text" json:"detail_json"`
	Operator     *ConsoleAdmin `gorm:"foreignKey:AdminID" json:"operator,omitempty"`
}

func (ConsoleOperationLog) TableName() string {
	return "console_operation_logs"
}
