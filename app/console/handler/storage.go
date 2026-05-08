package handler

import (
	"github.com/gin-gonic/gin"

	consoleservice "github.com/zhimma/grove/app/console/service"
	"github.com/zhimma/grove/internal/provider"
	"github.com/zhimma/grove/pkg/reqctx"
	"github.com/zhimma/grove/pkg/response"
	"github.com/zhimma/grove/pkg/route"
	"github.com/zhimma/grove/pkg/validation"
)

type StorageHandler struct {
	service *consoleservice.StorageService
}

type StorageConfigRequest struct {
	Disk string `form:"disk" label:"存储磁盘"`
}

func RegisterStorageRoutes(protected *gin.RouterGroup, p *provider.Provider) {
	h := &StorageHandler{
		service: consoleservice.NewStorageService(p.Storage),
	}

	group := route.Wrap(protected.Group("/storage"))
	group.GET("/config", h.Config).Name("文件存储.获取存储配置")
	group.GET("/all-configs", h.AllConfigs).Name("文件存储.获取全部存储配置")
	group.POST("/upload", h.Upload).Name("文件存储.上传文件")
}

func (h *StorageHandler) Config(c *gin.Context) {
	var req StorageConfigRequest
	if err := validation.BindQuery(c, &req); err != nil {
		response.Fail(c, err)
		return
	}
	result, err := h.service.GetStorageConfig(c.Request.Context(), consoleservice.GetStorageConfigInput{
		UserID: reqctx.GetAdminID(c),
		Disk:   req.Disk,
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
		response.Fail(c, "file is required")
		return
	}
	result, serviceErr := h.service.UploadFile(c.Request.Context(), consoleservice.UploadStorageFileInput{
		UserID:    reqctx.GetAdminID(c),
		Disk:      c.PostForm("disk"),
		Directory: c.PostForm("directory"),
		File:      file,
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
		"directory": c.PostForm("directory"),
	})
	response.Success(c, result)
}
