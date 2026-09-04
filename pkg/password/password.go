// Package password hashes and verifies account passwords. It exists so the
// cost factor and the constant-time miss path are decided once: the same
// choices were previously repeated at nine call sites across the CLI and the
// console services.
package password

import (
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// ErrEmpty is returned rather than hashing an empty string, which bcrypt would
// otherwise accept and produce a usable hash for.
var ErrEmpty = errors.New("password is required")

// missHash is a valid bcrypt hash of a value nothing can match. VerifyMiss
// compares against it so a login for an unknown account spends the same time
// as one for a known account with the wrong password.
const missHash = "$2y$10$WuzL7jUB./OeDcEIx.eBV.WkSEyl5dY1uxdfxdCq4hZDMFfeU1ZGC"

// Hash returns a bcrypt hash of plain at the cost the whole repository shares.
func Hash(plain string) (string, error) {
	if plain == "" {
		return "", ErrEmpty
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hashed), nil
}

// Verify reports whether plain matches hash.
//
// It deliberately returns a bool rather than an error: every caller only needs
// to know whether the credential was right, and a bcrypt error means the stored
// hash is unusable, which is a failed verification either way.
func Verify(hash, plain string) bool {
	if hash == "" || plain == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

// VerifyMiss spends a verification's worth of time without a stored hash.
//
// Call it on the path where the account does not exist. Returning early there
// makes an unknown account measurably faster than a wrong password, which turns
// login timing into an account-enumeration oracle.
func VerifyMiss(plain string) {
	_ = bcrypt.CompareHashAndPassword([]byte(missHash), []byte(plain))
}
