package api

import (
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/rizxfrog/VanPanelBackend/internal/agent/hub"
	"github.com/rizxfrog/VanPanelBackend/internal/agent/insight"
	"github.com/rizxfrog/VanPanelBackend/internal/agent/search"
	"github.com/rizxfrog/VanPanelBackend/internal/agent/service"
	"github.com/rizxfrog/VanPanelBackend/internal/agent/skill"
	"github.com/rizxfrog/VanPanelBackend/internal/model"
	"github.com/rizxfrog/VanPanelBackend/pkg/base"
	"github.com/rizxfrog/VanPanelBackend/pkg/jwt"
)

// Handler 智能体 API 处理器
type Handler struct {
	agentService  service.AgentService
	hubService    hub.HubService
	configService *service.ConfigService
	searchEngine  *search.SearchEngine
	skillStore    *skill.SkillStore
	insights      *insight.InsightsEngine
}

// NewHandler 创建智能体 API 处理器实例
func NewHandler(
	agentService service.AgentService,
	hubService hub.HubService,
	configService *service.ConfigService,
	searchEngine *search.SearchEngine,
	skillStore *skill.SkillStore,
	insights *insight.InsightsEngine,
) *Handler {
	return &Handler{
		agentService:  agentService,
		hubService:    hubService,
		configService: configService,
		searchEngine:  searchEngine,
		skillStore:    skillStore,
		insights:      insights,
	}
}

// RegisterRouters 注册路由
func (h *Handler) RegisterRouters(server *gin.Engine) {
	agentGroup := server.Group("/api/system/agent")
	{
		// 查询
		agentGroup.POST("/query", h.Query)
		agentGroup.POST("/query/stream", h.QueryStream)
		agentGroup.POST("/pipeline/query", h.QueryWithPipeline)
		agentGroup.POST("/pipeline/query/stream", h.QueryStreamWithPipeline)
		agentGroup.GET("/tools", h.ListTools)

		// 会话
		agentGroup.POST("/sessions/create", h.CreateSession)
		agentGroup.GET("/sessions/list", h.ListSessions)
		agentGroup.GET("/sessions/:id/detail", h.GetSession)
		agentGroup.GET("/sessions/:id/messages", h.ListMessages)
		agentGroup.DELETE("/sessions/:id/delete", h.DeleteSession)

		// 内置工具管理
		agentGroup.GET("/builtin-tools/list", h.ListBuiltinTools)
		agentGroup.PUT("/builtin-tools/:name/toggle", h.ToggleBuiltinTool)

		// Hub 插件
		agentGroup.GET("/hub/plugins/list", h.ListPlugins)
		agentGroup.GET("/hub/plugins/:id/detail", h.GetPlugin)
		agentGroup.POST("/hub/plugins/upload", h.UploadPlugin)
		agentGroup.POST("/hub/plugins/:id/install", h.InstallPlugin)
		agentGroup.DELETE("/hub/plugins/:id/uninstall", h.UninstallPlugin)
		agentGroup.PUT("/hub/plugins/:id/toggle", h.TogglePlugin)

		// 远程 MCP
		agentGroup.GET("/remote-mcps/list", h.ListRemoteMCPConfigs)
		agentGroup.POST("/remote-mcps/create", h.CreateRemoteMCPConfig)
		agentGroup.PUT("/remote-mcps/:id/update", h.UpdateRemoteMCPConfig)
		agentGroup.DELETE("/remote-mcps/:id/delete", h.DeleteRemoteMCPConfig)
		agentGroup.PUT("/remote-mcps/:id/toggle", h.ToggleRemoteMCPConfig)
		agentGroup.POST("/remote-mcps/:id/test", h.TestRemoteMCPConfig)

		// Agent 配置 CRUD
		agentGroup.GET("/config/list", h.ListAgentConfigs)
		agentGroup.GET("/config/:key", h.GetAgentConfig)
		agentGroup.PUT("/config/:key", h.UpdateAgentConfig)

		// 搜索、分析、技能
		agentGroup.GET("/search", h.SearchHistory)
		agentGroup.GET("/insights", h.GetInsights)
		agentGroup.GET("/skills", h.ListSkills)
		agentGroup.GET("/skills/:name", h.GetSkill)
		agentGroup.POST("/skills/:name/pin", h.PinSkill)
		agentGroup.POST("/skills/:name/unpin", h.UnpinSkill)
	}
}

// ==================== 查询 ====================

// Query 同步查询智能体
func (h *Handler) Query(ctx *gin.Context) {
	var req model.AgentQueryReq
	uc := ctx.MustGet("user").(jwt.UserClaims)
	base.HandleRequest(ctx, &req, func() (interface{}, error) {
		return h.agentService.Query(ctx, &req, uc.Uid)
	})
}

// QueryStream 流式查询智能体（SSE）
func (h *Handler) QueryStream(ctx *gin.Context) {
	var req model.AgentQueryReq
	uc := ctx.MustGet("user").(jwt.UserClaims)
	if err := ctx.ShouldBind(&req); err != nil {
		base.BadRequestError(ctx, err.Error())
		return
	}
	ctx.Header("Content-Type", "text/event-stream")
	ctx.Header("Cache-Control", "no-cache")
	ctx.Header("Connection", "keep-alive")
	if err := h.agentService.QueryStream(ctx, &req, uc.Uid, ctx.Writer); err != nil {
		base.ErrorWithMessage(ctx, err.Error())
	}
}

// ListTools 获取所有可用工具列表
func (h *Handler) ListTools(ctx *gin.Context) {
	base.HandleRequest(ctx, nil, func() (interface{}, error) {
		return h.agentService.ListTools(ctx)
	})
}

// QueryWithPipeline 使用 Pipeline 增强的同步查询
func (h *Handler) QueryWithPipeline(c *gin.Context) {
	var req model.AgentQueryReq
	uc := c.MustGet("user").(jwt.UserClaims)
	base.HandleRequest(c, &req, func() (interface{}, error) {
		return h.agentService.QueryWithPipeline(c, &req, uc.Uid)
	})
}

// QueryStreamWithPipeline 使用 Pipeline 增强的流式查询（SSE）
func (h *Handler) QueryStreamWithPipeline(c *gin.Context) {
	var req model.AgentQueryReq
	uc := c.MustGet("user").(jwt.UserClaims)
	if err := c.ShouldBind(&req); err != nil {
		base.BadRequestError(c, err.Error())
		return
	}
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	if err := h.agentService.QueryStreamWithPipeline(c, &req, uc.Uid, c.Writer); err != nil {
		base.ErrorWithMessage(c, err.Error())
	}
}

// ==================== 会话 ====================

// CreateSession 创建新会话
func (h *Handler) CreateSession(ctx *gin.Context) {
	var req model.CreateAgentSessionReq
	uc := ctx.MustGet("user").(jwt.UserClaims)
	base.HandleRequest(ctx, &req, func() (interface{}, error) {
		return h.agentService.CreateSession(ctx, &req, uc.Uid)
	})
}

// ListSessions 获取会话列表
func (h *Handler) ListSessions(ctx *gin.Context) {
	var req model.ListAgentSessionsReq
	uc := ctx.MustGet("user").(jwt.UserClaims)
	base.HandleRequest(ctx, &req, func() (interface{}, error) {
		return h.agentService.ListSessions(ctx, &req, uc.Uid)
	})
}

// GetSession 获取会话详情
func (h *Handler) GetSession(ctx *gin.Context) {
	id, err := base.GetCustomParamID(ctx, "id")
	if err != nil {
		base.BadRequestError(ctx, err.Error())
		return
	}
	base.HandleRequest(ctx, nil, func() (interface{}, error) {
		return h.agentService.GetSession(ctx, id)
	})
}

// ListMessages 获取会话消息列表
func (h *Handler) ListMessages(ctx *gin.Context) {
	var req model.ListAgentMessagesReq
	id, err := base.GetCustomParamID(ctx, "id")
	if err != nil {
		base.BadRequestError(ctx, err.Error())
		return
	}
	req.SessionID = fmt.Sprintf("%d", id)
	base.HandleRequest(ctx, &req, func() (interface{}, error) {
		return h.agentService.ListMessages(ctx, &req)
	})
}

// DeleteSession 删除会话
func (h *Handler) DeleteSession(ctx *gin.Context) {
	id, err := base.GetCustomParamID(ctx, "id")
	if err != nil {
		base.BadRequestError(ctx, err.Error())
		return
	}
	base.HandleRequest(ctx, nil, func() (interface{}, error) {
		return nil, h.agentService.DeleteSession(ctx, id)
	})
}

// ==================== 内置工具 ====================

// ListBuiltinTools 获取内置工具列表
func (h *Handler) ListBuiltinTools(ctx *gin.Context) {
	base.HandleRequest(ctx, nil, func() (interface{}, error) {
		return h.hubService.ListBuiltinTools(ctx)
	})
}

// ToggleBuiltinTool 切换内置工具启用状态
func (h *Handler) ToggleBuiltinTool(ctx *gin.Context) {
	name := ctx.Param("name")
	base.HandleRequest(ctx, nil, func() (interface{}, error) {
		return nil, h.hubService.ToggleBuiltinTool(ctx, name)
	})
}

// ==================== Hub 插件 ====================

// ListPlugins 获取插件列表
func (h *Handler) ListPlugins(ctx *gin.Context) {
	var req model.ListMCPPluginsReq
	base.HandleRequest(ctx, &req, func() (interface{}, error) {
		return h.hubService.ListPlugins(ctx, &req)
	})
}

// GetPlugin 获取插件详情
func (h *Handler) GetPlugin(ctx *gin.Context) {
	id, err := base.GetCustomParamID(ctx, "id")
	if err != nil {
		base.BadRequestError(ctx, err.Error())
		return
	}
	base.HandleRequest(ctx, nil, func() (interface{}, error) {
		return h.hubService.GetPlugin(ctx, id)
	})
}

// UploadPlugin 上传插件
func (h *Handler) UploadPlugin(ctx *gin.Context) {
	uc := ctx.MustGet("user").(jwt.UserClaims)
	file, _, err := ctx.Request.FormFile("binary")
	if err != nil {
		base.BadRequestError(ctx, "缺少二进制文件")
		return
	}
	defer file.Close()
	manifest := ctx.PostForm("manifest")
	base.HandleRequest(ctx, nil, func() (interface{}, error) {
		return h.hubService.UploadPlugin(ctx, manifest, file, uc.Uid)
	})
}

// InstallPlugin 安装插件
func (h *Handler) InstallPlugin(ctx *gin.Context) {
	uc := ctx.MustGet("user").(jwt.UserClaims)
	id, err := base.GetCustomParamID(ctx, "id")
	if err != nil {
		base.BadRequestError(ctx, err.Error())
		return
	}
	var req model.InstallMCPPluginReq
	req.ID = id
	base.HandleRequest(ctx, &req, func() (interface{}, error) {
		return nil, h.hubService.InstallPlugin(ctx, &req, uc.Uid)
	})
}

// UninstallPlugin 卸载插件
func (h *Handler) UninstallPlugin(ctx *gin.Context) {
	uc := ctx.MustGet("user").(jwt.UserClaims)
	id, err := base.GetCustomParamID(ctx, "id")
	if err != nil {
		base.BadRequestError(ctx, err.Error())
		return
	}
	var req model.UninstallMCPPluginReq
	req.ID = id
	base.HandleRequest(ctx, &req, func() (interface{}, error) {
		return nil, h.hubService.UninstallPlugin(ctx, &req, uc.Uid)
	})
}

// TogglePlugin 切换插件启用状态
func (h *Handler) TogglePlugin(ctx *gin.Context) {
	uc := ctx.MustGet("user").(jwt.UserClaims)
	id, err := base.GetCustomParamID(ctx, "id")
	if err != nil {
		base.BadRequestError(ctx, err.Error())
		return
	}
	var req model.ToggleMCPPluginReq
	req.ID = id
	base.HandleRequest(ctx, &req, func() (interface{}, error) {
		return nil, h.hubService.TogglePlugin(ctx, &req, uc.Uid)
	})
}

// ==================== 远程 MCP ====================

// ListRemoteMCPConfigs 获取远程 MCP 配置列表
func (h *Handler) ListRemoteMCPConfigs(ctx *gin.Context) {
	var req model.ListRemoteMCPConfigsReq
	base.HandleRequest(ctx, &req, func() (interface{}, error) {
		return h.hubService.ListRemoteConfigs(ctx, &req)
	})
}

// CreateRemoteMCPConfig 创建远程 MCP 配置
func (h *Handler) CreateRemoteMCPConfig(ctx *gin.Context) {
	var req model.CreateRemoteMCPConfigReq
	uc := ctx.MustGet("user").(jwt.UserClaims)
	base.HandleRequest(ctx, &req, func() (interface{}, error) {
		return nil, h.hubService.CreateRemoteConfig(ctx, &req, uc.Uid)
	})
}

// UpdateRemoteMCPConfig 更新远程 MCP 配置
func (h *Handler) UpdateRemoteMCPConfig(ctx *gin.Context) {
	var req model.UpdateRemoteMCPConfigReq
	id, err := base.GetCustomParamID(ctx, "id")
	if err != nil {
		base.BadRequestError(ctx, err.Error())
		return
	}
	req.ID = id
	base.HandleRequest(ctx, &req, func() (interface{}, error) {
		return nil, h.hubService.UpdateRemoteConfig(ctx, &req)
	})
}

// DeleteRemoteMCPConfig 删除远程 MCP 配置
func (h *Handler) DeleteRemoteMCPConfig(ctx *gin.Context) {
	id, err := base.GetCustomParamID(ctx, "id")
	if err != nil {
		base.BadRequestError(ctx, err.Error())
		return
	}
	base.HandleRequest(ctx, nil, func() (interface{}, error) {
		return nil, h.hubService.DeleteRemoteConfig(ctx, id)
	})
}

// ToggleRemoteMCPConfig 切换远程 MCP 配置启用状态
func (h *Handler) ToggleRemoteMCPConfig(ctx *gin.Context) {
	id, err := base.GetCustomParamID(ctx, "id")
	if err != nil {
		base.BadRequestError(ctx, err.Error())
		return
	}
	base.HandleRequest(ctx, nil, func() (interface{}, error) {
		return nil, h.hubService.ToggleRemoteConfig(ctx, id)
	})
}

// TestRemoteMCPConfig 测试远程 MCP 配置连通性
func (h *Handler) TestRemoteMCPConfig(ctx *gin.Context) {
	id, err := base.GetCustomParamID(ctx, "id")
	if err != nil {
		base.BadRequestError(ctx, err.Error())
		return
	}
	base.HandleRequest(ctx, nil, func() (interface{}, error) {
		return h.hubService.TestRemoteConfig(ctx, id)
	})
}

// ==================== Agent 配置 ====================

// GetAgentConfig 获取指定 key 的配置值
func (h *Handler) GetAgentConfig(ctx *gin.Context) {
	key := ctx.Param("key")
	value, err := h.configService.GetConfig(ctx, key)
	if err != nil {
		base.ErrorWithMessage(ctx, "config not found: "+key)
		return
	}
	base.SuccessWithData(ctx, gin.H{"key": key, "value": value})
}

// UpdateAgentConfig 更新配置项
func (h *Handler) UpdateAgentConfig(ctx *gin.Context) {
	key := ctx.Param("key")
	var req struct {
		Value string `json:"value"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		base.BadRequestError(ctx, "invalid request body")
		return
	}
	if err := h.configService.UpsertConfig(ctx, key, req.Value); err != nil {
		base.ErrorWithMessage(ctx, "update config failed: "+err.Error())
		return
	}
	base.Success(ctx)
}

// ListAgentConfigs 获取所有配置项
func (h *Handler) ListAgentConfigs(ctx *gin.Context) {
	cfgs, err := h.configService.ListConfigs(ctx)
	if err != nil {
		base.ErrorWithMessage(ctx, "list configs failed: "+err.Error())
		return
	}
	base.SuccessWithData(ctx, cfgs)
}

// ==================== 搜索、分析、技能 ====================

// SearchHistory 全文搜索会话历史
func (h *Handler) SearchHistory(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		base.BadRequestError(c, "搜索关键词不能为空")
		return
	}
	limit := 20
	if l, err := strconv.Atoi(c.DefaultQuery("limit", "20")); err == nil && l > 0 {
		limit = l
	}

	results, err := h.searchEngine.Search(c.Request.Context(), query, limit)
	if err != nil {
		base.ErrorWithMessage(c, "搜索失败: "+err.Error())
		return
	}
	base.SuccessWithData(c, results)
}

// GetInsights 生成 Token/费用/工具使用分析报告
func (h *Handler) GetInsights(c *gin.Context) {
	days := 30
	if d, err := strconv.Atoi(c.DefaultQuery("days", "30")); err == nil && d > 0 {
		days = d
	}

	report, err := h.insights.Generate(c.Request.Context(), days)
	if err != nil {
		base.ErrorWithMessage(c, "生成分析报告失败: "+err.Error())
		return
	}
	base.SuccessWithData(c, report)
}

// ListSkills 列出所有 skill
func (h *Handler) ListSkills(c *gin.Context) {
	skills, err := h.skillStore.ListSkills(c.Request.Context())
	if err != nil {
		base.ErrorWithMessage(c, "获取 skill 列表失败: "+err.Error())
		return
	}
	base.SuccessWithData(c, skills)
}

// GetSkill 获取指定 skill 详情
func (h *Handler) GetSkill(c *gin.Context) {
	name := c.Param("name")
	skill, err := h.skillStore.GetSkill(c.Request.Context(), name)
	if err != nil {
		base.ErrorWithMessage(c, "获取 skill 失败: "+err.Error())
		return
	}
	base.SuccessWithData(c, skill)
}

// PinSkill 置顶 skill
func (h *Handler) PinSkill(c *gin.Context) {
	name := c.Param("name")
	if err := h.skillStore.PinSkill(c.Request.Context(), name); err != nil {
		base.ErrorWithMessage(c, "固定 skill 失败: "+err.Error())
		return
	}
	base.Success(c)
}

// UnpinSkill 取消置顶 skill
func (h *Handler) UnpinSkill(c *gin.Context) {
	name := c.Param("name")
	if err := h.skillStore.UnpinSkill(c.Request.Context(), name); err != nil {
		base.ErrorWithMessage(c, "取消固定 skill 失败: "+err.Error())
		return
	}
	base.Success(c)
}
