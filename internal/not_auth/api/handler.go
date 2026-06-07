package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rizxfrog/VanPanelBackend/internal/not_auth/service"
)

type NotAuthHandler struct {
	svc service.NotAuthService
}

func NewNotAuthHandler(svc service.NotAuthService) *NotAuthHandler {
	return &NotAuthHandler{
		svc: svc,
	}
}

func (n *NotAuthHandler) RegisterRouters(server *gin.Engine) {
	notAuthGroup := server.Group("/api/not_auth")
	notAuthGroup.GET("/getBindIps", n.GetBindIps)
	notAuthGroup.GET("/getTreeNodeBindIps", n.GetBindIps)
}

func (n *NotAuthHandler) GetBindIps(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, []interface{}{})
}
