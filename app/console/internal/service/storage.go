package service

import (
	"context"
	"errors"
	"mime/multipart"
	"net/http"

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
