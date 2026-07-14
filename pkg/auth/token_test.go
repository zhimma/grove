package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestIssueAndParseAccessToken(t *testing.T) {
	manager, err := NewManager("test-secret", "test-issuer", 0)
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
	manager, err := NewManager("test-secret", "test-issuer", 0)
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
	manager, err := NewManager("test-secret", "test-issuer", time.Hour, 24*time.Hour)
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
	manager, err := NewManager("test-secret", "test-issuer", time.Hour)
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
