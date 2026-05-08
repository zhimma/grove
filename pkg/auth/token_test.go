package auth

import (
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

	tokenPair, err := manager.GenerateAdminTokenPair("console-admin-demo", "console")
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	claims, err := manager.ParseAccessToken(tokenPair.AccessToken)
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}
	if claims.AdminID != "console-admin-demo" || claims.UserType != "console" {
		t.Fatalf("unexpected console claims: %+v", claims)
	}
	if claims.UserID != "" || claims.Email != "" || claims.RoleID != "" || claims.IsSuper {
		t.Fatalf("unexpected console claims: %+v", claims)
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
