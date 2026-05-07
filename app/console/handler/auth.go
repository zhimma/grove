package handler

import (
	"github.com/gin-gonic/gin"

	consoleservice "github.com/zhimma/grove/app/console/service"
	"github.com/zhimma/grove/internal/provider"
	"github.com/zhimma/grove/pkg/reqctx"
	"github.com/zhimma/grove/pkg/response"
	"github.com/zhimma/grove/pkg/route"
	"github.com/zhimma/grove/pkg/validation"
)

type AuthHandler struct {
	authSvc *consoleservice.AuthService
}

type LoginRequest struct {
	Account  string `json:"account" binding:"required" label:"账号"`
	Password string `json:"password" binding:"required" label:"密码"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required" label:"刷新令牌"`
}

type LogoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required" label:"原密码"`
	NewPassword string `json:"new_password" binding:"required,min=8" label:"新密码"`
}

type UpdateProfileRequest struct {
	Account  string `json:"account" binding:"required" label:"账号"`
	Username string `json:"username" label:"用户名"`
	Email    string `json:"email" binding:"omitempty,email" label:"邮箱"`
	Phone    string `json:"phone" label:"手机号"`
	RealName string `json:"real_name" label:"真实姓名"`
	Nickname string `json:"nickname" label:"昵称"`
	Avatar   string `json:"avatar" label:"头像"`
}

type RoleSummary struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Code        string `json:"code"`
}

type AdminResponse struct {
	ID          string       `json:"id"`
	Account     string       `json:"account"`
	Username    string       `json:"username"`
	Email       string       `json:"email"`
	Phone       string       `json:"phone"`
	DisplayName string       `json:"display_name"`
	RealName    string       `json:"real_name"`
	Avatar      string       `json:"avatar"`
	RoleID      string       `json:"role_id"`
	Role        *RoleSummary `json:"role,omitempty"`
	IsSuper     bool         `json:"is_super"`
	Status      int          `json:"status"`
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

type LoginResponse struct {
	User  *AdminResponse `json:"user"`
	Token *TokenResponse `json:"token"`
}

type RefreshTokenResponse struct {
	Token *TokenResponse `json:"token"`
}

type AuthorizationOverviewResponse struct {
	APIPermissions []string `json:"api_permissions"`
	MenuKeys       []string `json:"menu_keys"`
}

func RegisterAuthRoutes(public, authed *gin.RouterGroup, p *provider.Provider) {
	h := &AuthHandler{
		authSvc: consoleservice.NewAuthService(p.DB, p.GetEnforcer("console"), p.TokenManager),
	}

	publicAuth := route.Wrap(public.Group("/auth"))
	{
		publicAuth.POST("/login", h.Login).Ignore()
		publicAuth.POST("/refresh", h.RefreshToken).Ignore()
	}

	authedAuth := route.Wrap(authed.Group("/auth"))
	{
		authedAuth.POST("/logout", h.Logout).Ignore()
		authedAuth.GET("/me", h.Me).Ignore()
		authedAuth.PUT("/me", h.UpdateMe).Ignore()
		authedAuth.PUT("/password", h.ChangePassword).Ignore()
		authedAuth.GET("/permissions", h.Permissions).Ignore()
	}
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := validation.BindJSON(c, &req); err != nil {
		response.Fail(c, err)
		return
	}

	out, err := h.authSvc.Login(c.Request.Context(), consoleservice.LoginInput{
		Account:  req.Account,
		Password: req.Password,
	})
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, LoginResponse{
		User: &AdminResponse{
			ID:          out.Admin.ID,
			Account:     out.Admin.Account,
			Username:    out.Admin.Username,
			Email:       out.Admin.Email,
			Phone:       out.Admin.Phone,
			DisplayName: out.Admin.GetDisplayName(),
			RealName:    out.Admin.RealName,
			Avatar:      out.Admin.Avatar,
			RoleID:      out.Admin.RoleID,
			Role: func() *RoleSummary {
				if out.Admin.Role == nil {
					return nil
				}
				return &RoleSummary{
					ID:          out.Admin.Role.ID,
					Name:        out.Admin.Role.Name,
					DisplayName: out.Admin.Role.DisplayName,
					Code:        out.Admin.Role.Code,
				}
			}(),
			IsSuper: out.Admin.HasSuperAccess(),
			Status:  out.Admin.Status,
		},
		Token: &TokenResponse{
			AccessToken:  out.Token.AccessToken,
			RefreshToken: out.Token.RefreshToken,
			ExpiresIn:    out.Token.ExpiresIn,
			TokenType:    out.Token.TokenType,
		},
	})
}

func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req RefreshTokenRequest
	if err := validation.BindJSON(c, &req); err != nil {
		response.Fail(c, err)
		return
	}

	out, err := h.authSvc.RefreshToken(c.Request.Context(), consoleservice.RefreshTokenInput{
		RefreshToken: req.RefreshToken,
	})
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, RefreshTokenResponse{
		Token: &TokenResponse{
			AccessToken:  out.Token.AccessToken,
			RefreshToken: out.Token.RefreshToken,
			ExpiresIn:    out.Token.ExpiresIn,
			TokenType:    out.Token.TokenType,
		},
	})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	var req LogoutRequest
	if c.Request.ContentLength > 0 {
		if err := validation.BindJSON(c, &req); err != nil {
			response.Fail(c, err)
			return
		}
	}

	if err := h.authSvc.Logout(c.Request.Context(), consoleservice.LogoutInput{
		AccessToken:  reqctx.GetAuthToken(c),
		RefreshToken: req.RefreshToken,
	}); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}

func (h *AuthHandler) Me(c *gin.Context) {
	admin, err := h.authSvc.GetCurrentAdmin(c.Request.Context(), reqctx.GetAdminID(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, &AdminResponse{
		ID:          admin.ID,
		Account:     admin.Account,
		Username:    admin.Username,
		Email:       admin.Email,
		Phone:       admin.Phone,
		DisplayName: admin.GetDisplayName(),
		RealName:    admin.RealName,
		Avatar:      admin.Avatar,
		RoleID:      admin.RoleID,
		Role: func() *RoleSummary {
			if admin.Role == nil {
				return nil
			}
			return &RoleSummary{
				ID:          admin.Role.ID,
				Name:        admin.Role.Name,
				DisplayName: admin.Role.DisplayName,
				Code:        admin.Role.Code,
			}
		}(),
		IsSuper: admin.HasSuperAccess(),
		Status:  admin.Status,
	})
}

func (h *AuthHandler) UpdateMe(c *gin.Context) {
	var req UpdateProfileRequest
	if err := validation.BindJSON(c, &req); err != nil {
		response.Fail(c, err)
		return
	}

	admin, err := h.authSvc.UpdateCurrentAdmin(c.Request.Context(), consoleservice.UpdateCurrentAdminInput{
		AdminID:     reqctx.GetAdminID(c),
		Account:     req.Account,
		Username:    req.Username,
		Email:       req.Email,
		Phone:       req.Phone,
		RealName:    req.RealName,
		DisplayName: req.Nickname,
		Avatar:      req.Avatar,
	})
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.Success(c, &AdminResponse{
		ID:          admin.ID,
		Account:     admin.Account,
		Username:    admin.Username,
		Email:       admin.Email,
		Phone:       admin.Phone,
		DisplayName: admin.GetDisplayName(),
		RealName:    admin.RealName,
		Avatar:      admin.Avatar,
		RoleID:      admin.RoleID,
		Role: func() *RoleSummary {
			if admin.Role == nil {
				return nil
			}
			return &RoleSummary{
				ID:          admin.Role.ID,
				Name:        admin.Role.Name,
				DisplayName: admin.Role.DisplayName,
				Code:        admin.Role.Code,
			}
		}(),
		IsSuper: admin.HasSuperAccess(),
		Status:  admin.Status,
	})
}

func (h *AuthHandler) ChangePassword(c *gin.Context) {
	var req ChangePasswordRequest
	if err := validation.BindJSON(c, &req); err != nil {
		response.Fail(c, err)
		return
	}

	if err := h.authSvc.ChangePassword(c.Request.Context(), consoleservice.ChangePasswordInput{
		AdminID:     reqctx.GetAdminID(c),
		OldPassword: req.OldPassword,
		NewPassword: req.NewPassword,
	}); err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, nil)
}

func (h *AuthHandler) Permissions(c *gin.Context) {
	overview, err := h.authSvc.GetAuthorizationOverview(c.Request.Context(), consoleservice.GetAuthorizationOverviewInput{
		UserID: reqctx.GetAdminID(c),
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, AuthorizationOverviewResponse{
		APIPermissions: overview.APIPermissions,
		MenuKeys:       overview.MenuKeys,
	})
}
