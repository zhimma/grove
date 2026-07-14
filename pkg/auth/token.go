package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	TokenTypeAccess = "access"
)

type Claims struct {
	UserID    string `json:"uid,omitempty"`
	AdminID   string `json:"admin_id,omitempty"`
	SessionID string `json:"session_id,omitempty"`
	UserType  string `json:"user_type,omitempty"`
	Email     string `json:"email,omitempty"`
	RoleID    string `json:"role_id,omitempty"`
	IsSuper   bool   `json:"is_super,omitempty"`
	TokenType string `json:"token_type,omitempty"`
	jwt.RegisteredClaims
}

type ClaimsInput struct {
	UserID    string
	AdminID   string
	SessionID string
	UserType  string
	Email     string
	RoleID    string
	IsSuper   bool
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

type Manager struct {
	secret        []byte
	issuer        string
	accessExpiry  time.Duration
	refreshExpiry time.Duration
}

func NewManager(secret, issuer string, accessExpiry time.Duration, refreshExpiry ...time.Duration) (*Manager, error) {
	if strings.TrimSpace(secret) == "" {
		return nil, errors.New("jwt secret is required")
	}
	if strings.TrimSpace(issuer) == "" {
		issuer = "grove"
	}
	if accessExpiry <= 0 {
		accessExpiry = 24 * time.Hour
	}

	resolvedRefreshExpiry := 7 * 24 * time.Hour
	if len(refreshExpiry) > 0 && refreshExpiry[0] > 0 {
		resolvedRefreshExpiry = refreshExpiry[0]
	}

	return &Manager{
		secret:        []byte(secret),
		issuer:        issuer,
		accessExpiry:  accessExpiry,
		refreshExpiry: resolvedRefreshExpiry,
	}, nil
}

func (m *Manager) IssueAccessToken(userID string) (string, error) {
	return m.IssueAccessTokenWithClaims(ClaimsInput{
		UserID:   userID,
		UserType: "api",
	})
}

func (m *Manager) IssueAccessTokenWithClaims(input ClaimsInput) (string, error) {
	return m.issueToken(input, TokenTypeAccess, m.accessExpiry)
}

func (m *Manager) GenerateTokenPairWithClaims(input ClaimsInput) (*TokenPair, error) {
	accessToken, err := m.issueToken(input, TokenTypeAccess, m.accessExpiry)
	if err != nil {
		return nil, err
	}
	refreshToken, err := newOpaqueToken()
	if err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(m.accessExpiry.Seconds()),
		TokenType:    "Bearer",
	}, nil
}

func (m *Manager) GenerateAdminTokenPair(adminID, sessionID, userType string) (*TokenPair, error) {
	if strings.TrimSpace(sessionID) == "" {
		return nil, errors.New("session id is required")
	}
	return m.GenerateTokenPairWithClaims(ClaimsInput{
		AdminID:   adminID,
		SessionID: sessionID,
		UserType:  userType,
	})
}

func (m *Manager) RefreshExpiry() time.Duration {
	if m == nil {
		return 0
	}
	return m.refreshExpiry
}

func (m *Manager) ParseAccessToken(tokenString string) (*Claims, error) {
	claims, err := m.ValidateToken(tokenString)
	if err != nil {
		return nil, err
	}
	if claims.TokenType != TokenTypeAccess {
		return nil, errors.New("invalid token type")
	}
	return claims, nil
}

func (m *Manager) ValidateToken(tokenString string) (*Claims, error) {
	parsed, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return m.secret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := parsed.Claims.(*Claims)
	if !ok || !parsed.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}

func (m *Manager) issueToken(input ClaimsInput, tokenType string, expiry time.Duration) (string, error) {
	now := time.Now()
	subject := strings.TrimSpace(input.AdminID)
	if subject == "" {
		subject = strings.TrimSpace(input.UserID)
	}

	claims := Claims{
		UserID:    strings.TrimSpace(input.UserID),
		AdminID:   strings.TrimSpace(input.AdminID),
		SessionID: strings.TrimSpace(input.SessionID),
		UserType:  strings.TrimSpace(input.UserType),
		Email:     strings.TrimSpace(input.Email),
		RoleID:    strings.TrimSpace(input.RoleID),
		IsSuper:   input.IsSuper,
		TokenType: tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.NewString(),
			Subject:   subject,
			Issuer:    m.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(expiry)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

func HashToken(tokenString string) string {
	sum := sha256.Sum256([]byte(tokenString))
	return hex.EncodeToString(sum[:])
}

func newOpaqueToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
