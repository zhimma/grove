package service

import (
	"context"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/zhimma/grove/internal/model"
	"github.com/zhimma/grove/pkg/auth"
	"github.com/zhimma/grove/pkg/database"
	"github.com/zhimma/grove/pkg/errx"
	"github.com/zhimma/grove/pkg/logger"
	"github.com/zhimma/grove/pkg/ratelimit"
	"github.com/zhimma/grove/pkg/rbac"
	"github.com/zhimma/grove/pkg/request"
	"github.com/zhimma/grove/pkg/transaction"
)

type AuthService struct {
	dbs          database.Connections
	enforcer     *rbac.Enforcer
	tokenManager *auth.Manager
	sessions     *SessionService
	loginGuard   ratelimit.LoginGuard
}

var invalidLoginPasswordHash = []byte("$2y$10$WuzL7jUB./OeDcEIx.eBV.WkSEyl5dY1uxdfxdCq4hZDMFfeU1ZGC")

type LoginInput struct {
	Account    string
	Password   string
	DeviceName string
}

type LoginOutput struct {
	Admin *model.ConsoleAdmin
	Token *auth.TokenPair
}

type RefreshTokenInput struct {
	RefreshToken string
}

type RefreshTokenOutput struct {
	Token *auth.TokenPair
}

type LogoutInput struct {
	AccessToken  string
	RefreshToken string
	SessionID    string
}

type ChangePasswordInput struct {
	AdminID     string
	OldPassword string
	NewPassword string
}

type UpdateCurrentAdminInput struct {
	AdminID     string
	Account     string
	Username    string
	Email       string
	Phone       string
	RealName    string
	DisplayName string
	Avatar      string
}

type GetAuthorizationOverviewInput struct {
	UserID string
}

type GetAuthorizationOverviewOutput struct {
	APIPermissions []string `json:"api_permissions"`
	MenuKeys       []string `json:"menu_keys"`
}

func NewAuthService(dbs database.Connections, enforcer *rbac.Enforcer, tm *auth.Manager, guards ...ratelimit.LoginGuard) *AuthService {
	service := &AuthService{
		dbs:          dbs,
		enforcer:     enforcer,
		tokenManager: tm,
		sessions:     NewSessionService(dbs, tm),
	}
	if len(guards) > 0 {
		service.loginGuard = guards[0]
	}
	return service
}

func (s *AuthService) Login(ctx context.Context, input LoginInput) (LoginOutput, error) {
	if s.dbs == nil || s.dbs.Default() == nil {
		return LoginOutput{}, errx.ServiceUnavailable().WithMessage("默认数据库未配置")
	}
	if s.tokenManager == nil {
		return LoginOutput{}, errx.ServiceUnavailable().WithMessage("令牌管理器未配置")
	}

	account := strings.TrimSpace(input.Account)
	if account == "" || strings.TrimSpace(input.Password) == "" {
		return LoginOutput{}, errx.InvalidParams().WithHTTPStatus(422).WithMessage("账号和密码不能为空")
	}
	meta := request.GetRequestMetaFromContext(ctx)
	loginKey := ratelimit.LoginKey(account, meta.ClientIP)
	if s.loginGuard != nil {
		if err := s.loginGuard.Allow(ctx, loginKey); err != nil {
			if retryAfter := ratelimit.RetryAfter(err); retryAfter > 0 {
				return LoginOutput{}, loginProtectionError("login_rate_limited", "登录请求过于频繁", retryAfter)
			}
			return LoginOutput{}, loginProtectionUnavailable(err)
		}
		if retryAfter, err := s.loginGuard.Locked(ctx, loginKey); err != nil {
			return LoginOutput{}, loginProtectionUnavailable(err)
		} else if retryAfter > 0 {
			return LoginOutput{}, loginProtectionError("login_locked", "登录失败次数过多，请稍后再试", retryAfter)
		}
	}

	var admin model.ConsoleAdmin
	if err := s.dbs.Default().WithContext(ctx).
		Preload("Role").
		Where("account = ? OR LOWER(email) = LOWER(?) OR phone = ?", account, account, account).
		First(&admin).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			_ = bcrypt.CompareHashAndPassword(invalidLoginPasswordHash, []byte(input.Password))
			s.writeLoginLog(ctx, "", account, false, "账号或密码错误")
			return LoginOutput{}, s.credentialFailure(ctx, loginKey)
		}
		return LoginOutput{}, errx.Internal().WithCause(err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(admin.Password), []byte(input.Password)); err != nil {
		s.writeLoginLog(ctx, admin.ID, admin.Account, false, "账号或密码错误")
		return LoginOutput{}, s.credentialFailure(ctx, loginKey)
	}
	if !admin.CanLogin() {
		s.writeLoginLog(ctx, admin.ID, admin.Account, false, "账号已被禁用")
		return LoginOutput{}, errx.Forbidden().WithMessage("账号已被禁用").WithCode("account_disabled")
	}
	if admin.RoleID != "" && (admin.Role == nil || !admin.Role.IsActive()) {
		s.writeLoginLog(ctx, admin.ID, admin.Account, false, "角色不可用")
		return LoginOutput{}, errx.Forbidden().WithMessage("角色不可用").WithCode("role_unavailable")
	}
	if s.loginGuard != nil {
		if err := s.loginGuard.Reset(ctx, loginKey); err != nil {
			return LoginOutput{}, loginProtectionUnavailable(err)
		}
	}

	now := time.Now()
	var tokenPair *auth.TokenPair
	if err := s.dbs.Default().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txCtx := transaction.WithDB(ctx, tx)
		_, pair, createErr := s.sessions.Create(txCtx, CreateSessionInput{
			AdminID:    admin.ID,
			DeviceName: input.DeviceName,
			ClientIP:   meta.ClientIP,
			UserAgent:  meta.UserAgent,
		})
		if createErr != nil {
			return createErr
		}
		tokenPair = pair
		if updateErr := tx.Model(&model.ConsoleAdmin{}).
			Where("id = ?", admin.ID).
			Updates(map[string]any{
				"last_login_at": now,
				"last_login_ip": truncateLoginIP(meta.ClientIP),
				"login_count":   gorm.Expr("login_count + 1"),
			}).Error; updateErr != nil {
			return errx.Internal().WithCause(updateErr)
		}
		return nil
	}); err != nil {
		return LoginOutput{}, err
	}
	admin.LastLoginAt = &now
	admin.LastLoginIP = truncateLoginIP(meta.ClientIP)
	admin.LoginCount++
	s.writeLoginLog(ctx, admin.ID, admin.Account, true, "")

	admin.Password = ""
	return LoginOutput{
		Admin: &admin,
		Token: tokenPair,
	}, nil
}

func (s *AuthService) credentialFailure(ctx context.Context, loginKey string) error {
	if s.loginGuard != nil {
		retryAfter, err := s.loginGuard.Failure(ctx, loginKey)
		if err != nil {
			return loginProtectionUnavailable(err)
		}
		if retryAfter > 0 {
			return loginProtectionError("login_locked", "登录失败次数过多，请稍后再试", retryAfter)
		}
	}
	return errx.Unauthorized().WithMessage("账号或密码错误").WithCode("invalid_credentials")
}

func loginProtectionError(code, message string, retryAfter time.Duration) error {
	retrySeconds := int64((retryAfter + time.Second - 1) / time.Second)
	if retrySeconds < 1 {
		retrySeconds = 1
	}
	return errx.TooManyRequests().
		WithCode(code).
		WithMessage(message).
		WithData(map[string]any{"retry_after": retrySeconds})
}

func loginProtectionUnavailable(err error) error {
	return errx.ServiceUnavailable().
		WithCode("login_protection_unavailable").
		WithMessage("登录保护服务暂不可用").
		WithCause(err)
}

func (s *AuthService) RefreshToken(ctx context.Context, input RefreshTokenInput) (RefreshTokenOutput, error) {
	refreshToken := strings.TrimSpace(input.RefreshToken)
	if refreshToken == "" {
		return RefreshTokenOutput{}, errx.InvalidParams().WithHTTPStatus(422).WithMessage("刷新令牌不能为空")
	}

	tokenPair, err := s.sessions.Rotate(ctx, refreshToken)
	if err != nil {
		return RefreshTokenOutput{}, err
	}

	return RefreshTokenOutput{Token: tokenPair}, nil
}

func (s *AuthService) Logout(ctx context.Context, input LogoutInput) error {
	sessionID := strings.TrimSpace(input.SessionID)
	if sessionID == "" && s.tokenManager != nil && strings.TrimSpace(input.AccessToken) != "" {
		claims, err := s.tokenManager.ParseAccessToken(strings.TrimSpace(input.AccessToken))
		if err != nil {
			return errx.Unauthorized().WithMessage("访问令牌无效").WithCause(err)
		}
		sessionID = claims.SessionID
	}
	return s.sessions.Revoke(ctx, sessionID, "logout")
}

func (s *AuthService) writeLoginLog(ctx context.Context, adminID, account string, success bool, failureReason string) {
	if s.dbs == nil || s.dbs.Default() == nil {
		logger.Warn().
			Str("module", "console_login").
			Str("account", strings.TrimSpace(account)).
			Msg("登录日志写入跳过：默认数据库未配置")
		return
	}
	meta := request.GetRequestMetaFromContext(ctx)
	record := model.ConsoleLoginLog{
		AdminID:       strings.TrimSpace(adminID),
		Account:       strings.TrimSpace(account),
		Success:       success,
		FailureReason: strings.TrimSpace(failureReason),
		RequestID:     strings.TrimSpace(meta.RequestID),
		ClientIP:      truncateLoginIP(meta.ClientIP),
		UserAgent:     truncateLoginUA(meta.UserAgent),
	}
	if err := s.dbs.Default().WithContext(ctx).Create(&record).Error; err != nil {
		logger.Warn().
			Err(err).
			Str("module", "console_login").
			Str("request_id", record.RequestID).
			Str("admin_id", record.AdminID).
			Str("account", record.Account).
			Msg("登录日志写入失败")
	}
}

func truncateLoginIP(value string) string {
	if len(value) <= 64 {
		return value
	}
	return value[:64]
}

func truncateLoginUA(value string) string {
	if len(value) <= 500 {
		return value
	}
	return value[:500]
}

func (s *AuthService) GetCurrentAdmin(ctx context.Context, adminID string) (*model.ConsoleAdmin, error) {
	if s.dbs == nil || s.dbs.Default() == nil {
		return nil, errx.ServiceUnavailable().WithMessage("默认数据库未配置")
	}

	var admin model.ConsoleAdmin
	if err := s.dbs.Default().WithContext(ctx).
		Preload("Role").
		Where("id = ?", adminID).
		First(&admin).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errx.NotFound().WithMessage("管理员不存在")
		}
		return nil, errx.Internal().WithCause(err)
	}
	admin.Password = ""
	return &admin, nil
}

func (s *AuthService) ChangePassword(ctx context.Context, input ChangePasswordInput) error {
	if s.dbs == nil || s.dbs.Default() == nil {
		return errx.ServiceUnavailable().WithMessage("默认数据库未配置")
	}
	return s.dbs.Default().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var admin model.ConsoleAdmin
		if err := tx.Where("id = ?", input.AdminID).First(&admin).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return errx.NotFound().WithMessage("管理员不存在")
			}
			return errx.Internal().WithCause(err)
		}
		if err := bcrypt.CompareHashAndPassword([]byte(admin.Password), []byte(input.OldPassword)); err != nil {
			return errx.Unauthorized().WithMessage("原密码不正确").WithCode("invalid_credentials")
		}
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.NewPassword), bcrypt.DefaultCost)
		if err != nil {
			return errx.Internal().WithCause(err)
		}
		if err := tx.Model(&model.ConsoleAdmin{}).
			Where("id = ?", input.AdminID).
			Updates(map[string]any{
				"password":             string(hashedPassword),
				"must_change_password": false,
			}).Error; err != nil {
			return errx.Internal().WithCause(err)
		}
		return s.sessions.RevokeAdmin(transaction.WithDB(ctx, tx), input.AdminID, "password_changed")
	})
}

func (s *AuthService) UpdateCurrentAdmin(ctx context.Context, input UpdateCurrentAdminInput) (*model.ConsoleAdmin, error) {
	if s.dbs == nil || s.dbs.Default() == nil {
		return nil, errx.ServiceUnavailable().WithMessage("默认数据库未配置")
	}

	adminID := strings.TrimSpace(input.AdminID)
	if adminID == "" {
		return nil, errx.InvalidParams().WithHTTPStatus(422).WithMessage("管理员ID不能为空")
	}

	account := strings.TrimSpace(input.Account)
	email := strings.TrimSpace(input.Email)
	phone := strings.TrimSpace(input.Phone)
	if account == "" {
		return nil, errx.InvalidParams().WithHTTPStatus(422).WithMessage("账号不能为空")
	}

	if err := s.ensureCurrentAdminUnique(ctx, adminID, account, email, phone); err != nil {
		return nil, err
	}

	updates := map[string]any{
		"account":      account,
		"username":     strings.TrimSpace(input.Username),
		"email":        email,
		"phone":        phone,
		"real_name":    strings.TrimSpace(input.RealName),
		"display_name": strings.TrimSpace(input.DisplayName),
		"avatar":       strings.TrimSpace(input.Avatar),
	}
	if err := s.dbs.Default().WithContext(ctx).
		Model(&model.ConsoleAdmin{}).
		Where("id = ?", adminID).
		Updates(updates).Error; err != nil {
		return nil, errx.Internal().WithCause(err)
	}

	return s.GetCurrentAdmin(ctx, adminID)
}

func (s *AuthService) GetAuthorizationOverview(ctx context.Context, input GetAuthorizationOverviewInput) (*GetAuthorizationOverviewOutput, error) {
	admin, err := s.GetCurrentAdmin(ctx, input.UserID)
	if err != nil {
		return nil, err
	}

	if admin.HasSuperAccess() {
		return &GetAuthorizationOverviewOutput{
			APIPermissions: []string{"*"},
			MenuKeys:       []string{"*"},
		}, nil
	}

	output := &GetAuthorizationOverviewOutput{
		APIPermissions: []string{},
		MenuKeys:       []string{},
	}

	if admin.RoleID == "" {
		return output, nil
	}

	if s.enforcer != nil {
		policies, err := s.enforcer.GetConsolePoliciesForRole(admin.RoleID)
		if err != nil {
			return nil, errx.Internal().WithCause(err)
		}
		apiPermissions := make([]string, 0, len(policies))
		for _, policy := range policies {
			if len(policy) >= 2 {
				apiPermissions = append(apiPermissions, policy[1])
			}
		}
		output.APIPermissions = uniqueNonEmptyStrings(apiPermissions)
	}

	if admin.Role != nil {
		output.MenuKeys = normalizeConsoleMenuKeys(admin.Role.MenuKeys)
	}

	return output, nil
}

func (s *AuthService) GetPermissions(ctx context.Context, adminID string) ([]string, error) {
	overview, err := s.GetAuthorizationOverview(ctx, GetAuthorizationOverviewInput{UserID: adminID})
	if err != nil {
		return nil, err
	}
	return overview.APIPermissions, nil
}

func (s *AuthService) ensureCurrentAdminUnique(ctx context.Context, excludeID, account, email, phone string) error {
	checks := []struct {
		column  string
		value   string
		code    string
		message string
	}{
		{column: "account", value: account, code: "account_exists", message: "账号已存在"},
		{column: "email", value: email, code: "email_exists", message: "邮箱已存在"},
		{column: "phone", value: phone, code: "phone_exists", message: "手机号已存在"},
	}

	for _, check := range checks {
		if check.value == "" {
			continue
		}
		query := s.dbs.Default().WithContext(ctx).Model(&model.ConsoleAdmin{}).Where(check.column+" = ?", check.value)
		if excludeID != "" {
			query = query.Where("id <> ?", excludeID)
		}
		var count int64
		if err := query.Count(&count).Error; err != nil {
			return errx.Internal().WithCause(err)
		}
		if count > 0 {
			return errx.Conflict().WithCode(check.code).WithMessage(check.message)
		}
	}

	return nil
}
