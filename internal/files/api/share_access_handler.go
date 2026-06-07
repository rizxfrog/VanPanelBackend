package api

import (
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/rizxfrog/VanPanelBackend/internal/files/service"
	"github.com/rizxfrog/VanPanelBackend/internal/model"
	"github.com/rizxfrog/VanPanelBackend/pkg/base"
)

type ShareAccessHandler struct {
	svc service.FileShareService
}

func NewShareAccessHandler(svc service.FileShareService) *ShareAccessHandler {
	return &ShareAccessHandler{svc: svc}
}

func (h *ShareAccessHandler) RegisterRouters(server *gin.Engine) {
	group := server.Group("/api/share")
	group.GET("/:code", h.GetShareInfo)
	group.POST("/:code/verify", h.VerifyAccess)
	group.GET("/:code/files", h.GetShareFiles)
	group.GET("/:code/download", h.DownloadFile)
}

// GetShareInfo 获取分享信息
func (h *ShareAccessHandler) GetShareInfo(ctx *gin.Context) {
	code := ctx.Param("code")
	if code == "" {
		base.BadRequestError(ctx, "分享码不能为空")
		return
	}

	base.HandleRequest(ctx, nil, func() (interface{}, error) {
		return h.svc.GetShareByCode(ctx, code)
	})
}

// VerifyAccess 验证访问权限
func (h *ShareAccessHandler) VerifyAccess(ctx *gin.Context) {
	code := ctx.Param("code")
	if code == "" {
		base.BadRequestError(ctx, "分享码不能为空")
		return
	}

	var req model.VerifyAccessReq
	base.HandleRequest(ctx, &req, func() (interface{}, error) {
		return h.svc.VerifyAccess(ctx, code, &req)
	})
}

// GetShareFiles 获取分享文件列表
func (h *ShareAccessHandler) GetShareFiles(ctx *gin.Context) {
	code := ctx.Param("code")
	if code == "" {
		base.BadRequestError(ctx, "分享码不能为空")
		return
	}

	base.HandleRequest(ctx, nil, func() (interface{}, error) {
		return h.svc.GetShareFiles(ctx, code)
	})
}

// DownloadFile 下载文件
func (h *ShareAccessHandler) DownloadFile(ctx *gin.Context) {
	code := ctx.Param("code")
	if code == "" {
		base.BadRequestError(ctx, "分享码不能为空")
		return
	}

	accessCode := ctx.Query("access_code")
	if accessCode == "" {
		base.BadRequestError(ctx, "提取码不能为空")
		return
	}

	filePath := ctx.Query("file_path")
	if filePath == "" {
		base.BadRequestError(ctx, "文件路径不能为空")
		return
	}

	// 验证并获取文件路径
	resolvedPath, err := h.svc.DownloadFile(ctx, code, accessCode, filePath)
	if err != nil {
		base.ErrorWithMessage(ctx, err.Error())
		return
	}

	// 设置响应头
	ctx.Header("Content-Disposition", "attachment; filename="+filepath.Base(resolvedPath))
	ctx.File(resolvedPath)
	ctx.Status(http.StatusOK)
}
