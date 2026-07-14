package secretbox

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestBoxEncryptDecryptUsesRandomNonce(t *testing.T) {
	box, err := New("0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("new box: %v", err)
	}
	first, err := box.Encrypt("top-secret")
	if err != nil {
		t.Fatalf("encrypt first value: %v", err)
	}
	second, err := box.Encrypt("top-secret")
	if err != nil {
		t.Fatalf("encrypt second value: %v", err)
	}
	if first == second || !strings.HasPrefix(first, "v1:") {
		t.Fatalf("ciphertexts must be versioned and randomized: %q %q", first, second)
	}
	plaintext, err := box.Decrypt(first)
	if err != nil {
		t.Fatalf("decrypt value: %v", err)
	}
	if plaintext != "top-secret" {
		t.Fatalf("unexpected plaintext: %q", plaintext)
	}
}

func TestBoxAcceptsBase64Key(t *testing.T) {
	key := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	box, err := New("base64:" + key)
	if err != nil {
		t.Fatalf("new box with base64 key: %v", err)
	}
	ciphertext, err := box.Encrypt("secret")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if plaintext, err := box.Decrypt(ciphertext); err != nil || plaintext != "secret" {
		t.Fatalf("base64 key round trip: plaintext=%q err=%v", plaintext, err)
	}
}

func TestBoxRejectsWeakAndMalformedKeys(t *testing.T) {
	for _, key := range []string{"", "short", "base64:not-base64", "base64:" + base64.StdEncoding.EncodeToString([]byte("too-short"))} {
		if _, err := New(key); err == nil {
			t.Fatalf("expected key %q to be rejected", key)
		}
	}
}

func TestBoxRejectsWrongKeyTamperingAndMalformedCiphertext(t *testing.T) {
	box, _ := New("0123456789abcdef0123456789abcdef")
	other, _ := New("abcdef0123456789abcdef0123456789")
	ciphertext, err := box.Encrypt("secret")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if _, err := other.Decrypt(ciphertext); err == nil {
		t.Fatal("wrong key must not decrypt ciphertext")
	}

	payload, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(ciphertext, "v1:"))
	if err != nil {
		t.Fatalf("decode ciphertext for tamper test: %v", err)
	}
	payload[len(payload)-1] ^= 0xff
	tampered := "v1:" + base64.RawStdEncoding.EncodeToString(payload)
	if _, err := box.Decrypt(tampered); err == nil {
		t.Fatal("tampered ciphertext must be rejected")
	}
	for _, value := range []string{"", "v2:anything", "v1:not-base64", "v1:YQ=="} {
		if _, err := box.Decrypt(value); err == nil {
			t.Fatalf("malformed ciphertext %q must be rejected", value)
		}
	}
}
