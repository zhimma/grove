package handler

import "github.com/zhimma/grove/internal/model"

type AdminResponse struct {
	ID                 string             `json:"id"`
	Account            string             `json:"account"`
	Username           string             `json:"username"`
	Email              string             `json:"email"`
	Phone              string             `json:"phone"`
	RealName           string             `json:"real_name"`
	DisplayName        string             `json:"display_name"`
	Avatar             string             `json:"avatar"`
	RoleID             string             `json:"role_id"`
	RoleName           string             `json:"role_name"`
	Role               *AdminRoleResponse `json:"role,omitempty"`
	Status             int                `json:"status"`
	StatusText         string             `json:"status_text"`
	EmailVerified      bool               `json:"email_verified"`
	PhoneVerified      bool               `json:"phone_verified"`
	MustChangePassword bool               `json:"must_change_password"`
	IsSuper            bool               `json:"is_super"`
	Remark             string             `json:"remark"`
	CreatedAt          string             `json:"created_at"`
	UpdatedAt          string             `json:"updated_at"`
}

type AdminRoleResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Code        string `json:"code"`
}

func newAdminResponse(admin *model.ConsoleAdmin) AdminResponse {
	if admin == nil {
		return AdminResponse{}
	}
	result := AdminResponse{
		ID:                 admin.ID,
		Account:            admin.Account,
		Username:           admin.Username,
		Email:              admin.Email,
		Phone:              admin.Phone,
		RealName:           admin.RealName,
		DisplayName:        admin.GetDisplayName(),
		Avatar:             admin.Avatar,
		RoleID:             admin.RoleID,
		Status:             admin.Status,
		StatusText:         adminStatusToText(admin.Status),
		EmailVerified:      admin.EmailVerified,
		PhoneVerified:      admin.PhoneVerified,
		MustChangePassword: admin.MustChangePassword,
		IsSuper:            admin.HasSuperAccess(),
		Remark:             admin.Remark,
		CreatedAt:          admin.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:          admin.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
	if admin.Role != nil {
		result.RoleName = admin.Role.Name
		result.Role = &AdminRoleResponse{
			ID:          admin.Role.ID,
			Name:        admin.Role.Name,
			DisplayName: admin.Role.DisplayName,
			Code:        admin.Role.Code,
		}
	}
	return result
}

func adminStatusToText(status int) string {
	switch status {
	case 1:
		return "启用"
	case 0:
		return "禁用"
	case 2:
		return "锁定"
	default:
		return "未知"
	}
}
