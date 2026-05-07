package storage

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

type LocalConfig struct {
	Root    string
	BaseURL string
	Secret  string
}

type LocalDriver struct {
	root    string
	baseURL string
	secret  string
}

func NewLocalDriver(cfg LocalConfig) (*LocalDriver, error) {
	root := strings.TrimSpace(cfg.Root)
	if root == "" {
		return nil, fmt.Errorf("local storage root is required")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("create local storage root: %w", err)
	}
	secret := strings.TrimSpace(cfg.Secret)
	if secret == "" {
		secret = "grove-local-storage"
	}
	return &LocalDriver{
		root:    root,
		baseURL: strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/"),
		secret:  secret,
	}, nil
}

func (d *LocalDriver) Name() string {
	return "local"
}

func (d *LocalDriver) Put(_ context.Context, objectPath string, content []byte) error {
	fullPath, err := d.fullPath(objectPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return fmt.Errorf("create local storage directory: %w", err)
	}
	if err := os.WriteFile(fullPath, content, 0o600); err != nil {
		return fmt.Errorf("write local object: %w", err)
	}
	return nil
}

func (d *LocalDriver) PutFile(ctx context.Context, objectPath string, file *multipart.FileHeader) (string, error) {
	src, err := file.Open()
	if err != nil {
		return "", fmt.Errorf("open upload file: %w", err)
	}
	defer src.Close()

	content, err := io.ReadAll(src)
	if err != nil {
		return "", fmt.Errorf("read upload file: %w", err)
	}

	targetPath := buildUploadedObjectPath(objectPath, file.Filename)
	if err := d.Put(ctx, targetPath, content); err != nil {
		return "", err
	}
	return targetPath, nil
}

func (d *LocalDriver) Delete(_ context.Context, objectPaths ...string) error {
	for _, objectPath := range objectPaths {
		fullPath, err := d.fullPath(objectPath)
		if err != nil {
			return err
		}
		if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("delete local object: %w", err)
		}
	}
	return nil
}

func (d *LocalDriver) Exists(_ context.Context, objectPath string) (bool, error) {
	fullPath, err := d.fullPath(objectPath)
	if err != nil {
		return false, err
	}
	_, statErr := os.Stat(fullPath)
	if statErr == nil {
		return true, nil
	}
	if os.IsNotExist(statErr) {
		return false, nil
	}
	return false, fmt.Errorf("stat local object: %w", statErr)
}

func (d *LocalDriver) URL(objectPath string) string {
	base := d.baseURL
	if base == "" {
		base = "/storage"
	}
	return strings.TrimRight(base, "/") + "/" + escapeObjectPath(objectPath)
}

func (d *LocalDriver) TemporaryURL(objectPath string, expiry time.Duration) string {
	expiresAt := time.Now().Add(expiry).Unix()
	mac := hmac.New(sha256.New, []byte(d.secret))
	_, _ = mac.Write([]byte(fmt.Sprintf("%s:%d", objectPath, expiresAt)))
	signature := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("%s?expires=%d&signature=%s", d.URL(objectPath), expiresAt, signature)
}

func (d *LocalDriver) fullPath(objectPath string) (string, error) {
	cleanPath := path.Clean("/" + strings.TrimSpace(objectPath))
	if strings.Contains(cleanPath, "..") {
		return "", fmt.Errorf("invalid local storage path")
	}
	fullPath := filepath.Join(d.root, strings.TrimPrefix(cleanPath, "/"))
	absRoot, err := filepath.Abs(d.root)
	if err != nil {
		return "", fmt.Errorf("resolve local root: %w", err)
	}
	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		return "", fmt.Errorf("resolve local path: %w", err)
	}
	if !strings.HasPrefix(absPath, absRoot) {
		return "", fmt.Errorf("local path escapes storage root")
	}
	return absPath, nil
}

func buildUploadedObjectPath(directory, filename string) string {
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(filename)))
	if ext == "" {
		ext = ".bin"
	}
	name := uuid.NewString() + ext
	cleanDir := strings.Trim(strings.TrimSpace(directory), "/")
	if cleanDir == "" {
		return name
	}
	return cleanDir + "/" + name
}

func escapeObjectPath(objectPath string) string {
	parts := strings.Split(strings.TrimLeft(objectPath, "/"), "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return strings.Join(parts, "/")
}
