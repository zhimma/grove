package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"
)

type Claims struct {
	UserID    string `json:"uid,omitempty"`
	AdminID   string `json:"admin_id,omitempty"`
	UserType  string `json:"user_type,omitempty"`
	Email     string `json:"email,omitempty"`
	RoleID    string `json:"role_id,omitempty"`
	IsSuper   bool   `json:"is_super,omitempty"`
	TokenType string `json:"token_type,omitempty"`
	jwt.RegisteredClaims
}

type ClaimsInput struct {
	UserID   string
	AdminID  string
	UserType string
	Email    string
	RoleID   string
	IsSuper  bool
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

type blacklistEntry struct {
	expiresAt time.Time
}

type Manager struct {
	secret        []byte
	issuer        string
	accessExpiry  time.Duration
	refreshExpiry time.Duration
	mu            sync.Mutex
	blacklist     map[string]blacklistEntry
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
		blacklist:     map[string]blacklistEntry{},
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
	refreshToken, err := m.issueToken(input, TokenTypeRefresh, m.refreshExpiry)
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

func (m *Manager) GenerateAdminTokenPair(adminID, userType string) (*TokenPair, error) {
	return m.GenerateTokenPairWithClaims(ClaimsInput{
		AdminID:  adminID,
		UserType: userType,
	})
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

func (m *Manager) ParseRefreshToken(tokenString string) (*Claims, error) {
	claims, err := m.ValidateToken(tokenString)
	if err != nil {
		return nil, err
	}
	if claims.TokenType != TokenTypeRefresh {
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
	if m.isRevoked(tokenString) {
		return nil, errors.New("token revoked")
	}
	return claims, nil
}

func (m *Manager) Revoke(tokenString string) error {
	parsed, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return m.secret, nil
	})
	if err != nil {
		return err
	}
	claims, ok := parsed.Claims.(*Claims)
	if !ok || claims.ExpiresAt == nil {
		return errors.New("invalid token")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.gcBlacklistLocked()
	m.blacklist[tokenHash(tokenString)] = blacklistEntry{
		expiresAt: claims.ExpiresAt.Time,
	}
	return nil
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

func (m *Manager) isRevoked(tokenString string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.gcBlacklistLocked()
	_, ok := m.blacklist[tokenHash(tokenString)]
	return ok
}

func (m *Manager) gcBlacklistLocked() {
	now := time.Now()
	for key, entry := range m.blacklist {
		if !entry.expiresAt.After(now) {
			delete(m.blacklist, key)
		}
	}
}

func tokenHash(tokenString string) string {
	sum := sha256.Sum256([]byte(tokenString))
	return hex.EncodeToString(sum[:])
}

// ExtractBearer pulls the token out of an Authorization header.
// Returns ("", false) for an empty header, a missing Bearer prefix,
// or any other scheme.
func ExtractBearer(header string) (string, bool) {
	h := strings.TrimSpace(header)
	if h == "" {
		return "", false
	}
	const scheme = "bearer "
	if len(h) < len(scheme) || !strings.EqualFold(h[:len(scheme)], scheme) {
		return "", false
	}
	tok := strings.TrimSpace(h[len(scheme):])
	if tok == "" {
		return "", false
	}
	return tok, true
}
