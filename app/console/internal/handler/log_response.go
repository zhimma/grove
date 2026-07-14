package handler

import "github.com/zhimma/grove/internal/model"

type OperationLogItem struct {
	ID           string `json:"id"`
	AdminID      string `json:"admin_id"`
	AdminAccount string `json:"admin_account"`
	AdminName    string `json:"admin_name"`
	Method       string `json:"method"`
	Path         string `json:"path"`
	Route        string `json:"route"`
	Module       string `json:"module"`
	Action       string `json:"action"`
	TargetType   string `json:"target_type"`
	TargetID     string `json:"target_id"`
	RequestID    string `json:"request_id"`
	StatusCode   int    `json:"status_code"`
	Success      bool   `json:"success"`
	ErrorMessage string `json:"error_message"`
	DurationMS   int64  `json:"duration_ms"`
	ClientIP     string `json:"client_ip"`
	UserAgent    string `json:"user_agent"`
	RequestQuery string `json:"request_query"`
	CreatedAt    string `json:"created_at"`
}

type OperationLogDetailResponse struct {
	Log    OperationLogItem `json:"log"`
	Detail map[string]any   `json:"detail"`
}

type LoginLogItem struct {
	ID            string `json:"id"`
	AdminID       string `json:"admin_id"`
	AdminAccount  string `json:"admin_account"`
	AdminName     string `json:"admin_name"`
	Account       string `json:"account"`
	Success       bool   `json:"success"`
	FailureReason string `json:"failure_reason"`
	RequestID     string `json:"request_id"`
	ClientIP      string `json:"client_ip"`
	UserAgent     string `json:"user_agent"`
	CreatedAt     string `json:"created_at"`
}

func toOperationLogItem(item model.ConsoleOperationLog) OperationLogItem {
	result := OperationLogItem{
		ID:           item.ID,
		AdminID:      item.AdminID,
		Method:       item.Method,
		Path:         item.Path,
		Route:        item.Route,
		Module:       item.Module,
		Action:       item.Action,
		TargetType:   item.TargetType,
		TargetID:     item.TargetID,
		RequestID:    item.RequestID,
		StatusCode:   item.StatusCode,
		Success:      item.Success,
		ErrorMessage: item.ErrorMessage,
		DurationMS:   item.DurationMS,
		ClientIP:     item.ClientIP,
		UserAgent:    item.UserAgent,
		RequestQuery: item.RequestQuery,
		CreatedAt:    item.CreatedAt.Format("2006-01-02 15:04:05"),
	}
	if item.Operator != nil {
		result.AdminAccount = item.Operator.Account
		result.AdminName = item.Operator.GetDisplayName()
	}
	return result
}

func toLoginLogItem(item model.ConsoleLoginLog) LoginLogItem {
	result := LoginLogItem{
		ID:            item.ID,
		AdminID:       item.AdminID,
		Account:       item.Account,
		Success:       item.Success,
		FailureReason: item.FailureReason,
		RequestID:     item.RequestID,
		ClientIP:      item.ClientIP,
		UserAgent:     item.UserAgent,
		CreatedAt:     item.CreatedAt.Format("2006-01-02 15:04:05"),
	}
	if item.Operator != nil {
		result.AdminAccount = item.Operator.Account
		result.AdminName = item.Operator.GetDisplayName()
	}
	return result
}
