package api

import (
	"fmt"
	"strconv"

	"github.com/GoSimplicity/AI-CloudOps/internal/files/service"
	"github.com/GoSimplicity/AI-CloudOps/internal/model"
	"github.com/GoSimplicity/AI-CloudOps/pkg/base"
	"github.com/gin-gonic/gin"
)

type FileShareHandler struct {
	svc service.FileShareService
}

func NewFileShareHandler(svc service.FileShareService) *FileShareHandler {
	return &FileShareHandler{svc: svc}
}

func (h *FileShareHandler) RegisterRouters(server *gin.Engine) {
	group := server.Group("/api/files")
	group.POST("/share", h.CreateShare)
	group.GET("/shares", h.ListShares)
	group.GET("/share/:id", h.GetShare)
	group.PUT("/share/:id", h.UpdateShare)
	group.DELETE("/share/:id", h.DeleteShare)
	group.POST("/share/merge", h.MergeShares)
}

// CreateShare 创建分享链接
func (h *FileShareHandler) CreateShare(ctx *gin.Context) {
	var req model.CreateShareReq
	base.HandleRequest(ctx, &req, func() (interface{}, error) {
		// 从上下文获取用户ID
		userID, exists := ctx.Get("user_id")
		if !exists {
			return nil, fmt.Errorf("未登录")
		}
		return h.svc.CreateShare(ctx, userID.(int), &req)
	})
}

// ListShares 获取分享列表
func (h *FileShareHandler) ListShares(ctx *gin.Context) {
	var req model.ShareListReq
	base.HandleRequest(ctx, &req, func() (interface{}, error) {
		shares, total, err := h.svc.ListShares(ctx, &req)
		if err != nil {
			return nil, err
		}
		return model.ListResp[model.FileShare]{
			Items: shares,
			Total: total,
		}, nil
	})
}

// GetShare 获取分享详情
func (h *FileShareHandler) GetShare(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		base.BadRequestError(ctx, "无效的分享ID")
		return
	}

	base.HandleRequest(ctx, nil, func() (interface{}, error) {
		return h.svc.GetShareByID(ctx, uint(id))
	})
}

// UpdateShare 更新分享设置
func (h *FileShareHandler) UpdateShare(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		base.BadRequestError(ctx, "无效的分享ID")
		return
	}

	var req model.UpdateShareReq
	base.HandleRequest(ctx, &req, func() (interface{}, error) {
		return nil, h.svc.UpdateShare(ctx, uint(id), &req)
	})
}

// DeleteShare 删除分享
func (h *FileShareHandler) DeleteShare(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		base.BadRequestError(ctx, "无效的分享ID")
		return
	}

	base.HandleRequest(ctx, nil, func() (interface{}, error) {
		// 从上下文获取用户ID和角色
		userID, _ := ctx.Get("user_id")
		accountType, _ := ctx.Get("account_type")
		isAdmin := accountType.(int8) == 2
		return nil, h.svc.DeleteShare(ctx, uint(id), userID.(int), isAdmin)
	})
}

// MergeShares 合并分享链接
func (h *FileShareHandler) MergeShares(ctx *gin.Context) {
	var req model.MergeSharesReq
	base.HandleRequest(ctx, &req, func() (interface{}, error) {
		return h.svc.MergeShares(ctx, &req)
	})
}
