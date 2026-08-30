package handler

import (
	"time"

	"github.com/zhimma/grove/internal/model"
)

type UserResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Email       string `json:"email"`
	Phone       string `json:"phone"`
	Avatar      string `json:"avatar"`
	Status      int    `json:"status"`
	StatusText  string `json:"status_text"`
	Remark      string `json:"remark"`
	LastLoginAt string `json:"last_login_at,omitempty"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

func newUserResponse(user *model.User) UserResponse {
	if user == nil {
		return UserResponse{}
	}
	return UserResponse{
		ID:          user.ID,
		Name:        user.Name,
		Email:       user.Email,
		Phone:       user.Phone,
		Avatar:      user.Avatar,
		Status:      user.Status,
		StatusText:  userStatusToText(user.Status),
		Remark:      user.Remark,
		LastLoginAt: formatNullableTime(user.LastLoginAt),
		CreatedAt:   user.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:   user.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
}

func formatNullableTime(value *time.Time) string {
	if value == nil || value.IsZero() {
		return ""
	}
	return value.Format("2006-01-02 15:04:05")
}

func userStatusToText(status int) string {
	if status == model.UserStatusActive {
		return "启用"
	}
	if status == model.UserStatusDisabled {
		return "停用"
	}
	return "未知"
}
