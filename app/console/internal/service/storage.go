package service

import (
	"context"
	"mime/multipart"
	"strings"

	"github.com/zhimma/grove/pkg/errx"
	pkgstorage "github.com/zhimma/grove/pkg/storage"
)

type StorageService struct {
	manager *pkgstorage.Manager
}

type GetStorageConfigInput struct {
	UserID string
	Disk   string
}

type GetAllStorageConfigsOutput struct {
	Default string                    `json:"default"`
	Disks   []pkgstorage.ClientConfig `json:"disks"`
}

type UploadStorageFileInput struct {
	UserID    string
	Disk      string
	Directory string
	File      *multipart.FileHeader
}

func NewStorageService(manager *pkgstorage.Manager) *StorageService {
	return &StorageService{manager: manager}
}

func (s *StorageService) GetStorageConfig(ctx context.Context, in GetStorageConfigInput) (*pkgstorage.ClientConfig, error) {
	if s == nil || s.manager == nil {
		return nil, errx.ServiceUnavailable().WithMessage("存储管理器未配置")
	}
	userID := strings.TrimSpace(in.UserID)
	if userID == "" {
		userID = "anonymous"
	}
	cfg, err := s.manager.IssueClientConfig(ctx, in.Disk, userID)
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
	file, err := s.manager.SaveUploadedFile(ctx, in.Disk, in.Directory, in.File)
	if err != nil {
		return nil, errx.InvalidParams().WithMessage(err.Error())
	}
	return file, nil
}
