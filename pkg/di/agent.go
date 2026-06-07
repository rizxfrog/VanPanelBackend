package di

import (
	"context"

	"github.com/rizxfrog/VanPanelBackend/internal/agent/api"
	agentAudit "github.com/rizxfrog/VanPanelBackend/internal/agent/audit"
	agentDao "github.com/rizxfrog/VanPanelBackend/internal/agent/dao"
	agentHub "github.com/rizxfrog/VanPanelBackend/internal/agent/hub"
	agentRisk "github.com/rizxfrog/VanPanelBackend/internal/agent/risk"
	agentService "github.com/rizxfrog/VanPanelBackend/internal/agent/service"
	"github.com/rizxfrog/VanPanelBackend/internal/agent/tool/builtin"
	agentToolManager "github.com/rizxfrog/VanPanelBackend/internal/agent/tool/mcp/manager"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ==================== 配置提供者 ====================

// ProvideAgentConfig 从全局配置中提取 Agent 配置
func ProvideAgentConfig() *AgentConfig {
	cfg := GlobalConfig.Agent
	return &cfg
}

// ProvideAgentRiskConfig 从全局配置中提取 Agent 风险配置
func ProvideAgentRiskConfig() *AgentRiskConfig {
	cfg := GlobalConfig.Agent.Risk
	return &cfg
}

// ProvideAgentHubConfig 从全局配置中提取 Agent Hub 配置
func ProvideAgentHubConfig() *AgentHubConfig {
	cfg := GlobalConfig.Agent.Hub
	return &cfg
}

// ==================== DAO ====================

// ProvideAgentDAO 创建 Agent DAO
func ProvideAgentDAO(db *gorm.DB, l *zap.Logger) agentDao.AgentDAO {
	return agentDao.NewAgentDAO(db, l)
}

// ==================== 工具和风险 ====================

// ProvideAgentToolManager 创建工具管理器
func ProvideAgentToolManager(dao agentDao.AgentDAO, l *zap.Logger) *agentToolManager.ToolManager {
	mgr := agentToolManager.NewToolManager(dao, l)
	// 注册内置工具实现到 ToolManager
	mgr.SetBuiltinTools(builtin.NewBuiltinTools())
	// 异步 seed 数据库中的内置工具记录，确保 cl_agent_builtin_tools 表不为空
	go func() {
		defs := builtin.BuiltinToolDefs()
		if err := dao.SeedBuiltinTools(context.Background(), defs); err != nil {
			l.Warn("seed builtin tools to database failed", zap.Error(err))
		}
	}()
	return mgr
}

// ProvideAgentRiskEvaluator 创建风险评估器
func ProvideAgentRiskEvaluator(cfg *AgentRiskConfig) *agentRisk.Evaluator {
	evalCfg := &agentRisk.EvaluatorConfig{
		HighRiskPatterns: cfg.HighRiskPatterns,
		ProtectedPaths:   cfg.ProtectedPaths,
		Shell: agentRisk.ShellConfig{
			DefaultRisk: cfg.Shell.DefaultRisk,
			Blacklist:   cfg.Shell.Blacklist,
			Whitelist:   cfg.Shell.Whitelist,
		},
	}
	return agentRisk.NewEvaluator(evalCfg)
}

// ==================== 审计 ====================

// ProvideAgentAuditStore 创建审计存储
func ProvideAgentAuditStore(dao agentDao.AgentDAO, l *zap.Logger) agentAudit.Store {
	return agentAudit.NewMemoryStore(dao, l)
}

// ==================== 服务 ====================

// ProvideAgentService 将 di 配置转换为 service 本地配置并创建智能体服务
func ProvideAgentService(
	dao agentDao.AgentDAO,
	toolMgr *agentToolManager.ToolManager,
	riskEval *agentRisk.Evaluator,
	auditStore agentAudit.Store,
	cfg *AgentConfig,
	l *zap.Logger,
) agentService.AgentService {
	svcCfg := &agentService.Config{
		LLM: agentService.LLMConfig{
			Provider:    cfg.LLM.Provider,
			BaseURL:     cfg.LLM.BaseURL,
			APIKey:      cfg.LLM.APIKey,
			Model:       cfg.LLM.Model,
			Temperature: cfg.LLM.Temperature,
			MaxTokens:   cfg.LLM.MaxTokens,
		},
		MaxHistory: cfg.MaxHistory,
	}
	return agentService.NewAgentService(dao, toolMgr, riskEval, auditStore, svcCfg, l, nil)
}

// ProvideHubService 创建 Hub 服务
func ProvideHubService(
	dao agentDao.AgentDAO,
	toolMgr *agentToolManager.ToolManager,
	cfg *AgentHubConfig,
	l *zap.Logger,
) agentHub.HubService {
	hubCfg := &agentHub.AgentHubConfig{
		PluginDir:            cfg.PluginDir,
		MaxPluginSize:        cfg.MaxPluginSize,
		MaxConcurrentPlugins: cfg.MaxConcurrentPlugins,
	}
	return agentHub.NewHubService(dao, toolMgr, hubCfg, l)
}

// ==================== Handler ====================

// ProvideAgentHandler 创建智能体 API 处理器
func ProvideAgentHandler(
	agentSvc agentService.AgentService,
	hubSvc agentHub.HubService,
) *api.Handler {
	return api.NewHandler(agentSvc, hubSvc)
}
