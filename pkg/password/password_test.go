package password

import (
	"errors"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func TestHashVerifiesAndIsSalted(t *testing.T) {
	first, err := Hash("correct horse")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	second, err := Hash("correct horse")
	if err != nil {
		t.Fatalf("hash again: %v", err)
	}

	if first == second {
		t.Fatal("two hashes of the same password are identical, so the salt is not being applied")
	}
	for _, hash := range []string{first, second} {
		if !Verify(hash, "correct horse") {
			t.Fatalf("Verify rejected the password it hashed: %s", hash)
		}
		if Verify(hash, "wrong horse") {
			t.Fatal("Verify accepted the wrong password")
		}
	}
}

// An empty password would otherwise hash into something a login could match.
func TestHashRejectsAnEmptyPassword(t *testing.T) {
	if _, err := Hash(""); !errors.Is(err, ErrEmpty) {
		t.Fatalf("Hash(\"\") error = %v, want ErrEmpty", err)
	}
}

func TestVerifyRejectsEmptyInputAndDamagedHashes(t *testing.T) {
	valid, err := Hash("secret")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}

	cases := map[string]struct{ hash, plain string }{
		"empty hash":       {"", "secret"},
		"empty password":   {valid, ""},
		"both empty":       {"", ""},
		"not a hash":       {"plaintext-in-the-column", "secret"},
		"truncated hash":   {valid[:len(valid)-5], "secret"},
		"wrong cost field": {strings.Replace(valid, "$10$", "$zz$", 1), "secret"},
	}
	for name, input := range cases {
		if Verify(input.hash, input.plain) {
			t.Errorf("%s: Verify returned true", name)
		}
	}
}

// The hash VerifyMiss compares against has to be a real bcrypt hash. If it
// were malformed, bcrypt would reject it immediately and the unknown-account
// path would return faster than a wrong password — the timing difference the
// function exists to remove.
func TestVerifyMissComparesAgainstARealHashAtTheSameCost(t *testing.T) {
	cost, err := bcrypt.Cost([]byte(missHash))
	if err != nil {
		t.Fatalf("missHash is not a valid bcrypt hash: %v", err)
	}
	if cost != bcrypt.DefaultCost {
		t.Fatalf("missHash cost = %d, want %d so a miss costs what a real check costs", cost, bcrypt.DefaultCost)
	}

	// Nothing should ever match it.
	if Verify(missHash, "") || Verify(missHash, "password") || Verify(missHash, missHash) {
		t.Fatal("something matched the miss hash")
	}
}

func TestVerifyMissSpendsComparableTimeToVerify(t *testing.T) {
	hash, err := Hash("real password")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}

	start := time.Now()
	Verify(hash, "wrong password")
	hit := time.Since(start)

	start = time.Now()
	VerifyMiss("wrong password")
	miss := time.Since(start)

	// Same cost factor, so the two should be within an order of magnitude even
	// on a loaded machine. A malformed missHash would return ~instantly.
	if miss*10 < hit {
		t.Fatalf("miss path took %v against %v for a real check, fast enough to enumerate accounts", miss, hit)
	}
}
