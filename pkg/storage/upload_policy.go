package storage

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path"
	"path/filepath"
	"strings"
)

var (
	ErrUploadEmpty         = errors.New("upload file is empty")
	ErrUploadTooLarge      = errors.New("upload file is too large")
	ErrUploadExtension     = errors.New("upload file extension is not allowed")
	ErrUploadMIME          = errors.New("upload file MIME type is not allowed")
	ErrUploadMagic         = errors.New("upload file signature does not match extension")
	ErrUploadActiveContent = errors.New("active content cannot be publicly hosted")
	ErrUploadStore         = errors.New("store uploaded file failed")
)

type UploadPolicyConfig struct {
	Name       string
	Directory  string
	MaxBytes   int64
	Extensions []string
	MIMETypes  []string
}

type UploadPolicy struct {
	name       string
	directory  string
	maxBytes   int64
	extensions map[string]struct{}
	mimeTypes  map[string]struct{}
}

type ValidatedUpload struct {
	Reader      io.Reader
	Filename    string
	Extension   string
	Directory   string
	Size        int64
	ContentType string
}

func NewUploadPolicy(cfg UploadPolicyConfig) (UploadPolicy, error) {
	name := strings.ToLower(strings.TrimSpace(cfg.Name))
	if name == "" {
		return UploadPolicy{}, fmt.Errorf("upload policy name is required")
	}
	if cfg.MaxBytes <= 0 {
		return UploadPolicy{}, fmt.Errorf("upload policy %q max bytes must be positive", name)
	}
	directory := strings.Trim(strings.TrimSpace(cfg.Directory), "/")
	cleanDirectory := strings.TrimPrefix(path.Clean("/"+directory), "/")
	if directory == "" || cleanDirectory != directory {
		return UploadPolicy{}, fmt.Errorf("upload policy %q directory is invalid", name)
	}
	extensions := make(map[string]struct{}, len(cfg.Extensions))
	for _, value := range cfg.Extensions {
		ext := normalizeExtension(value)
		if ext != "" {
			extensions[ext] = struct{}{}
		}
	}
	if len(extensions) == 0 {
		return UploadPolicy{}, fmt.Errorf("upload policy %q requires extensions", name)
	}
	mimeTypes := make(map[string]struct{}, len(cfg.MIMETypes))
	for _, value := range cfg.MIMETypes {
		value = normalizeMIME(value)
		if value != "" {
			mimeTypes[value] = struct{}{}
		}
	}
	if len(mimeTypes) == 0 {
		return UploadPolicy{}, fmt.Errorf("upload policy %q requires MIME types", name)
	}
	return UploadPolicy{
		name:       name,
		directory:  directory,
		maxBytes:   cfg.MaxBytes,
		extensions: extensions,
		mimeTypes:  mimeTypes,
	}, nil
}

func DefaultUploadPolicyConfigs() []UploadPolicyConfig {
	return []UploadPolicyConfig{
		{
			Name:       "avatar",
			Directory:  "avatars",
			MaxBytes:   5 * 1024 * 1024,
			Extensions: []string{".jpg", ".jpeg", ".png", ".gif", ".webp"},
			MIMETypes:  []string{"image/jpeg", "image/png", "image/gif", "image/webp"},
		},
		{
			Name:       "document",
			Directory:  "documents",
			MaxBytes:   20 * 1024 * 1024,
			Extensions: []string{".pdf", ".txt", ".csv", ".doc", ".docx", ".xls", ".xlsx"},
			MIMETypes: []string{
				"application/pdf",
				"text/plain",
				"text/csv",
				"application/x-ole-storage",
				"application/zip",
				"application/octet-stream",
			},
		},
	}
}

func (p UploadPolicy) Name() string {
	return p.name
}

func (p UploadPolicy) MaxBytes() int64 {
	return p.maxBytes
}

func (p UploadPolicy) Inspect(filename string, size int64, reader io.Reader) (*ValidatedUpload, error) {
	if size <= 0 || reader == nil {
		return nil, ErrUploadEmpty
	}
	if size > p.maxBytes {
		return nil, fmt.Errorf("%w: maximum is %d bytes", ErrUploadTooLarge, p.maxBytes)
	}
	ext := normalizeExtension(filepath.Ext(strings.TrimSpace(filename)))
	if _, ok := p.extensions[ext]; !ok {
		return nil, fmt.Errorf("%w: %s", ErrUploadExtension, ext)
	}

	head := make([]byte, 512)
	n, err := io.ReadFull(reader, head)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, fmt.Errorf("read upload signature: %w", err)
	}
	head = head[:n]
	if len(head) == 0 {
		return nil, ErrUploadEmpty
	}
	detectedMIME := normalizeMIME(http.DetectContentType(head))
	if isActiveExtension(ext) || isActiveMIME(detectedMIME) {
		return nil, fmt.Errorf("%w: %s", ErrUploadActiveContent, detectedMIME)
	}
	if !p.allowsMIME(detectedMIME) {
		return nil, fmt.Errorf("%w: %s", ErrUploadMIME, detectedMIME)
	}
	if !matchesFileSignature(ext, detectedMIME, head) {
		return nil, fmt.Errorf("%w: %s is %s", ErrUploadMagic, ext, detectedMIME)
	}

	return &ValidatedUpload{
		Reader:      io.MultiReader(bytes.NewReader(head), reader),
		Filename:    strings.TrimSpace(filename),
		Extension:   ext,
		Directory:   p.directory,
		Size:        size,
		ContentType: detectedMIME,
	}, nil
}

func (p UploadPolicy) allowsMIME(value string) bool {
	if _, ok := p.mimeTypes[value]; ok {
		return true
	}
	major, _, ok := strings.Cut(value, "/")
	if !ok {
		return false
	}
	_, ok = p.mimeTypes[major+"/*"]
	return ok
}

func normalizeExtension(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" || value == "." {
		return ""
	}
	if !strings.HasPrefix(value, ".") {
		value = "." + value
	}
	return value
}

func normalizeMIME(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	mediaType, _, err := mime.ParseMediaType(value)
	if err != nil {
		return value
	}
	return strings.ToLower(mediaType)
}

func isActiveExtension(ext string) bool {
	switch ext {
	case ".css", ".htm", ".html", ".js", ".mjs", ".svg", ".xhtml", ".xml":
		return true
	default:
		return false
	}
}

func isActiveMIME(value string) bool {
	switch value {
	case "application/javascript", "application/xhtml+xml", "application/xml", "image/svg+xml", "text/css", "text/html", "text/javascript", "text/xml":
		return true
	default:
		return false
	}
}

func matchesFileSignature(ext, detectedMIME string, head []byte) bool {
	switch ext {
	case ".jpg", ".jpeg":
		return len(head) >= 3 && head[0] == 0xff && head[1] == 0xd8 && head[2] == 0xff
	case ".png":
		return bytes.HasPrefix(head, []byte("\x89PNG\r\n\x1a\n"))
	case ".gif":
		return bytes.HasPrefix(head, []byte("GIF87a")) || bytes.HasPrefix(head, []byte("GIF89a"))
	case ".webp":
		return len(head) >= 12 && bytes.Equal(head[:4], []byte("RIFF")) && bytes.Equal(head[8:12], []byte("WEBP"))
	case ".pdf":
		return bytes.HasPrefix(head, []byte("%PDF-"))
	case ".doc", ".xls":
		return bytes.HasPrefix(head, []byte{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1})
	case ".docx", ".xlsx":
		return bytes.HasPrefix(head, []byte("PK\x03\x04")) || bytes.HasPrefix(head, []byte("PK\x05\x06"))
	case ".csv", ".txt":
		return detectedMIME == "text/plain" || detectedMIME == "text/csv"
	default:
		return true
	}
}
