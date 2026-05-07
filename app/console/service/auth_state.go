package service

import (
	"context"

	"gorm.io/gorm"

	"github.com/zhimma/grove/internal/model"
	"github.com/zhimma/grove/pkg/database"
	pkgerrors "github.com/zhimma/grove/pkg/errors"
)

type AdminAuthState struct {
	AdminID  string
	Username string
	RoleID   string
	IsSuper  bool
}

type AdminAuthStateResolver interface {
	ResolveAdminAuthState(ctx context.Context, adminID string) (*AdminAuthState, error)
}

type adminAuthStateResolver struct {
	dbRepo database.Repo
}

func NewAdminAuthStateResolver(dbRepo database.Repo) AdminAuthStateResolver {
	return &adminAuthStateResolver{dbRepo: dbRepo}
}

func (r *adminAuthStateResolver) ResolveAdminAuthState(ctx context.Context, adminID string) (*AdminAuthState, error) {
	var admin model.ConsoleAdmin
	if err := r.dbRepo.Default().WithContext(ctx).
		Preload("Role").
		Where("id = ?", adminID).
		First(&admin).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, pkgerrors.Unauthorized().WithMessage("未登录或登录已失效")
		}
		return nil, pkgerrors.Internal().WithCause(err)
	}

	if !admin.CanLogin() {
		return nil, pkgerrors.Forbidden().WithMessage("账号已被禁用").WithCode("account_disabled")
	}
	if admin.RoleID != "" && (admin.Role == nil || !admin.Role.IsActive()) {
		return nil, pkgerrors.Forbidden().WithMessage("角色不可用").WithCode("role_unavailable")
	}

	username := admin.Email
	if username == "" {
		username = admin.Username
	}
	if username == "" {
		username = admin.Account
	}

	return &AdminAuthState{
		AdminID:  admin.ID,
		Username: username,
		RoleID:   admin.RoleID,
		IsSuper:  admin.HasSuperAccess(),
	}, nil
}
