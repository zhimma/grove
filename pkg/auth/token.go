package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	TokenTypeAccess = "access"

	// UserType values identify the security surface that issued a token. They
	// are intentionally kept small and explicit so an API token can never be
	// mistaken for a Console session token.
	UserTypeAPI     = "api"
	UserTypeConsole = "console"

	// Audience values bind a token to its issuing security surface.
	AudienceAPI     = "api"
	AudienceConsole = "console"
)

var (
	ErrInvalidToken            = errors.New("invalid token")
	ErrUnexpectedSigningMethod = errors.New("unexpected signing method")
	ErrInvalidIssuer           = errors.New("invalid token issuer")
	ErrInvalidAudience         = errors.New("invalid token audience")
	ErrInvalidUserType         = errors.New("invalid token user type")
	ErrInvalidSubject          = errors.New("invalid token subject")
	ErrInvalidIssuedAt         = errors.New("invalid token issued-at")
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

// Config describes how tokens are signed and how long they live. Zero-valued
// expiries fall back to DefaultConfig.
type Config struct {
	Secret        string
	Issuer        string
	AccessExpiry  time.Duration
	RefreshExpiry time.Duration
}

// DefaultConfig supplies everything except the secret, which has no safe
// default and must always be provided.
func DefaultConfig() Config {
	return Config{
		Issuer:        "grove",
		AccessExpiry:  24 * time.Hour,
		RefreshExpiry: 7 * 24 * time.Hour,
	}
}

func NewManager(config Config) (*Manager, error) {
	if strings.TrimSpace(config.Secret) == "" {
		return nil, errors.New("jwt secret is required")
	}

	defaults := DefaultConfig()
	if strings.TrimSpace(config.Issuer) == "" {
		config.Issuer = defaults.Issuer
	}
	if config.AccessExpiry <= 0 {
		config.AccessExpiry = defaults.AccessExpiry
	}
	if config.RefreshExpiry <= 0 {
		config.RefreshExpiry = defaults.RefreshExpiry
	}

	return &Manager{
		secret:        []byte(config.Secret),
		issuer:        config.Issuer,
		accessExpiry:  config.AccessExpiry,
		refreshExpiry: config.RefreshExpiry,
	}, nil
}

func (m *Manager) IssueAccessToken(userID string) (string, error) {
	return m.IssueAccessTokenWithClaims(ClaimsInput{
		UserID:   userID,
		UserType: UserTypeAPI,
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
	if strings.TrimSpace(userType) == "" {
		userType = UserTypeConsole
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

// ParseAccessTokenForUserType validates an access token for one security
// surface. Keeping this check at the parser boundary prevents callers from
// accidentally accepting a valid token issued for another application.
func (m *Manager) ParseAccessTokenForUserType(tokenString, userType string) (*Claims, error) {
	claims, err := m.ValidateTokenForUserType(tokenString, userType)
	if err != nil {
		return nil, err
	}
	if claims.TokenType != TokenTypeAccess {
		return nil, errors.New("invalid token type")
	}
	return claims, nil
}

// ValidateTokenForUserType is the user-type-scoped counterpart of
// ValidateToken. userType must be one of the known security surfaces.
func (m *Manager) ValidateTokenForUserType(tokenString, userType string) (*Claims, error) {
	userType = normalizeUserType(userType)
	if !isKnownUserType(userType) {
		return nil, ErrInvalidUserType
	}
	claims, err := m.validateToken(tokenString)
	if err != nil {
		return nil, err
	}
	if claims.UserType != userType {
		return nil, ErrInvalidUserType
	}
	if err := validateAudience(claims, userType); err != nil {
		return nil, err
	}
	return claims, nil
}

func (m *Manager) ValidateToken(tokenString string) (*Claims, error) {
	claims, err := m.validateToken(tokenString)
	if err != nil {
		return nil, err
	}
	if !isKnownUserType(claims.UserType) {
		return nil, ErrInvalidUserType
	}
	if err := validateAudience(claims, claims.UserType); err != nil {
		return nil, err
	}
	return claims, nil
}

func (m *Manager) validateToken(tokenString string) (*Claims, error) {
	if m == nil || len(m.secret) == 0 {
		return nil, ErrInvalidToken
	}
	tokenString = strings.TrimSpace(tokenString)
	if tokenString == "" {
		return nil, ErrInvalidToken
	}

	// jwt.ParseWithClaims validates exp/nbf by default, but exp and iat are
	// optional unless the corresponding parser options are enabled. Require
	// both claims so a signed token cannot live forever or be minted in the
	// future. WithValidMethods also closes the HMAC algorithm-confusion gap.
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(m.issuer),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
	)
	parsed, err := parser.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (any, error) {
		if token.Method == nil || token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, ErrUnexpectedSigningMethod
		}
		return m.secret, nil
	})
	if err != nil {
		// Preserve the stable sentinel for issuer failures while retaining the
		// parser's detailed error as a wrapped cause for diagnostics.
		if strings.Contains(strings.ToLower(err.Error()), "issuer") {
			return nil, fmt.Errorf("%w: %v", ErrInvalidIssuer, err)
		}
		return nil, err
	}
	claims, ok := parsed.Claims.(*Claims)
	if !ok || !parsed.Valid {
		return nil, ErrInvalidToken
	}
	if strings.TrimSpace(claims.Subject) == "" {
		return nil, ErrInvalidSubject
	}
	if claims.IssuedAt == nil {
		return nil, ErrInvalidIssuedAt
	}
	if err := validateIdentity(claims); err != nil {
		return nil, err
	}
	return claims, nil
}

func (m *Manager) issueToken(input ClaimsInput, tokenType string, expiry time.Duration) (string, error) {
	if m == nil || len(m.secret) == 0 {
		return "", ErrInvalidToken
	}
	userType := normalizeUserType(input.UserType)
	if userType == "" {
		userType = UserTypeAPI
	}
	if !isKnownUserType(userType) {
		return "", fmt.Errorf("%w: %s", ErrInvalidUserType, userType)
	}
	if expiry <= 0 {
		return "", errors.New("token expiry must be positive")
	}
	now := time.Now()
	subject := strings.TrimSpace(input.AdminID)
	if subject == "" {
		subject = strings.TrimSpace(input.UserID)
	}
	if subject == "" {
		return "", ErrInvalidSubject
	}
	if userType == UserTypeAPI && strings.TrimSpace(input.UserID) == "" {
		return "", ErrInvalidSubject
	}
	if userType == UserTypeConsole && (strings.TrimSpace(input.AdminID) == "" || strings.TrimSpace(input.SessionID) == "") {
		return "", ErrInvalidSubject
	}

	claims := Claims{
		UserID:    strings.TrimSpace(input.UserID),
		AdminID:   strings.TrimSpace(input.AdminID),
		SessionID: strings.TrimSpace(input.SessionID),
		UserType:  userType,
		Email:     strings.TrimSpace(input.Email),
		RoleID:    strings.TrimSpace(input.RoleID),
		IsSuper:   input.IsSuper,
		TokenType: tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.NewString(),
			Subject:   subject,
			Issuer:    m.issuer,
			Audience:  jwt.ClaimStrings{audienceForUserType(userType)},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(expiry)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

func normalizeUserType(userType string) string {
	return strings.ToLower(strings.TrimSpace(userType))
}

func isKnownUserType(userType string) bool {
	switch normalizeUserType(userType) {
	case UserTypeAPI, UserTypeConsole:
		return true
	default:
		return false
	}
}

func audienceForUserType(userType string) string {
	switch normalizeUserType(userType) {
	case UserTypeConsole:
		return AudienceConsole
	default:
		return AudienceAPI
	}
}

func validateAudience(claims *Claims, userType string) error {
	if claims == nil || len(claims.Audience) != 1 {
		return ErrInvalidAudience
	}
	expected := audienceForUserType(userType)
	if strings.TrimSpace(claims.Audience[0]) != expected {
		return ErrInvalidAudience
	}
	return nil
}

func validateIdentity(claims *Claims) error {
	if claims == nil {
		return ErrInvalidToken
	}
	switch normalizeUserType(claims.UserType) {
	case UserTypeAPI:
		if strings.TrimSpace(claims.UserID) == "" || strings.TrimSpace(claims.AdminID) != "" {
			return ErrInvalidSubject
		}
		if strings.TrimSpace(claims.Subject) != strings.TrimSpace(claims.UserID) {
			return ErrInvalidSubject
		}
	case UserTypeConsole:
		if strings.TrimSpace(claims.AdminID) == "" || strings.TrimSpace(claims.SessionID) == "" || strings.TrimSpace(claims.UserID) != "" {
			return ErrInvalidSubject
		}
		if strings.TrimSpace(claims.Subject) != strings.TrimSpace(claims.AdminID) {
			return ErrInvalidSubject
		}
	default:
		return ErrInvalidUserType
	}
	return nil
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
