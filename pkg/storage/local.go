package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

type LocalConfig struct {
	Root    string
	BaseURL string
}

type LocalDriver struct {
	root    string
	baseURL string
}

func NewLocalDriver(cfg LocalConfig) (*LocalDriver, error) {
	root := strings.TrimSpace(cfg.Root)
	if root == "" {
		return nil, fmt.Errorf("local storage root is required")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("create local storage root: %w", err)
	}
	return &LocalDriver{
		root:    root,
		baseURL: strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/"),
	}, nil
}

func (d *LocalDriver) Name() string {
	return "local"
}

func (d *LocalDriver) Put(ctx context.Context, objectPath string, content []byte) error {
	return d.PutStream(ctx, objectPath, bytes.NewReader(content), int64(len(content)), contentTypeByPath(objectPath))
}

func (d *LocalDriver) PutStream(ctx context.Context, objectPath string, reader io.Reader, size int64, _ string) error {
	if reader == nil || size < 0 {
		return fmt.Errorf("invalid local object stream")
	}
	fullPath, err := d.fullPath(objectPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return fmt.Errorf("create local storage directory: %w", err)
	}
	tempFile, err := os.CreateTemp(filepath.Dir(fullPath), ".grove-upload-*")
	if err != nil {
		return fmt.Errorf("create local temporary object: %w", err)
	}
	tempPath := tempFile.Name()
	committed := false
	defer func() {
		_ = tempFile.Close()
		if !committed {
			_ = os.Remove(tempPath)
		}
	}()

	written, err := io.Copy(tempFile, io.LimitReader(contextReader{ctx: ctx, reader: reader}, size+1))
	if err != nil {
		return fmt.Errorf("write local object: %w", err)
	}
	if written != size {
		return fmt.Errorf("write local object: expected %d bytes, copied %d", size, written)
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("close local object: %w", err)
	}
	if err := os.Rename(tempPath, fullPath); err != nil {
		return fmt.Errorf("commit local object: %w", err)
	}
	committed = true
	return nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	select {
	case <-r.ctx.Done():
		return 0, r.ctx.Err()
	default:
		return r.reader.Read(p)
	}
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

// Open returns a read-only stream for an object. The manager validates the
// public object key before calling this method; the driver repeats its root
// containment check as a defense in depth for direct callers.
func (d *LocalDriver) Open(_ context.Context, objectPath string) (io.ReadCloser, error) {
	fullPath, err := d.fullPath(objectPath)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrObjectNotFound
		}
		return nil, fmt.Errorf("open local object: %w", err)
	}
	return file, nil
}

func (d *LocalDriver) URL(objectPath string) string {
	base := d.baseURL
	if base == "" {
		base = "/storage"
	}
	return strings.TrimRight(base, "/") + "/" + escapeObjectPath(objectPath)
}

func (d *LocalDriver) fullPath(objectPath string) (string, error) {
	cleanPath, err := validateObjectPath(objectPath)
	if err != nil {
		return "", fmt.Errorf("invalid local storage path")
	}
	fullPath := filepath.Join(d.root, cleanPath)
	absRoot, err := filepath.Abs(d.root)
	if err != nil {
		return "", fmt.Errorf("resolve local root: %w", err)
	}
	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		return "", fmt.Errorf("resolve local path: %w", err)
	}
	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil {
		return "", fmt.Errorf("resolve local relative path: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
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
