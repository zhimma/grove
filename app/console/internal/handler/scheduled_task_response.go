package handler

import (
	"time"

	"github.com/zhimma/grove/internal/model"
)

type ScheduledTaskItem struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	DisplayName    string `json:"display_name"`
	Schedule       string `json:"schedule"`
	Enabled        bool   `json:"enabled"`
	Mutex          bool   `json:"mutex"`
	TimeoutSeconds int    `json:"timeout_seconds"`
	RunRequestedAt string `json:"run_requested_at"`
	LastRunAt      string `json:"last_run_at"`
	LastStatus     string `json:"last_status"`
	LastError      string `json:"last_error"`
	LastDurationMS int64  `json:"last_duration_ms"`
	UpdatedAt      string `json:"updated_at"`
}

func toScheduledTaskItem(task model.ConsoleScheduledTask) ScheduledTaskItem {
	return ScheduledTaskItem{
		ID:             task.ID,
		Name:           task.Name,
		DisplayName:    task.DisplayName,
		Schedule:       task.Schedule,
		Enabled:        task.Enabled,
		Mutex:          task.Mutex,
		TimeoutSeconds: task.TimeoutSeconds,
		RunRequestedAt: formatOptionalTime(task.RunRequestedAt),
		LastRunAt:      formatOptionalTime(task.LastRunAt),
		LastStatus:     task.LastStatus,
		LastError:      task.LastError,
		LastDurationMS: task.LastDurationMS,
		UpdatedAt:      task.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
}

func formatOptionalTime(value *time.Time) string {
	if value == nil || value.IsZero() {
		return ""
	}
	return value.Format("2006-01-02 15:04:05")
}
