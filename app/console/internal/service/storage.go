package service

import (
	"context"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/zhimma/grove/pkg/errx"
	pkgstorage "github.com/zhimma/grove/pkg/storage"
)

type StorageService struct {
	manager *pkgstorage.Manager
}

type GetStorageConfigInput struct {
	Disk string
}

type GetAllStorageConfigsOutput struct {
	Default string                    `json:"default"`
	Disks   []pkgstorage.ClientConfig `json:"disks"`
}

type UploadStorageFileInput struct {
	Disk    string
	Purpose string
	File    *multipart.FileHeader
}

type DownloadStorageFileInput struct {
	Disk string
	Path string
}

type DownloadStorageFileOutput struct {
	Reader      io.ReadCloser
	Filename    string
	ContentType string
}

func NewStorageService(manager *pkgstorage.Manager) *StorageService {
	return &StorageService{manager: manager}
}

func (s *StorageService) GetStorageConfig(_ context.Context, in GetStorageConfigInput) (*pkgstorage.ClientConfig, error) {
	if s == nil || s.manager == nil {
		return nil, errx.ServiceUnavailable().WithMessage("存储管理器未配置")
	}
	cfg, err := s.manager.Describe(in.Disk)
	if err != nil {
		return nil, errx.InvalidParams().WithMessage(err.Error())
	}
	return cfg, nil
}

func (s *StorageService) GetAllStorageConfigs(_ context.Context) (*GetAllStorageConfigsOutput, error) {
	if s == nil || s.manager == nil {
		return nil, errx.ServiceUnavailable().WithMessage("存储管理器未配置")
	}
	return &GetAllStorageConfigsOutput{
		Default: s.manager.DefaultDisk(),
		Disks:   s.manager.DescribeAll(),
	}, nil
}

func (s *StorageService) UploadFile(ctx context.Context, in UploadStorageFileInput) (*pkgstorage.StoredFile, error) {
	if s == nil || s.manager == nil {
		return nil, errx.ServiceUnavailable().WithMessage("存储管理器未配置")
	}
	file, err := s.manager.SaveUploadedFile(ctx, in.Disk, in.Purpose, in.File)
	if err != nil {
		if errors.Is(err, pkgstorage.ErrUploadStore) {
			return nil, errx.Internal().WithCause(err)
		}
		if errors.Is(err, pkgstorage.ErrUploadTooLarge) {
			return nil, errx.New(http.StatusRequestEntityTooLarge, "upload_too_large", "文件超过用途策略大小限制")
		}
		if errors.Is(err, pkgstorage.ErrUploadEmpty) ||
			errors.Is(err, pkgstorage.ErrUploadExtension) ||
			errors.Is(err, pkgstorage.ErrUploadMIME) ||
			errors.Is(err, pkgstorage.ErrUploadMagic) ||
			errors.Is(err, pkgstorage.ErrUploadActiveContent) {
			return nil, errx.InvalidParams().
				WithHTTPStatus(http.StatusUnprocessableEntity).
				WithCode("invalid_upload").
				WithMessage(err.Error())
		}
		return nil, errx.InvalidParams().WithMessage(err.Error())
	}
	return file, nil
}

// OpenFile opens a stored object for the protected Console download handler.
// Authorization is deliberately kept at the route/middleware boundary; this
// service only enforces storage-key validation and maps driver failures to the
// stable HTTP error envelope.
func (s *StorageService) OpenFile(ctx context.Context, in DownloadStorageFileInput) (*DownloadStorageFileOutput, error) {
	if s == nil || s.manager == nil {
		return nil, errx.ServiceUnavailable().WithMessage("存储管理器未配置")
	}
	path := strings.TrimSpace(in.Path)
	if path == "" {
		return nil, errx.InvalidParams().WithHTTPStatus(http.StatusUnprocessableEntity).
			WithCode("invalid_storage_path").WithMessage("文件路径不能为空")
	}
	reader, err := s.manager.Open(ctx, in.Disk, path)
	if err != nil {
		switch {
		case errors.Is(err, pkgstorage.ErrObjectPath):
			return nil, errx.InvalidParams().WithHTTPStatus(http.StatusUnprocessableEntity).
				WithCode("invalid_storage_path").WithMessage("文件路径不合法")
		case errors.Is(err, pkgstorage.ErrObjectNotFound):
			return nil, errx.NotFound().WithCode("storage_object_not_found").WithMessage("文件不存在")
		case errors.Is(err, pkgstorage.ErrObjectRead):
			return nil, errx.ServiceUnavailable().WithCode("storage_read_unavailable").WithMessage("当前存储不支持下载")
		default:
			return nil, errx.Internal().WithCause(err)
		}
	}
	filename := filepath.Base(path)
	if filename == "." || filename == "/" || filename == "" {
		filename = "download"
	}
	contentType := "application/octet-stream"
	if guessed := mime.TypeByExtension(strings.ToLower(filepath.Ext(filename))); guessed != "" {
		if mediaType, _, parseErr := mime.ParseMediaType(guessed); parseErr == nil {
			contentType = mediaType
		} else {
			contentType = guessed
		}
	}
	return &DownloadStorageFileOutput{Reader: reader, Filename: filename, ContentType: contentType}, nil
}
