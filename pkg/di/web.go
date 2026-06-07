package di

import (
	"github.com/gin-gonic/gin"
	agentApi "github.com/rizxfrog/VanPanelBackend/internal/agent/api"
	filesApi "github.com/rizxfrog/VanPanelBackend/internal/files/api"
	notAuthHandler "github.com/rizxfrog/VanPanelBackend/internal/not_auth/api"
	systemApi "github.com/rizxfrog/VanPanelBackend/internal/system/api"
	terminalApi "github.com/rizxfrog/VanPanelBackend/internal/terminal/api"
)

// InitGinServer 初始化web服务
func InitGinServer(
	m []gin.HandlerFunc,
	userHdl *systemApi.UserHandler,
	authApiHdl *systemApi.ApiHandler,
	authRoleHdl *systemApi.RoleHandler,
	systemHdl *systemApi.SystemHandler,
	notAuthHdl *notAuthHandler.NotAuthHandler,
	auditHdl *systemApi.AuditHandler,
	terminalHdl *terminalApi.TerminalHandler,
	fileHdl *filesApi.FileHandler,
	fileShareHdl *filesApi.FileShareHandler,
	shareAccessHdl *filesApi.ShareAccessHandler,
	agentHdl *agentApi.Handler,
) *gin.Engine {
	server := gin.Default()
	server.Use(m...)
	userHdl.RegisterRoutes(server)
	authApiHdl.RegisterRouters(server)
	authRoleHdl.RegisterRouters(server)
	systemHdl.RegisterRouters(server)
	auditHdl.RegisterRouters(server)
	notAuthHdl.RegisterRouters(server)
	terminalHdl.RegisterRouters(server)
	fileHdl.RegisterRouters(server)
	fileShareHdl.RegisterRouters(server)
	shareAccessHdl.RegisterRouters(server)
	agentHdl.RegisterRouters(server)
	return server
}
