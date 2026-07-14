package handler

import consoleservice "github.com/zhimma/grove/app/console/internal/service"

type RoleResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Code        string `json:"code"`
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
	Sort        int    `json:"sort"`
	Status      int    `json:"status"`
	StatusText  string `json:"status_text"`
	IsSuper     bool   `json:"is_super"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

func newRoleResponse(role consoleservice.Role) RoleResponse {
	return RoleResponse{
		ID:          role.ID,
		Name:        role.Name,
		Code:        role.Code,
		DisplayName: role.DisplayName,
		Description: role.Description,
		Sort:        role.Sort,
		Status:      role.Status,
		StatusText:  roleStatusToText(role.Status),
		IsSuper:     role.IsSuper,
		CreatedAt:   role.CreatedAt,
		UpdatedAt:   role.UpdatedAt,
	}
}

func roleStatusToText(status int) string {
	switch status {
	case 1:
		return "启用"
	case 0:
		return "禁用"
	default:
		return "未知"
	}
}
