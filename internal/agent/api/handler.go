package api

import (
	"strconv"

	agentmodel "github.com/GoSimplicity/AI-CloudOps/internal/agent/model"
	agentservice "github.com/GoSimplicity/AI-CloudOps/internal/agent/service"
	"github.com/GoSimplicity/AI-CloudOps/pkg/base"
	"github.com/GoSimplicity/AI-CloudOps/pkg/jwt"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *agentservice.Service
}

func NewHandler(service *agentservice.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRouters(server *gin.Engine) {
	group := server.Group("/api/system/agent")
	group.GET("/tools", h.Tools)
	group.POST("/sessions", h.CreateSession)
	group.POST("/query", h.Query)
	group.GET("/sessions/:id/events", h.Events)
	group.POST("/approvals/:id/confirm", h.ConfirmApproval)
	group.POST("/approvals/:id/reject", h.RejectApproval)
}

func (h *Handler) Tools(ctx *gin.Context) {
	base.SuccessWithData(ctx, h.service.Tools())
}

func (h *Handler) CreateSession(ctx *gin.Context) {
	userID, username := currentUser(ctx)
	session, err := h.service.CreateSession(ctx, userID, username)
	if err != nil {
		base.ErrorWithMessage(ctx, err.Error())
		return
	}
	base.SuccessWithData(ctx, session)
}

func (h *Handler) Query(ctx *gin.Context) {
	var req agentmodel.QueryRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		base.BadRequestError(ctx, err.Error())
		return
	}
	userID, username := currentUser(ctx)
	resp, err := h.service.Query(ctx, userID, username, req)
	if err != nil {
		base.ErrorWithMessage(ctx, err.Error())
		return
	}
	base.SuccessWithData(ctx, resp)
}

func (h *Handler) Events(ctx *gin.Context) {
	events, err := h.service.Events(ctx, ctx.Param("id"))
	if err != nil {
		base.ErrorWithMessage(ctx, err.Error())
		return
	}
	base.SuccessWithData(ctx, events)
}

func (h *Handler) ConfirmApproval(ctx *gin.Context) {
	result, err := h.service.ConfirmApproval(ctx, ctx.Param("id"))
	if err != nil {
		base.ErrorWithMessage(ctx, err.Error())
		return
	}
	base.SuccessWithData(ctx, result)
}

func (h *Handler) RejectApproval(ctx *gin.Context) {
	approval, err := h.service.RejectApproval(ctx, ctx.Param("id"))
	if err != nil {
		base.ErrorWithMessage(ctx, err.Error())
		return
	}
	base.SuccessWithData(ctx, approval)
}

func currentUser(ctx *gin.Context) (uint, string) {
	value, ok := ctx.Get("user")
	if !ok {
		return 0, "anonymous"
	}
	user, ok := value.(jwt.UserClaims)
	if !ok {
		return 0, "anonymous"
	}
	if user.Uid < 0 {
		return 0, user.Username
	}
	return uint(user.Uid), firstNonEmpty(user.Username, strconv.Itoa(user.Uid))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
