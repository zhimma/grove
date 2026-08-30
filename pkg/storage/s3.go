package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime"
	"path/filepath"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type S3Config struct {
	Endpoint  string
	Region    string
	Bucket    string
	AccessKey string
	SecretKey string
	Secure    bool
	BaseURL   string
}

type S3Driver struct {
	client   *minio.Client
	bucket   string
	baseURL  string
	endpoint string
}

func NewS3Driver(cfg S3Config) (*S3Driver, error) {
	if strings.TrimSpace(cfg.Endpoint) == "" || strings.TrimSpace(cfg.Bucket) == "" {
		return nil, fmt.Errorf("s3 endpoint and bucket are required")
	}
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Region: cfg.Region,
		Secure: cfg.Secure,
	})
	if err != nil {
		return nil, fmt.Errorf("create s3 client: %w", err)
	}

	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		scheme := "http"
		if cfg.Secure {
			scheme = "https"
		}
		baseURL = fmt.Sprintf("%s://%s/%s", scheme, cfg.Endpoint, cfg.Bucket)
	}

	return &S3Driver{
		client:   client,
		bucket:   cfg.Bucket,
		baseURL:  baseURL,
		endpoint: cfg.Endpoint,
	}, nil
}

func (d *S3Driver) Name() string {
	return "s3"
}

func (d *S3Driver) Put(ctx context.Context, objectPath string, content []byte) error {
	return d.PutStream(ctx, objectPath, bytes.NewReader(content), int64(len(content)), contentTypeByPath(objectPath))
}

func (d *S3Driver) PutStream(ctx context.Context, objectPath string, reader io.Reader, size int64, contentType string) error {
	if strings.TrimSpace(contentType) == "" {
		contentType = "application/octet-stream"
	}
	_, err := d.client.PutObject(ctx, d.bucket, objectPath, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return fmt.Errorf("put s3 object: %w", err)
	}
	return nil
}

func (d *S3Driver) Delete(ctx context.Context, objectPaths ...string) error {
	for _, objectPath := range objectPaths {
		err := d.client.RemoveObject(ctx, d.bucket, objectPath, minio.RemoveObjectOptions{})
		if err != nil {
			return fmt.Errorf("delete s3 object: %w", err)
		}
	}
	return nil
}

func (d *S3Driver) Exists(ctx context.Context, objectPath string) (bool, error) {
	_, err := d.client.StatObject(ctx, d.bucket, objectPath, minio.StatObjectOptions{})
	if err == nil {
		return true, nil
	}
	resp := minio.ToErrorResponse(err)
	if resp.Code == "NoSuchKey" || resp.Code == "NoSuchObject" || resp.Code == "NotFound" {
		return false, nil
	}
	return false, fmt.Errorf("stat s3 object: %w", err)
}

func (d *S3Driver) Open(ctx context.Context, objectPath string) (io.ReadCloser, error) {
	object, err := d.client.GetObject(ctx, d.bucket, objectPath, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("get s3 object: %w", err)
	}
	// GetObject is lazy in minio-go; Stat makes missing objects fail before the
	// HTTP response is committed by a download handler.
	if _, err := object.Stat(); err != nil {
		_ = object.Close()
		resp := minio.ToErrorResponse(err)
		if resp.Code == "NoSuchKey" || resp.Code == "NoSuchObject" || resp.Code == "NotFound" {
			return nil, ErrObjectNotFound
		}
		return nil, fmt.Errorf("stat s3 object: %w", err)
	}
	return object, nil
}

func (d *S3Driver) URL(objectPath string) string {
	return strings.TrimRight(d.baseURL, "/") + "/" + escapeObjectPath(objectPath)
}

func contentTypeByPath(objectPath string) string {
	if guessed := mime.TypeByExtension(strings.ToLower(filepath.Ext(objectPath))); guessed != "" {
		return guessed
	}
	return "application/octet-stream"
}
