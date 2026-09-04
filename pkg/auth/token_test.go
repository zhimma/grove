package auth

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestIssueAndParseAccessToken(t *testing.T) {
	manager, err := NewManager(Config{Secret: "test-secret", Issuer: "test-issuer", AccessExpiry: 0})
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	token, err := manager.IssueAccessToken("api-user")
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	claims, err := manager.ParseAccessToken(token)
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}

	if claims.UserID != "api-user" {
		t.Fatalf("expected api-user, got %s", claims.UserID)
	}
	if claims.UserType != "api" {
		t.Fatalf("expected default api user type, got %s", claims.UserType)
	}
}

func TestIssueAndParseConsoleClaims(t *testing.T) {
	manager, err := NewManager(Config{Secret: "test-secret", Issuer: "test-issuer", AccessExpiry: 0})
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	tokenPair, err := manager.GenerateAdminTokenPair("console-admin-demo", "console-session-demo", "console")
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	claims, err := manager.ParseAccessToken(tokenPair.AccessToken)
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}
	if claims.AdminID != "console-admin-demo" || claims.SessionID != "console-session-demo" || claims.UserType != "console" {
		t.Fatalf("unexpected console claims: %+v", claims)
	}
	if claims.UserID != "" || claims.Email != "" || claims.RoleID != "" || claims.IsSuper {
		t.Fatalf("unexpected console claims: %+v", claims)
	}
}

func TestConsoleRefreshTokenIsOpaqueAndHashable(t *testing.T) {
	manager, err := NewManager(Config{Secret: "test-secret", Issuer: "test-issuer", AccessExpiry: time.Hour, RefreshExpiry: 24 * time.Hour})
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	first, err := manager.GenerateAdminTokenPair("console-admin-demo", "console-session-demo", "console")
	if err != nil {
		t.Fatalf("generate first token pair: %v", err)
	}
	second, err := manager.GenerateAdminTokenPair("console-admin-demo", "console-session-demo", "console")
	if err != nil {
		t.Fatalf("generate second token pair: %v", err)
	}

	if first.RefreshToken == "" || strings.Count(first.RefreshToken, ".") == 2 {
		t.Fatalf("refresh token must be opaque, got %q", first.RefreshToken)
	}
	if first.RefreshToken == second.RefreshToken {
		t.Fatal("refresh tokens must be random")
	}
	hash := HashToken(first.RefreshToken)
	if len(hash) != 64 || hash == first.RefreshToken {
		t.Fatalf("unexpected refresh token hash %q", hash)
	}
	if manager.RefreshExpiry() != 24*time.Hour {
		t.Fatalf("unexpected refresh expiry %s", manager.RefreshExpiry())
	}
}

func TestValidateTokenRejectsUnexpectedSigningMethod(t *testing.T) {
	manager, err := NewManager(Config{Secret: "test-secret", Issuer: "test-issuer", AccessExpiry: time.Hour})
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	token := jwt.NewWithClaims(jwt.SigningMethodNone, Claims{
		UserID:    "api-user",
		UserType:  "api",
		TokenType: TokenTypeAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "api-user",
			Issuer:    "test-issuer",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	})
	tokenString, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("sign none token: %v", err)
	}

	if _, err := manager.ValidateToken(tokenString); err == nil {
		t.Fatal("expected unexpected signing method to be rejected")
	}
}

func TestValidateTokenRejectsHS384EvenThoughItIsHMAC(t *testing.T) {
	manager, err := NewManager(Config{Secret: "test-secret", Issuer: "test-issuer", AccessExpiry: time.Hour})
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS384, Claims{
		UserID:    "api-user",
		UserType:  UserTypeAPI,
		TokenType: TokenTypeAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "api-user",
			Issuer:    "test-issuer",
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	})
	tokenString, err := token.SignedString([]byte("test-secret"))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	if _, err := manager.ValidateToken(tokenString); err == nil {
		t.Fatal("expected HS384 token to be rejected")
	}
}

func TestValidateTokenRequiresIssuerAndExpirationAndSubject(t *testing.T) {
	manager, err := NewManager(Config{Secret: "test-secret", Issuer: "test-issuer", AccessExpiry: time.Hour})
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	cases := []struct {
		name   string
		claims jwt.RegisteredClaims
	}{
		{name: "wrong issuer", claims: jwt.RegisteredClaims{Subject: "api-user", Issuer: "other", IssuedAt: jwt.NewNumericDate(time.Now()), ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}},
		{name: "missing issuer", claims: jwt.RegisteredClaims{Subject: "api-user", IssuedAt: jwt.NewNumericDate(time.Now()), ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}},
		{name: "missing expiration", claims: jwt.RegisteredClaims{Subject: "api-user", Issuer: "test-issuer", IssuedAt: jwt.NewNumericDate(time.Now())}},
		{name: "missing subject", claims: jwt.RegisteredClaims{Issuer: "test-issuer", IssuedAt: jwt.NewNumericDate(time.Now()), ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			token := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
				UserID:           "api-user",
				UserType:         UserTypeAPI,
				TokenType:        TokenTypeAccess,
				RegisteredClaims: tc.claims,
			})
			tokenString, err := token.SignedString([]byte("test-secret"))
			if err != nil {
				t.Fatalf("sign token: %v", err)
			}
			if _, err := manager.ValidateToken(tokenString); err == nil {
				t.Fatal("expected malformed registered claims to be rejected")
			}
		})
	}
}

func TestValidateTokenRequiresIssuedAt(t *testing.T) {
	manager, err := NewManager(Config{Secret: "test-secret", Issuer: "test-issuer", AccessExpiry: time.Hour})
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		UserID:    "api-user",
		UserType:  UserTypeAPI,
		TokenType: TokenTypeAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "api-user",
			Issuer:    "test-issuer",
			Audience:  jwt.ClaimStrings{AudienceAPI},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	})
	serialized, err := token.SignedString([]byte("test-secret"))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	if _, err := manager.ValidateToken(serialized); !errors.Is(err, ErrInvalidIssuedAt) {
		t.Fatalf("expected ErrInvalidIssuedAt, got %v", err)
	}
}

func TestValidateTokenRequiresExpectedAudience(t *testing.T) {
	manager, err := NewManager(Config{Secret: "test-secret", Issuer: "test-issuer", AccessExpiry: time.Hour})
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	for _, audience := range []jwt.ClaimStrings{nil, {AudienceConsole}, {AudienceAPI, AudienceConsole}} {
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
			UserID:    "api-user",
			UserType:  UserTypeAPI,
			TokenType: TokenTypeAccess,
			RegisteredClaims: jwt.RegisteredClaims{
				Subject:   "api-user",
				Issuer:    "test-issuer",
				Audience:  audience,
				IssuedAt:  jwt.NewNumericDate(time.Now()),
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			},
		})
		serialized, signErr := token.SignedString([]byte("test-secret"))
		if signErr != nil {
			t.Fatalf("sign token: %v", signErr)
		}
		if _, err := manager.ValidateToken(serialized); !errors.Is(err, ErrInvalidAudience) {
			t.Fatalf("audience %v: expected ErrInvalidAudience, got %v", audience, err)
		}
	}
}

func TestParseAccessTokenForUserTypeRejectsCrossSurfaceToken(t *testing.T) {
	manager, err := NewManager(Config{Secret: "test-secret", Issuer: "test-issuer", AccessExpiry: time.Hour})
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	apiToken, err := manager.IssueAccessToken("api-user")
	if err != nil {
		t.Fatalf("issue api token: %v", err)
	}
	if _, err := manager.ParseAccessTokenForUserType(apiToken, UserTypeConsole); err == nil {
		t.Fatal("expected API token to be rejected for console surface")
	}
	consolePair, err := manager.GenerateAdminTokenPair("admin-1", "session-1", UserTypeConsole)
	if err != nil {
		t.Fatalf("issue console token: %v", err)
	}
	if _, err := manager.ParseAccessTokenForUserType(consolePair.AccessToken, UserTypeAPI); err == nil {
		t.Fatal("expected console token to be rejected for API surface")
	}
}
