package api

import (
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"
	filemodel "github.com/rizxfrog/VanPanelBackend/internal/files/model"
	"github.com/rizxfrog/VanPanelBackend/internal/files/service"
	"github.com/rizxfrog/VanPanelBackend/pkg/base"
)

type FileHandler struct {
	svc service.FileService
}

func NewFileHandler(svc service.FileService) *FileHandler {
	return &FileHandler{svc: svc}
}

func (h *FileHandler) RegisterRouters(server *gin.Engine) {
	group := server.Group("/api/files")
	group.GET("/roots", h.Roots)
	group.POST("/list", h.List)
	group.POST("/content", h.Content)
	group.POST("/save", h.Save)
	group.POST("/create", h.Create)
	group.POST("/rename", h.Rename)
	group.POST("/delete", h.Delete)
	group.POST("/move", h.Move)
	group.POST("/chmod", h.Chmod)
	group.POST("/chown", h.Chown)
	group.POST("/compress", h.Compress)
	group.POST("/decompress", h.Decompress)
	group.POST("/upload", h.Upload)
	group.GET("/download", h.Download)
}

func (h *FileHandler) Roots(ctx *gin.Context) {
	base.HandleRequest(ctx, nil, func() (interface{}, error) {
		return h.svc.Roots(ctx)
	})
}

func (h *FileHandler) List(ctx *gin.Context) {
	var req filemodel.ListRequest
	base.HandleRequest(ctx, &req, func() (interface{}, error) {
		return h.svc.List(ctx, req)
	})
}

func (h *FileHandler) Content(ctx *gin.Context) {
	var req filemodel.ContentRequest
	base.HandleRequest(ctx, &req, func() (interface{}, error) {
		return h.svc.Content(ctx, req)
	})
}

func (h *FileHandler) Save(ctx *gin.Context) {
	var req filemodel.SaveRequest
	base.HandleRequest(ctx, &req, func() (interface{}, error) {
		return nil, h.svc.Save(ctx, req)
	})
}

func (h *FileHandler) Create(ctx *gin.Context) {
	var req filemodel.CreateRequest
	base.HandleRequest(ctx, &req, func() (interface{}, error) {
		return nil, h.svc.Create(ctx, req)
	})
}

func (h *FileHandler) Rename(ctx *gin.Context) {
	var req filemodel.RenameRequest
	base.HandleRequest(ctx, &req, func() (interface{}, error) {
		return nil, h.svc.Rename(ctx, req)
	})
}

func (h *FileHandler) Delete(ctx *gin.Context) {
	var req filemodel.DeleteRequest
	base.HandleRequest(ctx, &req, func() (interface{}, error) {
		return nil, h.svc.Delete(ctx, req)
	})
}

func (h *FileHandler) Move(ctx *gin.Context) {
	var req filemodel.MoveRequest
	base.HandleRequest(ctx, &req, func() (interface{}, error) {
		return nil, h.svc.Move(ctx, req)
	})
}

func (h *FileHandler) Chmod(ctx *gin.Context) {
	var req filemodel.ChmodRequest
	base.HandleRequest(ctx, &req, func() (interface{}, error) {
		return nil, h.svc.Chmod(ctx, req)
	})
}

func (h *FileHandler) Chown(ctx *gin.Context) {
	var req filemodel.ChownRequest
	base.HandleRequest(ctx, &req, func() (interface{}, error) {
		return nil, h.svc.Chown(ctx, req)
	})
}

func (h *FileHandler) Compress(ctx *gin.Context) {
	var req filemodel.CompressRequest
	base.HandleRequest(ctx, &req, func() (interface{}, error) {
		return nil, h.svc.Compress(ctx, req)
	})
}

func (h *FileHandler) Decompress(ctx *gin.Context) {
	var req filemodel.DecompressRequest
	base.HandleRequest(ctx, &req, func() (interface{}, error) {
		return nil, h.svc.Decompress(ctx, req)
	})
}

func (h *FileHandler) Upload(ctx *gin.Context) {
	targetPath := ctx.PostForm("path")
	if targetPath == "" {
		base.BadRequestError(ctx, "path is required")
		return
	}
	fileHeader, err := ctx.FormFile("file")
	if err != nil {
		base.BadRequestError(ctx, err.Error())
		return
	}
	dst, err := h.svc.ResolveUploadPath(ctx, filemodel.TargetRequest{TargetType: ctx.PostForm("target_type")}, targetPath, fileHeader.Filename)
	if err != nil {
		base.ErrorWithMessage(ctx, err.Error())
		return
	}
	if err := ctx.SaveUploadedFile(fileHeader, dst); err != nil {
		base.ErrorWithMessage(ctx, err.Error())
		return
	}
	base.Success(ctx)
}

func (h *FileHandler) Download(ctx *gin.Context) {
	pathValue := ctx.Query("path")
	if pathValue == "" {
		base.BadRequestError(ctx, "path is required")
		return
	}
	ctx.Header("Content-Disposition", "attachment; filename="+filepath.Base(pathValue))
	ctx.File(pathValue)
	ctx.Status(http.StatusOK)
}
