//go:build wireinject

package di

import (
	"github.com/gin-gonic/gin"
	"github.com/google/wire"
	_ "github.com/google/wire"
	filesHandler "github.com/rizxfrog/VanPanelBackend/internal/files/api"
	filesDao "github.com/rizxfrog/VanPanelBackend/internal/files/dao"
	filesService "github.com/rizxfrog/VanPanelBackend/internal/files/service"
	notAuthHandler "github.com/rizxfrog/VanPanelBackend/internal/not_auth/api"
	notAuthService "github.com/rizxfrog/VanPanelBackend/internal/not_auth/service"
	authHandler "github.com/rizxfrog/VanPanelBackend/internal/system/api"
	authDao "github.com/rizxfrog/VanPanelBackend/internal/system/dao"
	authService "github.com/rizxfrog/VanPanelBackend/internal/system/service"
	terminalApi "github.com/rizxfrog/VanPanelBackend/internal/terminal/api"
	ijwt "github.com/rizxfrog/VanPanelBackend/pkg/jwt"
	"github.com/rizxfrog/VanPanelBackend/pkg/sse"
	"github.com/rizxfrog/VanPanelBackend/pkg/terminal"
)

type Cmd struct {
	Server *gin.Engine
}

var HandlerSet = wire.NewSet(
	authHandler.NewRoleHandler,
	authHandler.NewApiHandler,
	authHandler.NewAuditHandler,
	authHandler.NewSystemHandler,
	authHandler.NewUserHandler,
	notAuthHandler.NewNotAuthHandler,
	terminalApi.NewTerminalHandler,
	terminal.NewTerminalHandler,
	filesHandler.NewFileHandler,
	filesHandler.NewFileShareHandler,
	filesHandler.NewShareAccessHandler,
	ProvideAgentHandler,
)

var ServiceSet = wire.NewSet(
	authService.NewUserService,
	authService.NewApiService,
	authService.NewRoleService,
	authService.NewAuditService,
	authService.NewSystemService,
	notAuthService.NewNotAuthService,
	filesService.NewFileService,
	filesService.NewFileShareService,
	ProvideFileManagerConfig,
	ProvideTerminalConfig,
	ProvideTerminalLocalAdapter,
	ProvideTerminalService,
)

var DaoSet = wire.NewSet(
	authDao.NewUserDAO,
	authDao.NewRoleDAO,
	authDao.NewApiDAO,
	authDao.NewAuditDAO,
	filesDao.NewFileShareDAO,
	ProvideAgentDAO,
)

var UtilSet = wire.NewSet(
	ijwt.NewJWTHandler,
	sse.NewHandler,
)

var Injector = wire.NewSet(
	InitMiddlewares,
	InitGinServer,
	InitLogger,
	InitRedis,
	InitDB,
	wire.Struct(new(Cmd), "*"),
)

var AgentSet = wire.NewSet(
	ProvideAgentConfig,
	ProvideAgentRiskConfig,
	ProvideAgentHubConfig,
	ProvideAgentToolManager,
	ProvideAgentRiskEvaluator,
	ProvideAgentAuditStore,
	ProvideAgentService,
	ProvideHubService,
	ProvideAgentGuardChain,
	ProvideAgentMemoryProvider,
	ProvideAgentPipeline,
)

func ProvideCmd() *Cmd {
	wire.Build(
		Injector,
		HandlerSet,
		ServiceSet,
		DaoSet,
		UtilSet,
		AgentSet,
	)
	return &Cmd{}
}
