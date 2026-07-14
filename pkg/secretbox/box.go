package secretbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
)

const version = "v1"

type Box struct {
	aead cipher.AEAD
}

func New(input string) (*Box, error) {
	key, err := parseKey(input)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(key)
	block, err := aes.NewCipher(digest[:])
	if err != nil {
		return nil, fmt.Errorf("create AES cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create AES-GCM: %w", err)
	}
	return &Box{aead: aead}, nil
}

func (b *Box) Encrypt(plaintext string) (string, error) {
	if b == nil || b.aead == nil {
		return "", fmt.Errorf("secret box is not configured")
	}
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate secret nonce: %w", err)
	}
	payload := make([]byte, 0, len(nonce)+len(plaintext)+b.aead.Overhead())
	payload = append(payload, nonce...)
	payload = b.aead.Seal(payload, nonce, []byte(plaintext), []byte(version))
	return version + ":" + base64.RawStdEncoding.EncodeToString(payload), nil
}

func (b *Box) Decrypt(ciphertext string) (string, error) {
	if b == nil || b.aead == nil {
		return "", fmt.Errorf("secret box is not configured")
	}
	prefix, encoded, ok := strings.Cut(strings.TrimSpace(ciphertext), ":")
	if !ok || prefix != version || encoded == "" {
		return "", fmt.Errorf("unsupported secret ciphertext format")
	}
	payload, err := base64.RawStdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("decode secret ciphertext: %w", err)
	}
	nonceSize := b.aead.NonceSize()
	if len(payload) < nonceSize+b.aead.Overhead() {
		return "", fmt.Errorf("secret ciphertext is too short")
	}
	nonce := payload[:nonceSize]
	plaintext, err := b.aead.Open(nil, nonce, payload[nonceSize:], []byte(version))
	if err != nil {
		return "", fmt.Errorf("decrypt secret ciphertext: %w", err)
	}
	return string(plaintext), nil
}

func parseKey(input string) ([]byte, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, fmt.Errorf("config encryption key is required")
	}
	if encoded, ok := strings.CutPrefix(input, "base64:"); ok {
		key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
		if err != nil {
			return nil, fmt.Errorf("decode config encryption key: %w", err)
		}
		if len(key) != 32 {
			return nil, fmt.Errorf("base64 config encryption key must decode to 32 bytes")
		}
		return key, nil
	}
	if len([]byte(input)) < 32 {
		return nil, fmt.Errorf("config encryption key must be at least 32 bytes")
	}
	return []byte(input), nil
}
