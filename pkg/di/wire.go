//go:build wireinject

/*
 * MIT License
 *
 * Copyright (c) 2024 Bamboo
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in
 * all copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
 * THE SOFTWARE.
 *
 */

package di

import (
	"github.com/gin-gonic/gin"
	"github.com/google/wire"
	_ "github.com/google/wire"
	"github.com/hibiken/asynq"
	containerApi "github.com/rizxfrog/VanPanelBackend/internal/container/api"
	containerService "github.com/rizxfrog/VanPanelBackend/internal/container/service"
	cron "github.com/rizxfrog/VanPanelBackend/internal/cron"
	cronApi "github.com/rizxfrog/VanPanelBackend/internal/cron/api"
	cronDao "github.com/rizxfrog/VanPanelBackend/internal/cron/dao"
	cronHandler "github.com/rizxfrog/VanPanelBackend/internal/cron/handler"
	cronScheduler "github.com/rizxfrog/VanPanelBackend/internal/cron/scheduler"
	cronService "github.com/rizxfrog/VanPanelBackend/internal/cron/service"
	filesHandler "github.com/rizxfrog/VanPanelBackend/internal/files/api"
	filesDao "github.com/rizxfrog/VanPanelBackend/internal/files/dao"
	filesService "github.com/rizxfrog/VanPanelBackend/internal/files/service"
	k8sHandler "github.com/rizxfrog/VanPanelBackend/internal/k8s/api"
	"github.com/rizxfrog/VanPanelBackend/internal/k8s/client"
	k8sDao "github.com/rizxfrog/VanPanelBackend/internal/k8s/dao"
	"github.com/rizxfrog/VanPanelBackend/internal/k8s/manager"
	k8sService "github.com/rizxfrog/VanPanelBackend/internal/k8s/service"
	notAuthHandler "github.com/rizxfrog/VanPanelBackend/internal/not_auth/api"
	notAuthService "github.com/rizxfrog/VanPanelBackend/internal/not_auth/service"
	promHandler "github.com/rizxfrog/VanPanelBackend/internal/prometheus/api"
	"github.com/rizxfrog/VanPanelBackend/internal/prometheus/cache"
	alertDao "github.com/rizxfrog/VanPanelBackend/internal/prometheus/dao/alert"
	configDao "github.com/rizxfrog/VanPanelBackend/internal/prometheus/dao/config"
	scrapeJobDao "github.com/rizxfrog/VanPanelBackend/internal/prometheus/dao/scrape"
	alertService "github.com/rizxfrog/VanPanelBackend/internal/prometheus/service/alert"
	configService "github.com/rizxfrog/VanPanelBackend/internal/prometheus/service/config"
	scrapeJobService "github.com/rizxfrog/VanPanelBackend/internal/prometheus/service/scrape"
	"github.com/rizxfrog/VanPanelBackend/internal/startup"
	authHandler "github.com/rizxfrog/VanPanelBackend/internal/system/api"
	authDao "github.com/rizxfrog/VanPanelBackend/internal/system/dao"
	authService "github.com/rizxfrog/VanPanelBackend/internal/system/service"
	terminalApi "github.com/rizxfrog/VanPanelBackend/internal/terminal/api"
	treeHandler "github.com/rizxfrog/VanPanelBackend/internal/tree/api"
	treeDao "github.com/rizxfrog/VanPanelBackend/internal/tree/dao"
	treeService "github.com/rizxfrog/VanPanelBackend/internal/tree/service"
	workorderHandler "github.com/rizxfrog/VanPanelBackend/internal/workorder/api"
	workorderDao "github.com/rizxfrog/VanPanelBackend/internal/workorder/dao"
	workorderService "github.com/rizxfrog/VanPanelBackend/internal/workorder/service"
	ijwt "github.com/rizxfrog/VanPanelBackend/pkg/jwt"
	"github.com/rizxfrog/VanPanelBackend/pkg/sse"
	pkgSSH "github.com/rizxfrog/VanPanelBackend/pkg/ssh"
	"github.com/rizxfrog/VanPanelBackend/pkg/terminal"
)

type Cmd struct {
	Server       *gin.Engine
	Bootstrap    startup.ApplicationBootstrap
	CronManager  cron.CronManager
	AsynqServer  *asynq.Server
	AsynqClient  *asynq.Client
	Scheduler    *asynq.Scheduler
	CronHandlers *cronHandler.CronHandlers
}

var HandlerSet = wire.NewSet(
	authHandler.NewRoleHandler,
	authHandler.NewApiHandler,
	authHandler.NewAuditHandler,
	authHandler.NewSystemHandler,
	authHandler.NewUserHandler,
	notAuthHandler.NewNotAuthHandler,
	k8sHandler.NewK8sNodeHandler,
	k8sHandler.NewK8sClusterHandler,
	k8sHandler.NewK8sDeploymentHandler,
	k8sHandler.NewK8sNamespaceHandler,
	k8sHandler.NewK8sSvcHandler,
	k8sHandler.NewK8sYamlTaskHandler,
	k8sHandler.NewK8sYamlTemplateHandler,
	k8sHandler.NewK8sDaemonSetHandler,
	k8sHandler.NewK8sEventHandler,
	k8sHandler.NewK8sStatefulSetHandler,
	k8sHandler.NewK8sServiceAccountHandler,
	k8sHandler.NewK8sRoleHandler,
	k8sHandler.NewK8sClusterRoleHandler,
	k8sHandler.NewK8sRoleBindingHandler,
	k8sHandler.NewK8sClusterRoleBindingHandler,
	k8sHandler.NewK8sRBACHandler,
	k8sHandler.NewK8sIngressHandler,
	k8sHandler.NewK8sPodHandler,
	k8sHandler.NewK8sConfigMapHandler,
	k8sHandler.NewK8sSecretHandler,
	k8sHandler.NewK8sPVHandler,
	k8sHandler.NewK8sPVCHandler,
	promHandler.NewAlertPoolHandler,
	promHandler.NewMonitorConfigHandler,
	promHandler.NewOnDutyGroupHandler,
	promHandler.NewRecordRuleHandler,
	promHandler.NewAlertRuleHandler,
	promHandler.NewSendGroupHandler,
	promHandler.NewScrapeJobHandler,
	promHandler.NewScrapePoolHandler,
	promHandler.NewAlertEventHandler,
	workorderHandler.NewFormDesignHandler,
	workorderHandler.NewInstanceHandler,
	workorderHandler.NewInstanceFlowHandler,
	workorderHandler.NewInstanceCommentHandler,
	workorderHandler.NewInstanceTimeLineHandler,
	workorderHandler.NewTemplateHandler,
	workorderHandler.NewWorkorderProcessHandler,
	workorderHandler.NewCategoryGroupHandler,
	workorderHandler.NewNotificationHandler,
	treeHandler.NewTreeNodeHandler,
	treeHandler.NewTreeLocalHandler,
	treeHandler.NewTreeCloudHandler,
	treeHandler.NewCloudAccountHandler,
	treeHandler.NewCloudAccountRegionHandler,
	terminalApi.NewTerminalHandler,
	terminal.NewTerminalHandler,
	cronApi.NewCronJobHandler,
	filesHandler.NewFileHandler,
	filesHandler.NewFileShareHandler,
	filesHandler.NewShareAccessHandler,
	containerApi.NewContainerHandler,
	ProvideAgentHandler,
)

var ServiceSet = wire.NewSet(
	k8sService.NewClusterService,
	k8sService.NewDeploymentService,
	k8sService.NewNamespaceService,
	k8sService.NewSvcService,
	k8sService.NewNodeService,
	k8sService.NewTaintService,
	k8sService.NewYamlTaskService,
	k8sService.NewYamlTemplateService,
	k8sService.NewDaemonSetService,
	k8sService.NewEventService,
	k8sService.NewStatefulSetService,
	k8sService.NewServiceAccountService,
	k8sService.NewRoleService,
	k8sService.NewClusterRoleService,
	k8sService.NewRoleBindingService,
	k8sService.NewClusterRoleBindingService,
	k8sService.NewRBACService,
	k8sService.NewIngressService,
	k8sService.NewPodService,
	k8sService.NewConfigMapService,
	k8sService.NewSecretService,
	k8sService.NewPVService,
	k8sService.NewPVCService,
	authService.NewUserService,
	authService.NewApiService,
	authService.NewRoleService,
	authService.NewAuditService,
	authService.NewSystemService,
	alertService.NewAlertManagerEventService,
	alertService.NewAlertManagerOnDutyService,
	alertService.NewAlertManagerPoolService,
	alertService.NewAlertManagerRecordService,
	alertService.NewAlertManagerRuleService,
	alertService.NewAlertManagerSendService,
	scrapeJobService.NewPrometheusScrapeService,
	scrapeJobService.NewPrometheusPoolService,
	configService.NewMonitorConfigService,
	notAuthService.NewNotAuthService,
	workorderService.NewFormDesignService,
	workorderService.NewInstanceService,
	workorderService.NewInstanceFlowService,
	workorderService.NewInstanceCommentService,
	workorderService.NewWorkorderInstanceTimeLineService,
	workorderService.NewWorkorderTemplateService,
	workorderService.NewWorkorderProcessService,
	workorderService.NewCategoryGroupService,
	workorderService.NewWorkorderNotificationService,
	treeService.NewTreeNodeService,
	treeService.NewTreeLocalService,
	treeService.NewTreeCloudService,
	treeService.NewCloudAccountService,
	treeService.NewCloudAccountRegionService,
	cronService.NewCronService,
	filesService.NewFileService,
	filesService.NewFileShareService,
	containerService.NewContainerService,
	ProvideFileManagerConfig,
	ProvideTerminalConfig,
	ProvideTerminalLocalAdapter,
	ProvideTerminalSSHAdapter,
	ProvideTerminalService,
)

var DaoSet = wire.NewSet(
	alertDao.NewAlertManagerEventDAO,
	alertDao.NewAlertManagerOnDutyDAO,
	alertDao.NewAlertManagerPoolDAO,
	alertDao.NewAlertManagerRecordDAO,
	alertDao.NewAlertManagerRuleDAO,
	alertDao.NewAlertManagerSendDAO,
	scrapeJobDao.NewScrapeJobDAO,
	scrapeJobDao.NewScrapePoolDAO,
	configDao.NewMonitorConfigDAO,
	authDao.NewUserDAO,
	authDao.NewRoleDAO,
	authDao.NewApiDAO,
	authDao.NewAuditDAO,
	k8sDao.NewClusterDAO,
	k8sDao.NewYamlTaskDAO,
	k8sDao.NewYamlTemplateDAO,
	workorderDao.NewWorkorderFormDesignDAO,
	workorderDao.NewTemplateDAO,
	workorderDao.NewWorkorderInstanceDAO,
	workorderDao.NewProcessDAO,
	workorderDao.NewWorkorderCategoryDAO,
	workorderDao.NewWorkorderInstanceCommentDAO,
	workorderDao.NewInstanceFlowDAO,
	workorderDao.NewInstanceTimeLineDAO,
	workorderDao.NewNotificationDAO,
	treeDao.NewTreeNodeDAO,
	treeDao.NewTreeLocalDAO,
	treeDao.NewTreeCloudDAO,
	treeDao.NewCloudAccountDAO,
	treeDao.NewCloudAccountRegionDAO,
	filesDao.NewFileShareDAO,
	cronDao.NewCronJobDAO,
	ProvideAgentDAO,
)

var SSHSet = wire.NewSet(
	pkgSSH.NewClient,
)

var UtilSet = wire.NewSet(
	ijwt.NewJWTHandler,
	sse.NewHandler,
)

var ManagerSet = wire.NewSet(
	manager.NewClusterManager,
	manager.NewDeploymentManager,
	manager.NewNamespaceManager,
	manager.NewServiceManager,
	manager.NewNodeManager,
	manager.NewEventManager,
	manager.NewStatefulSetManager,
	manager.NewDaemonSetManager,
	manager.NewServiceAccountManager,
	manager.NewTaintManager,
	manager.NewYamlManager,
	manager.NewConfigMapManager,
	manager.NewSecretManager,
	manager.NewPVManager,
	manager.NewPVCManager,
	manager.NewClusterRoleManager,
	manager.NewClusterRoleBindingManager,
	manager.NewRoleManager,
	manager.NewRoleBindingManager,
	manager.NewIngressManager,
	manager.NewPodManager,
)

var JobSet = wire.NewSet(
	startup.NewApplicationBootstrap,
)

var CronSet = wire.NewSet(
	cron.NewUnifiedCronManager,
	cron.NewBuiltinTaskManager,
	cronHandler.NewCronHandlers,
	cronScheduler.NewCronScheduler,
)

var Injector = wire.NewSet(
	InitMiddlewares,
	InitGinServer,
	InitLogger,
	InitRedis,
	InitDB,
	CronSet,
	wire.Struct(new(Cmd), "*"),
)

var CacheSet = wire.NewSet(
	cache.NewMonitorCache,
	cache.NewAlertManagerConfigCache,
	cache.NewAlertRuleConfigCache,
	cache.NewRecordRuleConfigCache,
	cache.NewPrometheusConfigCache,
	cache.NewBatchConfigManager,
)

var ClientSet = wire.NewSet(
	client.NewK8sClient,
)

var AsynqSet = wire.NewSet(
	InitAsynqClient,
	InitAsynqServer,
	InitScheduler,
)

var NotificationSet = wire.NewSet(
	InitNotificationConfig,
	InitNotificationManager,
)

var AgentSet = wire.NewSet(
	ProvideAgentConfig,
	ProvideAgentRiskConfig,
	ProvideAgentHubConfig,
	ProvideAgentToolManager,
	ProvideAgentRiskEvaluator,
	ProvideAgentService,
	ProvideHubService,
)

func ProvideCmd() *Cmd {
	wire.Build(
		Injector,
		HandlerSet,
		ServiceSet,
		DaoSet,
		SSHSet,
		UtilSet,
		JobSet,
		ManagerSet,
		CacheSet,
		ClientSet,
		AsynqSet,
		NotificationSet,
		AgentSet,
	)
	return &Cmd{}
}
