package api

import (
	"net/http"
	"path/filepath"

	filemodel "github.com/GoSimplicity/AI-CloudOps/internal/files/model"
	"github.com/GoSimplicity/AI-CloudOps/internal/files/service"
	"github.com/GoSimplicity/AI-CloudOps/pkg/base"
	"github.com/gin-gonic/gin"
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
