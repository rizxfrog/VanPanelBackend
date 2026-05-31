package api

import (
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	containermodel "github.com/rizxfrog/VanPanelBackend/internal/container/model"
	containerservice "github.com/rizxfrog/VanPanelBackend/internal/container/service"
	"github.com/rizxfrog/VanPanelBackend/pkg/base"
)

type ContainerHandler struct {
	service containerservice.ContainerService
}

func NewContainerHandler(service containerservice.ContainerService) *ContainerHandler {
	return &ContainerHandler{service: service}
}

func (h *ContainerHandler) RegisterRouters(server *gin.Engine) {
	group := server.Group("/api/system/containers")
	group.GET("", h.List)
	group.POST("/:id/start", h.Start)
	group.POST("/:id/stop", h.Stop)
	group.POST("/:id/restart", h.Restart)
	group.DELETE("/:id", h.Delete)
	group.GET("/:id/stats", h.Stats)
	group.GET("/:id/logs", h.Logs)
}

func (h *ContainerHandler) List(ctx *gin.Context) {
	result, err := h.service.List(ctx, containermodel.ListQuery{
		Name:     ctx.Query("name"),
		State:    ctx.DefaultQuery("state", "all"),
		Page:     parseInt(ctx.Query("page")),
		PageSize: parseInt(ctx.Query("pageSize")),
	})
	if err != nil {
		base.ErrorWithMessage(ctx, err.Error())
		return
	}
	base.SuccessWithData(ctx, result)
}

func (h *ContainerHandler) Start(ctx *gin.Context) {
	h.operate(ctx, "start")
}

func (h *ContainerHandler) Stop(ctx *gin.Context) {
	h.operate(ctx, "stop")
}

func (h *ContainerHandler) Restart(ctx *gin.Context) {
	h.operate(ctx, "restart")
}

func (h *ContainerHandler) Delete(ctx *gin.Context) {
	h.operate(ctx, "delete")
}

func (h *ContainerHandler) Stats(ctx *gin.Context) {
	stats, err := h.service.Stats(ctx, ctx.Param("id"))
	if err != nil {
		base.ErrorWithMessage(ctx, err.Error())
		return
	}
	base.SuccessWithData(ctx, stats)
}

func (h *ContainerHandler) Logs(ctx *gin.Context) {
	follow := parseBool(ctx.Query("follow"))
	reader, err := h.service.Logs(ctx, ctx.Param("id"), containermodel.LogOptions{
		Tail:       ctx.DefaultQuery("tail", "200"),
		Since:      ctx.Query("since"),
		Timestamps: parseBool(ctx.Query("timestamps")),
		Follow:     follow,
	})
	if err != nil {
		base.ErrorWithMessage(ctx, err.Error())
		return
	}
	defer reader.Close()

	if follow {
		ctx.Header("Content-Type", "text/event-stream")
		ctx.Header("Cache-Control", "no-cache")
		ctx.Header("Connection", "keep-alive")
		buf := make([]byte, 4096)
		ctx.Stream(func(w io.Writer) bool {
			n, readErr := reader.Read(buf)
			if n > 0 {
				_, _ = w.Write([]byte("data: "))
				_, _ = w.Write(buf[:n])
				_, _ = w.Write([]byte("\n\n"))
				return true
			}
			return readErr == nil
		})
		return
	}

	body, err := io.ReadAll(reader)
	if err != nil {
		base.ErrorWithMessage(ctx, err.Error())
		return
	}
	ctx.Data(http.StatusOK, "text/plain; charset=utf-8", body)
}

func (h *ContainerHandler) operate(ctx *gin.Context, operation string) {
	if err := h.service.Operate(ctx, ctx.Param("id"), operation); err != nil {
		base.ErrorWithMessage(ctx, err.Error())
		return
	}
	base.SuccessWithMessage(ctx, "success")
}

func parseInt(value string) int {
	if value == "" {
		return 0
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return parsed
}

func parseBool(value string) bool {
	parsed, err := strconv.ParseBool(value)
	return err == nil && parsed
}
