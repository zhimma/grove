package handler

import (
	"errors"
	"mime"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	consoleservice "github.com/zhimma/grove/app/console/internal/service"
	"github.com/zhimma/grove/pkg/errx"
	"github.com/zhimma/grove/pkg/logger"
	"github.com/zhimma/grove/pkg/response"
	"github.com/zhimma/grove/pkg/route"
	"github.com/zhimma/grove/pkg/storage"
	"github.com/zhimma/grove/pkg/validation"
)

type StorageHandler struct {
	service *consoleservice.StorageService
}

type StorageConfigRequest struct {
	Disk string `form:"disk" label:"存储磁盘"`
}

type StorageDownloadRequest struct {
	Disk string `form:"disk" label:"存储磁盘"`
	Path string `form:"path" label:"文件路径"`
}

func RegisterStorageRoutes(protected *gin.RouterGroup, manager *storage.Manager, catalog *route.Catalog) {
	h := &StorageHandler{
		service: consoleservice.NewStorageService(manager),
	}

	group := wrapRoute(protected.Group("/storage"), catalog)
	group.GET("/config", h.Config).Name("文件存储.获取存储配置")
	group.GET("/all-configs", h.AllConfigs).Name("文件存储.获取全部存储配置")
	group.POST("/upload", h.Upload).Name("文件存储.上传文件")
	group.GET("/download", h.Download).Name("文件存储.下载文件")
}

func (h *StorageHandler) Config(c *gin.Context) {
	var req StorageConfigRequest
	if err := validation.BindQuery(c, &req); err != nil {
		response.Fail(c, err)
		return
	}
	result, err := h.service.GetStorageConfig(c.Request.Context(), consoleservice.GetStorageConfigInput{
		Disk: req.Disk,
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, result)
}

func (h *StorageHandler) AllConfigs(c *gin.Context) {
	result, err := h.service.GetAllStorageConfigs(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, result)
}

func (h *StorageHandler) Upload(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			response.Fail(c, errx.RequestBodyTooLarge(maxBytesErr.Limit))
			return
		}
		response.Fail(c, "file is required")
		return
	}
	result, serviceErr := h.service.UploadFile(c.Request.Context(), consoleservice.UploadStorageFileInput{
		Disk:    c.PostForm("disk"),
		Purpose: c.PostForm("purpose"),
		File:    file,
	})
	if serviceErr != nil {
		response.Fail(c, serviceErr)
		return
	}
	setAuditMeta(c, "storage_object", result.Path, map[string]any{
		"disk":      result.Disk,
		"driver":    result.Driver,
		"path":      result.Path,
		"filename":  result.Filename,
		"size":      result.Size,
		"purpose":   result.Purpose,
		"mime_type": result.ContentType,
	})
	response.Success(c, result)
}

func (h *StorageHandler) Download(c *gin.Context) {
	var req StorageDownloadRequest
	if err := validation.BindQuery(c, &req); err != nil {
		response.Fail(c, err)
		return
	}
	result, err := h.service.OpenFile(c.Request.Context(), consoleservice.DownloadStorageFileInput{
		Disk: req.Disk,
		Path: req.Path,
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	defer func() {
		if closeErr := result.Reader.Close(); closeErr != nil {
			logger.Warn().Err(closeErr).Str("disk", req.Disk).Str("path", req.Path).Msg("关闭存储对象失败")
		}
	}()

	filename := sanitizeDownloadFilename(result.Filename)
	contentDisposition := mime.FormatMediaType("attachment", map[string]string{"filename": filename})
	c.Header("X-Content-Type-Options", "nosniff")
	c.DataFromReader(http.StatusOK, -1, result.ContentType, result.Reader, map[string]string{
		"Content-Disposition": contentDisposition,
	})
}

func sanitizeDownloadFilename(filename string) string {
	filename = strings.TrimSpace(filename)
	if filename == "" {
		return "download"
	}
	filename = strings.NewReplacer("\\", "_", "/", "_", "\r", "_", "\n", "_").Replace(filename)
	if filename == "." || filename == ".." {
		return "download"
	}
	return filename
}
