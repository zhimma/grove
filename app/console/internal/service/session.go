package service

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/zhimma/grove/internal/model"
	"github.com/zhimma/grove/pkg/auth"
	"github.com/zhimma/grove/pkg/database"
	"github.com/zhimma/grove/pkg/errx"
	"github.com/zhimma/grove/pkg/pagination"
	"github.com/zhimma/grove/pkg/transaction"
	"github.com/zhimma/grove/pkg/ulid"
)

const sessionActivityWriteInterval = 5 * time.Minute

type SessionService struct {
	dbs          *database.Connections
	tokenManager *auth.Manager
	pages        pagination.Policy
}

type CreateSessionInput struct {
	AdminID    string
	DeviceName string
	ClientIP   string
	UserAgent  string
}

type ListSessionsInput struct {
	Page     int
	PageSize int
	AdminID  string
	Keyword  string
	Status   string
}

type ListSessionsResult struct {
	List []model.ConsoleSession
	Meta pagination.Meta
}

func NewSessionService(dbs *database.Connections, tokenManager *auth.Manager, pages pagination.Policy) *SessionService {
	return &SessionService{dbs: dbs, tokenManager: tokenManager, pages: pages}
}

func (s *SessionService) Create(ctx context.Context, input CreateSessionInput) (*model.ConsoleSession, *auth.TokenPair, error) {
	db, err := s.db(ctx)
	if err != nil {
		return nil, nil, err
	}
	if s.tokenManager == nil {
		return nil, nil, errx.ServiceUnavailable().WithMessage("令牌管理器未配置")
	}

	adminID := strings.TrimSpace(input.AdminID)
	if adminID == "" {
		return nil, nil, errx.InvalidParams().WithHTTPStatus(422).WithMessage("管理员ID不能为空")
	}

	now := time.Now()
	sessionID := ulid.New()
	pair, err := s.tokenManager.GenerateAdminTokenPair(adminID, sessionID, "console")
	if err != nil {
		return nil, nil, errx.Internal().WithCause(err)
	}
	userAgent := truncateLoginUA(strings.TrimSpace(input.UserAgent))
	deviceName := truncateSessionDevice(strings.TrimSpace(input.DeviceName))
	if deviceName == "" {
		deviceName = truncateSessionDevice(userAgent)
	}
	if deviceName == "" {
		deviceName = "Unknown Device"
	}

	session := &model.ConsoleSession{
		AuditBase:        model.AuditBase{ID: sessionID},
		AdminID:          adminID,
		RefreshTokenHash: auth.HashToken(pair.RefreshToken),
		DeviceName:       deviceName,
		ClientIP:         truncateLoginIP(strings.TrimSpace(input.ClientIP)),
		UserAgent:        userAgent,
		LastActiveAt:     now,
		ExpiresAt:        now.Add(s.tokenManager.RefreshExpiry()),
	}
	if err := db.Create(session).Error; err != nil {
		return nil, nil, errx.Internal().WithCause(err)
	}
	return session, pair, nil
}

func (s *SessionService) Rotate(ctx context.Context, refreshToken string) (*auth.TokenPair, error) {
	db, err := s.db(ctx)
	if err != nil {
		return nil, err
	}
	if s.tokenManager == nil {
		return nil, errx.ServiceUnavailable().WithMessage("令牌管理器未配置")
	}
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return nil, errx.InvalidParams().WithHTTPStatus(422).WithMessage("刷新令牌不能为空")
	}

	now := time.Now()
	oldHash := auth.HashToken(refreshToken)
	var session model.ConsoleSession
	if err := db.Where("refresh_token_hash = ? AND revoked_at IS NULL AND expires_at > ?", oldHash, now).
		First(&session).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, invalidRefreshToken(err)
		}
		return nil, errx.Internal().WithCause(err)
	}
	if _, err := NewAdminAuthStateResolver(s.dbs).ResolveAdminAuthState(ctx, session.AdminID); err != nil {
		return nil, err
	}

	pair, err := s.tokenManager.GenerateAdminTokenPair(session.AdminID, session.ID, "console")
	if err != nil {
		return nil, errx.Internal().WithCause(err)
	}
	result := db.Model(&model.ConsoleSession{}).
		Where("id = ? AND refresh_token_hash = ? AND revoked_at IS NULL AND expires_at > ?", session.ID, oldHash, now).
		Updates(map[string]any{
			"refresh_token_hash": auth.HashToken(pair.RefreshToken),
			"last_active_at":     now,
			"updated_at":         now,
		})
	if result.Error != nil {
		return nil, errx.Internal().WithCause(result.Error)
	}
	if result.RowsAffected != 1 {
		return nil, invalidRefreshToken(nil)
	}
	return pair, nil
}

func (s *SessionService) Validate(ctx context.Context, adminID, sessionID string) (*model.ConsoleSession, error) {
	db, err := s.db(ctx)
	if err != nil {
		return nil, err
	}
	adminID = strings.TrimSpace(adminID)
	sessionID = strings.TrimSpace(sessionID)
	if adminID == "" || sessionID == "" {
		return nil, errx.Unauthorized().WithMessage("登录会话无效").WithCode("invalid_session")
	}

	now := time.Now()
	var session model.ConsoleSession
	if err := db.Where("id = ? AND admin_id = ? AND revoked_at IS NULL AND expires_at > ?", sessionID, adminID, now).
		First(&session).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errx.Unauthorized().WithMessage("登录会话已失效").WithCode("invalid_session")
		}
		return nil, errx.Internal().WithCause(err)
	}
	if session.LastActiveAt.Before(now.Add(-sessionActivityWriteInterval)) {
		_ = db.Model(&model.ConsoleSession{}).
			Where("id = ? AND last_active_at = ?", session.ID, session.LastActiveAt).
			Updates(map[string]any{"last_active_at": now, "updated_at": now}).Error
		session.LastActiveAt = now
	}
	return &session, nil
}

func (s *SessionService) Revoke(ctx context.Context, sessionID, reason string) error {
	db, err := s.db(ctx)
	if err != nil {
		return err
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil
	}
	now := time.Now()
	if err := db.Model(&model.ConsoleSession{}).
		Where("id = ? AND revoked_at IS NULL", sessionID).
		Updates(map[string]any{
			"revoked_at":    now,
			"revoke_reason": truncateSessionReason(reason),
			"updated_at":    now,
		}).Error; err != nil {
		return errx.Internal().WithCause(err)
	}
	return nil
}

func (s *SessionService) RevokeAdmin(ctx context.Context, adminID, reason string) error {
	db, err := s.db(ctx)
	if err != nil {
		return err
	}
	adminID = strings.TrimSpace(adminID)
	if adminID == "" {
		return nil
	}
	now := time.Now()
	if err := db.Model(&model.ConsoleSession{}).
		Where("admin_id = ? AND revoked_at IS NULL", adminID).
		Updates(map[string]any{
			"revoked_at":    now,
			"revoke_reason": truncateSessionReason(reason),
			"updated_at":    now,
		}).Error; err != nil {
		return errx.Internal().WithCause(err)
	}
	return nil
}

func (s *SessionService) List(ctx context.Context, input ListSessionsInput) (*ListSessionsResult, error) {
	db, err := s.db(ctx)
	if err != nil {
		return nil, err
	}
	page := s.pages.Resolve(pagination.Request{Page: input.Page, PageSize: input.PageSize})
	now := time.Now()
	query := db.Model(&model.ConsoleSession{}).Preload("Admin")
	if adminID := strings.TrimSpace(input.AdminID); adminID != "" {
		query = query.Where("admin_id = ?", adminID)
	}
	if keyword := strings.TrimSpace(input.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Joins("LEFT JOIN console_admins ON console_admins.id = console_sessions.admin_id").
			Where("console_admins.account LIKE ? OR console_sessions.device_name LIKE ? OR console_sessions.client_ip LIKE ?", like, like, like)
	}
	switch strings.ToLower(strings.TrimSpace(input.Status)) {
	case "active":
		query = query.Where("console_sessions.revoked_at IS NULL AND console_sessions.expires_at > ?", now)
	case "revoked":
		query = query.Where("console_sessions.revoked_at IS NOT NULL")
	case "expired":
		query = query.Where("console_sessions.revoked_at IS NULL AND console_sessions.expires_at <= ?", now)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, errx.Internal().WithCause(err)
	}
	var sessions []model.ConsoleSession
	query = page.Apply(query.Order("console_sessions.last_active_at DESC"))
	if err := query.Find(&sessions).Error; err != nil {
		return nil, errx.Internal().WithCause(err)
	}
	return &ListSessionsResult{List: sessions, Meta: pagination.NewMeta(total, page)}, nil
}

func (s *SessionService) db(ctx context.Context) (*gorm.DB, error) {
	if s == nil || s.dbs == nil || s.dbs.Default() == nil {
		return nil, errx.ServiceUnavailable().WithMessage("默认数据库未配置")
	}
	return transaction.GetDB(ctx, s.dbs.Default()), nil
}

func invalidRefreshToken(cause error) error {
	err := errx.Unauthorized().WithMessage("刷新令牌无效").WithCode("invalid_refresh_token")
	if cause != nil {
		return err.WithCause(cause)
	}
	return err
}

func truncateSessionDevice(value string) string {
	if len(value) <= 120 {
		return value
	}
	return value[:120]
}

func truncateSessionReason(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 255 {
		return value
	}
	return value[:255]
}
