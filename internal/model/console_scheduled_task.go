package model

import "time"

// Scheduled task run outcomes, written back by the Worker after each execution.
const (
	ScheduledTaskStatusSuccess = "success"
	ScheduledTaskStatusFailed  = "failed"
	ScheduledTaskStatusSkipped = "skipped"
)

// ConsoleScheduledTask holds the schedule for one task, never the task itself.
// The body lives in the Worker's code registry keyed by Name, so a row without
// a matching handler simply has nothing to run — the table cannot introduce
// work the binary does not already contain.
type ConsoleScheduledTask struct {
	AuditBase
	Name        string `gorm:"size:120;not null;uniqueIndex" json:"name"`
	DisplayName string `gorm:"size:160;not null;default:''" json:"display_name"`
	Schedule    string `gorm:"size:120;not null" json:"schedule"`
	// No GORM default tag on the booleans: with one, GORM omits a false value
	// from the INSERT and the column default silently turns it back into true.
	// The SQL migration still carries DEFAULT TRUE for rows written outside GORM.
	Enabled        bool `gorm:"not null;index" json:"enabled"`
	Mutex          bool `gorm:"not null" json:"mutex"`
	TimeoutSeconds int  `gorm:"not null;default:0" json:"timeout_seconds"`
	// RunRequestedAt is set by Console to ask for one off-schedule run. The
	// Worker clears it once it has taken the request, so a request runs once.
	RunRequestedAt *time.Time `gorm:"index" json:"run_requested_at,omitempty"`
	LastRunAt      *time.Time `json:"last_run_at,omitempty"`
	LastStatus     string     `gorm:"size:20;not null;default:''" json:"last_status"`
	LastError      string     `gorm:"size:1000;not null;default:''" json:"last_error"`
	LastDurationMS int64      `gorm:"not null;default:0" json:"last_duration_ms"`
}

func (ConsoleScheduledTask) TableName() string {
	return "console_scheduled_tasks"
}

// Timeout reports the per-run deadline. Zero means no deadline.
func (t ConsoleScheduledTask) Timeout() time.Duration {
	if t.TimeoutSeconds <= 0 {
		return 0
	}
	return time.Duration(t.TimeoutSeconds) * time.Second
}

// HasPendingRunRequest reports whether Console asked for an off-schedule run
// that the Worker has not consumed yet.
func (t ConsoleScheduledTask) HasPendingRunRequest() bool {
	return t.RunRequestedAt != nil
}
