package di

import (
	"context"
	"os"

	"github.com/rizxfrog/VanPanelBackend/internal/agent/api"
	agentAudit "github.com/rizxfrog/VanPanelBackend/internal/agent/audit"
	agentDao "github.com/rizxfrog/VanPanelBackend/internal/agent/dao"
	agentInsight "github.com/rizxfrog/VanPanelBackend/internal/agent/insight"
	agentNudge "github.com/rizxfrog/VanPanelBackend/internal/agent/nudge"
	agentSearch "github.com/rizxfrog/VanPanelBackend/internal/agent/search"
	agentGuard "github.com/rizxfrog/VanPanelBackend/internal/agent/guard"
	agentHub "github.com/rizxfrog/VanPanelBackend/internal/agent/hub"
	agentMemory "github.com/rizxfrog/VanPanelBackend/internal/agent/memory"
	agentPipeline "github.com/rizxfrog/VanPanelBackend/internal/agent/pipeline"
	agentRisk "github.com/rizxfrog/VanPanelBackend/internal/agent/risk"
	agentService "github.com/rizxfrog/VanPanelBackend/internal/agent/service"
	agentSkill "github.com/rizxfrog/VanPanelBackend/internal/agent/skill"
	"github.com/rizxfrog/VanPanelBackend/internal/agent/spi"
	"github.com/rizxfrog/VanPanelBackend/internal/agent/tool/builtin"
	agentToolManager "github.com/rizxfrog/VanPanelBackend/internal/agent/tool/mcp/manager"
	"github.com/cloudwego/eino/components/tool"
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
func ProvideAgentDAO(db *gorm.DB, l *zap.Logger, searchEngine *agentSearch.SearchEngine) agentDao.AgentDAO {
	return agentDao.NewAgentDAO(db, l, searchEngine)
}

// ProvideSearchEngine 创建全文搜索引擎
func ProvideSearchEngine(db *gorm.DB, l *zap.Logger) *agentSearch.SearchEngine {
	return agentSearch.NewSearchEngine(db, l)
}

// ProvideAgentConfigDAO creates the config DAO
func ProvideAgentConfigDAO(db *gorm.DB) *agentDao.AgentConfigDAO {
	return agentDao.NewAgentConfigDAO(db)
}

// ==================== 配置服务 ====================

// ProvideAgentConfigService creates the config service
func ProvideAgentConfigService(dao *agentDao.AgentConfigDAO) *agentService.ConfigService {
	return agentService.NewConfigService(dao)
}

// ProvideAgentLLMAuditor creates the LLM injection auditor (nil if not configured)
func ProvideAgentLLMAuditor(l *zap.Logger) *agentPipeline.LLMAuditor {
	baseURL := os.Getenv("AGENT_AUDITOR_BASE_URL")
	apiKey := os.Getenv("AGENT_AUDITOR_API_KEY")
	if baseURL != "" && apiKey != "" {
		l.Info("LLM injection auditor enabled",
			zap.String("base_url", baseURL))
		return agentPipeline.NewLLMAuditor(agentPipeline.LLMAuditorConfig{
			BaseURL: baseURL,
			APIKey:  apiKey,
		})
	}
	l.Info("LLM injection auditor not configured (set AGENT_AUDITOR_BASE_URL and AGENT_AUDITOR_API_KEY)")
	return nil
}

// ==================== 工具和风险 ====================

// ProvideAgentToolManager 创建工具管理器
func ProvideAgentToolManager(dao agentDao.AgentDAO, l *zap.Logger, skillStore *agentSkill.SkillStore) *agentToolManager.ToolManager {
	mgr := agentToolManager.NewToolManager(dao, l)
	// 注册内置工具实现到 ToolManager
	mgr.SetBuiltinTools(builtin.NewBuiltinTools(skillStore))
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
	pipelineStage *agentPipeline.Stage,
	nudgeReviewer *agentNudge.MemoryNudgeReviewer,
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
	return agentService.NewAgentService(dao, toolMgr, riskEval, auditStore, svcCfg, l, pipelineStage, nudgeReviewer)
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
	cfgSvc *agentService.ConfigService,
	searchEngine *agentSearch.SearchEngine,
	skillStore *agentSkill.SkillStore,
	insights *agentInsight.InsightsEngine,
) *api.Handler {
	return api.NewHandler(agentSvc, hubSvc, cfgSvc, searchEngine, skillStore, insights)
}

// ==================== Guard / Memory / Pipeline ====================

// ProvideAgentGuardChain 创建 GuardChain（双层防线）
func ProvideAgentGuardChain(eval *agentRisk.Evaluator, l *zap.Logger) *agentGuard.Chain {
	// 审计模型配置从环境变量读取
	auditorModel := os.Getenv("AGENT_AUDITOR_MODEL")
	auditorBaseURL := os.Getenv("AGENT_AUDITOR_BASE_URL")

	var auditor *agentGuard.Auditor
	if auditorModel != "" && auditorBaseURL != "" {
		auditorConfig := agentGuard.AuditorConfig{
			BaseURL: auditorBaseURL,
			Model:   auditorModel,
			APIKey:  os.Getenv("AGENT_AUDITOR_API_KEY"),
		}
		auditor = agentGuard.NewAuditor(auditorConfig)
		l.Info("auditor model configured",
			zap.String("model", auditorModel),
			zap.String("base_url", auditorBaseURL),
		)
	} else {
		l.Info("auditor model not configured, GuardChain will use rule engine only")
	}
	return agentGuard.NewChain(eval, auditor)
}

// ProvideAgentMemoryProvider 创建 MemoryProvider
func ProvideAgentMemoryProvider(dao agentDao.AgentDAO, l *zap.Logger) spi.MemoryProvider {
	return agentMemory.NewProvider(dao, l)
}

// ProvideAgentPipeline 创建 Pipeline Stage
func ProvideAgentPipeline(
	dao agentDao.AgentDAO,
	cfgSvc *agentService.ConfigService,
	auditor *agentPipeline.LLMAuditor,
	l *zap.Logger,
) *agentPipeline.Stage {
	// Wrap ConfigService methods to avoid circular imports
	configGetter := func(ctx context.Context) ([]agentPipeline.InjectRule, error) {
		rules, err := cfgSvc.GetInjectionRules(ctx)
		if err != nil {
			return nil, err
		}
		out := make([]agentPipeline.InjectRule, len(rules))
		for i, r := range rules {
			out[i] = agentPipeline.InjectRule{Desc: r.Desc, Re: r.Re}
		}
		return out, nil
	}
	promptGetter := func(ctx context.Context) (*agentPipeline.LLMAuditPrompt, error) {
		cfg, err := cfgSvc.GetLLMAuditPrompt(ctx)
		if err != nil {
			return nil, err
		}
		return &agentPipeline.LLMAuditPrompt{
			Enabled:      cfg.Enabled,
			Model:        cfg.Model,
			Temperature:  cfg.Temperature,
			MaxTokens:    cfg.MaxTokens,
			SystemPrompt: cfg.SystemPrompt,
			MaxRetries:   cfg.MaxRetries,
		}, nil
	}
	intentAnalyzer := agentPipeline.NewHybridIntentAnalyzer(configGetter, auditor, promptGetter, l)
	// Use the existing memory provider
	memoryProvider := agentMemory.NewProvider(dao, l)
	return agentPipeline.NewStage(intentAnalyzer, memoryProvider, l)
}

// ==================== Skill ====================

// ProvideAgentSkillStore 创建 skill 文件系统存储
func ProvideAgentSkillStore(cfg *AgentConfig, l *zap.Logger) (*agentSkill.SkillStore, error) {
	baseDir := cfg.Skill.BaseDir
	if baseDir == "" {
		baseDir = "data/skills"
	}
	return agentSkill.NewSkillStore(baseDir, l)
}

// ==================== Skill Manager Tool ====================

// ProvideAgentSkillManagerTool 创建 skill_manage Eino 工具
func ProvideAgentSkillManagerTool(store *agentSkill.SkillStore) tool.InvokableTool {
	return agentSkill.NewSkillManagerTool(store)
}

// ==================== Nudge ====================

// ProvideAgentNudgeReviewer 创建记忆轻推审查器
func ProvideAgentNudgeReviewer(cfg *AgentConfig, l *zap.Logger) *agentNudge.MemoryNudgeReviewer {
	nudgeCfg := agentNudge.NudgeConfig{
		MemoryInterval: cfg.Nudge.MemoryInterval,
		SkillInterval:  cfg.Nudge.SkillInterval,
	}
	return agentNudge.NewMemoryNudgeReviewer(nudgeCfg, "data/memory", l)
}

// ==================== Insights ====================

// ProvideAgentInsightsEngine 创建分析引擎
func ProvideAgentInsightsEngine(db *gorm.DB, l *zap.Logger) *agentInsight.InsightsEngine {
	return agentInsight.NewInsightsEngine(db, l)
}
