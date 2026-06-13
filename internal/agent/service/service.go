package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	einoopenai "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"

	agentaudit "github.com/rizxfrog/VanPanelBackend/internal/agent/audit"
	"github.com/rizxfrog/VanPanelBackend/internal/agent/dao"
	agentmodel "github.com/rizxfrog/VanPanelBackend/internal/agent/model"
	"github.com/rizxfrog/VanPanelBackend/internal/agent/nudge"
	"github.com/rizxfrog/VanPanelBackend/internal/agent/pipeline"
	"github.com/rizxfrog/VanPanelBackend/internal/agent/risk"
	"github.com/rizxfrog/VanPanelBackend/internal/agent/tool/mcp/manager"
	"github.com/rizxfrog/VanPanelBackend/internal/model"
)

const personaPrompt = "你是一个运维助手，你有多种工具可以调用来查询系统信息。当用户询问系统状态、可用工具、或需要执行运维操作时，必须优先使用工具来获取实时数据，不要凭记忆回答。如果用户问\"有哪些工具\"或\"可以用什么工具\"，直接列出你实际可用的工具名称和用途。"

// LLMConfig LLM 配置，与 di.AgentLLMConfig 字段对齐，避免 import cycle。
type LLMConfig struct {
	Provider    string
	BaseURL     string
	APIKey      string
	Model       string
	Temperature float64
	MaxTokens   int
}

// Config 智能体服务配置，与 di.AgentConfig 字段对齐，避免 import cycle。
type Config struct {
	LLM        LLMConfig
	MaxHistory int
}

// AgentService 智能体服务接口
type AgentService interface {
	Query(ctx context.Context, req *model.AgentQueryReq, userID int) (*model.AgentQueryResponse, error)
	QueryStream(ctx context.Context, req *model.AgentQueryReq, userID int, writer io.Writer) error
	QueryWithPipeline(ctx context.Context, req *model.AgentQueryReq, userID int) (*model.AgentQueryResponse, error)
	QueryStreamWithPipeline(ctx context.Context, req *model.AgentQueryReq, userID int, writer io.Writer) error
	CreateSession(ctx context.Context, req *model.CreateAgentSessionReq, userID int) (*model.AgentSession, error)
	ListSessions(ctx context.Context, req *model.ListAgentSessionsReq, userID int) (model.ListResp[*model.AgentSession], error)
	GetSession(ctx context.Context, id int) (*model.AgentSession, error)
	GetSessionByKey(ctx context.Context, key string) (*model.AgentSession, error)
	DeleteSession(ctx context.Context, id int) error
	UpdateSession(ctx context.Context, session *model.AgentSession) error
	ListMessages(ctx context.Context, req *model.ListAgentMessagesReq) (model.ListResp[*model.AgentMessage], error)
	ListTools(ctx context.Context) ([]map[string]interface{}, error)
}

// agentService 智能体服务实现，集成 Eino ReAct Agent
type agentService struct {
	dao           dao.AgentDAO
	toolMgr       *manager.ToolManager
	riskEval      *risk.Evaluator
	auditStore    agentaudit.Store
	cfg           *Config
	logger        *zap.Logger
	pipelineStage *pipeline.Stage            // optional pipeline enhancement
	nudgeReviewer *nudge.MemoryNudgeReviewer // optional memory nudge
}

// NewAgentService 创建智能体服务实例
func NewAgentService(
	dao dao.AgentDAO,
	toolMgr *manager.ToolManager,
	riskEval *risk.Evaluator,
	auditStore agentaudit.Store,
	cfg *Config,
	logger *zap.Logger,
	pipelineStage *pipeline.Stage,
	nudgeReviewer *nudge.MemoryNudgeReviewer,
) AgentService {
	return &agentService{
		dao:           dao,
		toolMgr:       toolMgr,
		riskEval:      riskEval,
		auditStore:    auditStore,
		cfg:           cfg,
		logger:        logger,
		pipelineStage: pipelineStage,
		nudgeReviewer: nudgeReviewer,
	}
}

func (s *agentService) auditEvent(ctx context.Context, action, toolName, reason string, riskLevel agentmodel.RiskLevel, allowed bool, args string, result string, sessionID string, userID int, username string) {
	if s.auditStore == nil {
		return
	}
	metadata := make(map[string]interface{})
	if args != "" {
		metadata["args"] = args
	}
	if result != "" {
		metadata["result"] = result
	}
	event := agentmodel.AuditEvent{
		SessionID: sessionID,
		UserID:    uint(userID),
		Username:  username,
		Action:    action,
		ToolName:  toolName,
		Risk:      riskLevel,
		Allowed:   allowed,
		Reason:    reason,
		Metadata:  metadata,
	}
	if _, err := s.auditStore.Append(ctx, event); err != nil {
		s.logger.Warn("audit append failed", zap.Error(err))
	}
}

// createChatModel 根据配置创建 OpenAI 兼容的 ChatModel
func (s *agentService) createChatModel(ctx context.Context) (*einoopenai.ChatModel, error) {
	temp := float32(s.cfg.LLM.Temperature)
	maxTokens := s.cfg.LLM.MaxTokens

	return einoopenai.NewChatModel(ctx, &einoopenai.ChatModelConfig{
		BaseURL:     s.cfg.LLM.BaseURL,
		APIKey:      s.cfg.LLM.APIKey,
		Model:       s.cfg.LLM.Model,
		Temperature: &temp,
		MaxTokens:   &maxTokens,
	})
}

// Query 同步查询智能体
func (s *agentService) Query(ctx context.Context, req *model.AgentQueryReq, userID int) (*model.AgentQueryResponse, error) {
	// 自动创建会话（如果 session_id 为空）
	if req.SessionID == "" {
		session, err := s.CreateSession(ctx, &model.CreateAgentSessionReq{Title: "新对话"}, userID)
		if err != nil {
			return nil, fmt.Errorf("自动创建会话失败: %w", err)
		}
		req.SessionID = strconv.Itoa(session.ID)
	}

	// 解析会话 ID
	sessionID, err := strconv.Atoi(req.SessionID)
	if err != nil {
		return nil, fmt.Errorf("无效的会话 ID: %w", err)
	}

	// 校验会话存在
	session, err := s.dao.GetSession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("获取会话失败: %w", err)
	}
	_ = session

	// 保存用户消息
	if err := s.dao.CreateMessage(ctx, &model.AgentMessage{
		SessionID: req.SessionID,
		Role:      model.AgentMessageRoleUser,
		Content:   req.Question,
	}); err != nil {
		s.logger.Warn("保存用户消息失败", zap.Error(err))
	}

	// 审计: 接收用户消息
	s.auditEvent(ctx, agentaudit.ActionReceive, "", "", "", true, "", req.Question, req.SessionID, userID, "")

	// === 注入检测（在 Agent 推理之前） ===
	if s.pipelineStage != nil {
		pc := &pipeline.PipelineContext{
			UserInput: req.Question,
			SessionID: req.SessionID,
			UserID:    userID,
		}
		s.pipelineStage.RunIntentAnalysis(ctx, pc)
		if blocked, reason := s.pipelineStage.IsInjectionAttempt(pc); blocked {
			s.auditEvent(ctx, agentaudit.ActionReceive, "", reason, agentmodel.RiskHigh, false, "", req.Question, req.SessionID, userID, "")
			return &model.AgentQueryResponse{
				SessionID: req.SessionID,
				Answer:    "⚠️ 检测到提示词注入攻击，请求已拦截。原因: " + reason,
			}, nil
		}
	}

	// 加载历史消息
	history := s.loadHistory(ctx, req.SessionID)

	// 获取所有可用工具并包装安全拦截
	rawTools := s.toolMgr.GetAllTools(ctx)
	safeTools := make([]tool.BaseTool, 0, len(rawTools))
	for _, t := range rawTools {
		wt, err := wrapTool(t, s.riskEval, func(ctx context.Context, action, toolName, reason string, riskLevel agentmodel.RiskLevel, allowed bool, args string, result string) {
			s.auditEvent(ctx, action, toolName, reason, riskLevel, allowed, args, result, req.SessionID, userID, "")
		})
		if err != nil {
			s.logger.Warn("wrap tool failed, using original", zap.Error(err))
			safeTools = append(safeTools, t)
			continue
		}
		safeTools = append(safeTools, wt)
	}

	// 创建 ChatModel
	chatModel, err := s.createChatModel(ctx)
	if err != nil {
		return nil, fmt.Errorf("创建 ChatModel 失败: %w", err)
	}

	// 创建 ReAct Agent
	agent, err := react.NewAgent(ctx, &react.AgentConfig{
		ToolCallingModel: chatModel,
		ToolsConfig:      compose.ToolsNodeConfig{Tools: safeTools},
		MaxStep:          10,
		MessageModifier:  react.NewPersonaModifier(personaPrompt),
	})
	if err != nil {
		return nil, fmt.Errorf("创建 Agent 失败: %w", err)
	}

	// 构建消息列表：历史 + 当前问题
	messages := make([]*schema.Message, 0, len(history)+1)
	messages = append(messages, history...)
	messages = append(messages, &schema.Message{Role: schema.User, Content: req.Question})

	// 执行 Agent
	result, err := agent.Generate(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("Agent 执行失败: %w", err)
	}

	// 提取回复内容
	answer := result.Content
	if answer == "" && len(result.ToolCalls) > 0 {
		tcJSON, _ := json.Marshal(result.ToolCalls)
		answer = string(tcJSON)
	}

	// Nudge: record turn and check for memory review
	if s.nudgeReviewer != nil {
		toolCallCount := len(result.ToolCalls)
		s.nudgeReviewer.RecordTurn(toolCallCount)
		if shouldMemory, _ := s.nudgeReviewer.ShouldNudge(toolCallCount); shouldMemory {
			conversationText := buildConversationText(messages, answer)
			go s.nudgeReviewer.Review(context.Background(), conversationText)
			s.nudgeReviewer.ResetMemory()
		}
	}

	// 保存助手消息
	if err := s.dao.CreateMessage(ctx, &model.AgentMessage{
		SessionID: req.SessionID,
		Role:      model.AgentMessageRoleAssistant,
		Content:   answer,
	}); err != nil {
		s.logger.Warn("保存助手消息失败", zap.Error(err))
	}

	// 更新会话统计
	if err := s.dao.IncrementSessionCounts(ctx, sessionID, 1, 0); err != nil {
		s.logger.Warn("更新会话统计失败", zap.Error(err))
	}

	// 审计: 对话完成
	s.auditEvent(ctx, agentaudit.ActionComplete, "", "", "", true, "", answer, req.SessionID, userID, "")

	return &model.AgentQueryResponse{
		SessionID: req.SessionID,
		Answer:    answer,
	}, nil
}

// QueryWithPipeline 使用 Pipeline 增强的同步查询
func (s *agentService) QueryWithPipeline(ctx context.Context, req *model.AgentQueryReq, userID int) (*model.AgentQueryResponse, error) {
	// 自动创建会话（如果 session_id 为空）
	if req.SessionID == "" {
		session, err := s.CreateSession(ctx, &model.CreateAgentSessionReq{Title: "新对话"}, userID)
		if err != nil {
			return nil, fmt.Errorf("自动创建会话失败: %w", err)
		}
		req.SessionID = strconv.Itoa(session.ID)
	}

	// 解析会话 ID
	sessionID, err := strconv.Atoi(req.SessionID)
	if err != nil {
		return nil, fmt.Errorf("无效的会话 ID: %w", err)
	}

	// 校验会话存在
	session, err := s.dao.GetSession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("获取会话失败: %w", err)
	}
	_ = session

	// 保存用户消息
	if err := s.dao.CreateMessage(ctx, &model.AgentMessage{
		SessionID: req.SessionID,
		Role:      model.AgentMessageRoleUser,
		Content:   req.Question,
	}); err != nil {
		s.logger.Warn("保存用户消息失败", zap.Error(err))
	}

	// 审计: 接收用户消息
	s.auditEvent(ctx, agentaudit.ActionReceive, "", "", "", true, "", req.Question, req.SessionID, userID, "")

	// === ① 意图分析 ===
	pc := &pipeline.PipelineContext{
		UserInput: req.Question,
		SessionID: req.SessionID,
		UserID:    userID,
	}
	if s.pipelineStage != nil {
		s.pipelineStage.RunIntentAnalysis(ctx, pc)
	}

	// === 注入检测 ===
	if s.pipelineStage != nil {
		if blocked, reason := s.pipelineStage.IsInjectionAttempt(pc); blocked {
			s.auditEvent(ctx, agentaudit.ActionReceive, "", reason, agentmodel.RiskHigh, false, "", req.Question, req.SessionID, userID, "")
			return &model.AgentQueryResponse{
				SessionID: req.SessionID,
				Answer:    "⚠️ 检测到提示词注入攻击，请求已拦截。原因: " + reason,
			}, nil
		}
	}

	// === ② 记忆增强 ===
	memoryContext := ""
	if s.pipelineStage != nil {
		memoryContext, _ = s.pipelineStage.RunMemoryEnrichment(ctx, pc)
	}

	// 加载历史消息
	history := s.loadHistory(ctx, req.SessionID)

	// 构建增强后的 system prompt
	enrichedPrompt := personaPrompt
	if memoryContext != "" {
		enrichedPrompt += "\n" + memoryContext
	}

	// 获取所有可用工具并包装安全拦截
	rawTools := s.toolMgr.GetAllTools(ctx)
	safeTools := make([]tool.BaseTool, 0, len(rawTools))
	for _, t := range rawTools {
		wt, err := wrapTool(t, s.riskEval, func(ctx context.Context, action, toolName, reason string, riskLevel agentmodel.RiskLevel, allowed bool, args string, result string) {
			s.auditEvent(ctx, action, toolName, reason, riskLevel, allowed, args, result, req.SessionID, userID, "")
		})
		if err != nil {
			s.logger.Warn("wrap tool failed, using original", zap.Error(err))
			safeTools = append(safeTools, t)
			continue
		}
		safeTools = append(safeTools, wt)
	}

	// 创建 ChatModel
	chatModel, err := s.createChatModel(ctx)
	if err != nil {
		return nil, fmt.Errorf("创建 ChatModel 失败: %w", err)
	}

	// 创建 ReAct Agent（使用增强后的 prompt）
	agent, err := react.NewAgent(ctx, &react.AgentConfig{
		ToolCallingModel: chatModel,
		ToolsConfig:      compose.ToolsNodeConfig{Tools: safeTools},
		MaxStep:          10,
		MessageModifier:  react.NewPersonaModifier(enrichedPrompt),
	})
	if err != nil {
		return nil, fmt.Errorf("创建 Agent 失败: %w", err)
	}

	// 构建消息列表：历史 + 当前问题
	messages := make([]*schema.Message, 0, len(history)+1)
	messages = append(messages, history...)
	messages = append(messages, &schema.Message{Role: schema.User, Content: req.Question})

	// 执行 Agent
	result, err := agent.Generate(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("Agent 执行失败: %w", err)
	}

	// 提取回复内容
	answer := result.Content
	if answer == "" && len(result.ToolCalls) > 0 {
		tcJSON, _ := json.Marshal(result.ToolCalls)
		answer = string(tcJSON)
	}

	// Nudge: record turn and check for memory review
	if s.nudgeReviewer != nil {
		toolCallCount := len(result.ToolCalls)
		s.nudgeReviewer.RecordTurn(toolCallCount)
		if shouldMemory, _ := s.nudgeReviewer.ShouldNudge(toolCallCount); shouldMemory {
			conversationText := buildConversationText(messages, answer)
			go s.nudgeReviewer.Review(context.Background(), conversationText)
			s.nudgeReviewer.ResetMemory()
		}
	}

	// 保存助手消息
	if err := s.dao.CreateMessage(ctx, &model.AgentMessage{
		SessionID: req.SessionID,
		Role:      model.AgentMessageRoleAssistant,
		Content:   answer,
	}); err != nil {
		s.logger.Warn("保存助手消息失败", zap.Error(err))
	}

	// 更新会话统计
	if err := s.dao.IncrementSessionCounts(ctx, sessionID, 1, 0); err != nil {
		s.logger.Warn("更新会话统计失败", zap.Error(err))
	}

	// 审计: 对话完成
	s.auditEvent(ctx, agentaudit.ActionComplete, "", "", "", true, "", answer, req.SessionID, userID, "")

	return &model.AgentQueryResponse{
		SessionID: req.SessionID,
		Answer:    answer,
	}, nil
}

// QueryStream 流式查询智能体
func (s *agentService) QueryStream(ctx context.Context, req *model.AgentQueryReq, userID int, writer io.Writer) error {
	// 自动创建会话（如果 session_id 为空）
	if req.SessionID == "" {
		session, err := s.CreateSession(ctx, &model.CreateAgentSessionReq{Title: "新对话"}, userID)
		if err != nil {
			return fmt.Errorf("自动创建会话失败: %w", err)
		}
		req.SessionID = strconv.Itoa(session.ID)
	}

	sessionID, err := strconv.Atoi(req.SessionID)
	if err != nil {
		return fmt.Errorf("无效的会话 ID: %w", err)
	}

	// 校验会话存在
	if _, err := s.dao.GetSession(ctx, sessionID); err != nil {
		return fmt.Errorf("获取会话失败: %w", err)
	}

	// 保存用户消息
	if err := s.dao.CreateMessage(ctx, &model.AgentMessage{
		SessionID: req.SessionID,
		Role:      model.AgentMessageRoleUser,
		Content:   req.Question,
	}); err != nil {
		s.logger.Warn("保存用户消息失败", zap.Error(err))
	}

	// 审计: 接收用户消息
	s.auditEvent(ctx, agentaudit.ActionReceive, "", "", "", true, "", req.Question, req.SessionID, userID, "")

	// === 注入检测 ===
	if s.pipelineStage != nil {
		pc := &pipeline.PipelineContext{
			UserInput: req.Question,
			SessionID: req.SessionID,
			UserID:    userID,
		}
		s.pipelineStage.RunIntentAnalysis(ctx, pc)
		if blocked, reason := s.pipelineStage.IsInjectionAttempt(pc); blocked {
			s.auditEvent(ctx, agentaudit.ActionReceive, "", reason, agentmodel.RiskHigh, false, "", req.Question, req.SessionID, userID, "")
			_ = s.writeSSEEvent(writer, "error", map[string]string{
				"error": "⚠️ 检测到提示词注入攻击，请求已拦截。原因: " + reason,
			})
			return fmt.Errorf("injection blocked: %s", reason)
		}
	}

	// 加载历史消息
	history := s.loadHistory(ctx, req.SessionID)

	// 创建 ChatModel
	chatModel, err := s.createChatModel(ctx)
	if err != nil {
		return fmt.Errorf("创建 ChatModel 失败: %w", err)
	}

	// 构建消息列表
	messages := make([]*schema.Message, 0, len(history)+1)
	messages = append(messages, history...)
	messages = append(messages, &schema.Message{Role: schema.User, Content: req.Question})

	// 获取所有工具并包装安全层（支持工具结果回调到 SSE）
	rawTools := s.toolMgr.GetAllTools(ctx)
	safeTools := make([]tool.BaseTool, 0, len(rawTools))
	for _, t := range rawTools {
		wt, err := wrapToolWithCallback(t, s.riskEval,
			func(ctx context.Context, action, toolName, reason string, riskLevel agentmodel.RiskLevel, allowed bool, args string, result string) {
				s.auditEvent(ctx, action, toolName, reason, riskLevel, allowed, args, result, req.SessionID, userID, "")
			},
			func(toolCallID, toolName, args string) {
				_ = s.writeSSEEvent(writer, "tool_call", map[string]interface{}{
					"id":        toolCallID,
					"name":      toolName,
					"arguments": args,
				})
			},
			func(toolCallID, toolName, result, status string) {
				_ = s.writeSSEEvent(writer, "tool_result", map[string]interface{}{
					"id":     toolCallID,
					"name":   toolName,
					"status": status,
					"result": result,
				})
			},
		)
		if err != nil {
			s.logger.Warn("wrap tool failed, using original", zap.Error(err))
			safeTools = append(safeTools, t)
			continue
		}
		safeTools = append(safeTools, wt)
	}

	// 使用 react.Agent 流式执行（标准 OpenAI function calling）
	finalContent, err := runAgentStream(
		ctx, chatModel, safeTools, messages, writer, s.writeSSEEvent,
		req.SessionID, personaPrompt,
	)
	if err != nil {
		s.logger.Error("Agent Stream 执行失败", zap.Error(err))
		return fmt.Errorf("Agent Stream 失败: %w", err)
	}

	// Nudge: record turn and check for memory review (streaming path)
	if s.nudgeReviewer != nil {
		toolCallCount := 0 // streaming path doesn't expose tool call count
		s.nudgeReviewer.RecordTurn(toolCallCount)
		if shouldMemory, _ := s.nudgeReviewer.ShouldNudge(toolCallCount); shouldMemory {
			conversationText := buildConversationText(messages, finalContent)
			go s.nudgeReviewer.Review(context.Background(), conversationText)
			s.nudgeReviewer.ResetMemory()
		}
	}

	// 审计: 会话完成
	s.auditEvent(ctx, agentaudit.ActionComplete, "", "", "", true, "", finalContent, req.SessionID, userID, "")

	// 持久化助手消息
	if finalContent != "" {
		if err := s.dao.CreateMessage(ctx, &model.AgentMessage{
			SessionID: req.SessionID,
			Role:      model.AgentMessageRoleAssistant,
			Content:   finalContent,
		}); err != nil {
			s.logger.Warn("保存流式助手消息失败", zap.Error(err))
		}
		if err := s.dao.IncrementSessionCounts(ctx, sessionID, 1, 0); err != nil {
			s.logger.Warn("更新会话统计失败", zap.Error(err))
		}
	}

	return nil
}

// QueryStreamWithPipeline 使用 Pipeline 增强的流式查询
func (s *agentService) QueryStreamWithPipeline(ctx context.Context, req *model.AgentQueryReq, userID int, writer io.Writer) error {
	// 自动创建会话（如果 session_id 为空）
	if req.SessionID == "" {
		session, err := s.CreateSession(ctx, &model.CreateAgentSessionReq{Title: "新对话"}, userID)
		if err != nil {
			return fmt.Errorf("自动创建会话失败: %w", err)
		}
		req.SessionID = strconv.Itoa(session.ID)
	}

	sessionID, err := strconv.Atoi(req.SessionID)
	if err != nil {
		return fmt.Errorf("无效的会话 ID: %w", err)
	}

	// 校验会话存在
	if _, err := s.dao.GetSession(ctx, sessionID); err != nil {
		return fmt.Errorf("获取会话失败: %w", err)
	}

	// 保存用户消息
	if err := s.dao.CreateMessage(ctx, &model.AgentMessage{
		SessionID: req.SessionID,
		Role:      model.AgentMessageRoleUser,
		Content:   req.Question,
	}); err != nil {
		s.logger.Warn("保存用户消息失败", zap.Error(err))
	}

	// 审计: 接收用户消息
	s.auditEvent(ctx, agentaudit.ActionReceive, "", "", "", true, "", req.Question, req.SessionID, userID, "")

	// === ① 意图分析 ===
	pc := &pipeline.PipelineContext{
		UserInput: req.Question,
		SessionID: req.SessionID,
		UserID:    userID,
		Writer:    writer,
	}
	if s.pipelineStage != nil {
		s.pipelineStage.RunIntentAnalysis(ctx, pc)
	}

	// === 注入检测（SSE 错误事件） ===
	if s.pipelineStage != nil {
		if blocked, reason := s.pipelineStage.IsInjectionAttempt(pc); blocked {
			s.auditEvent(ctx, agentaudit.ActionReceive, "", reason, agentmodel.RiskHigh, false, "", req.Question, req.SessionID, userID, "")
			s.writeSSEEvent(writer, "error", map[string]string{"error": "⚠️ 检测到提示词注入攻击，请求已拦截。原因: " + reason})
			return fmt.Errorf("injection blocked: %s", reason)
		}
	}

	// === ② 记忆增强 ===
	memoryContext := ""
	if s.pipelineStage != nil {
		memoryContext, _ = s.pipelineStage.RunMemoryEnrichment(ctx, pc)
	}

	// 加载历史消息
	history := s.loadHistory(ctx, req.SessionID)

	// 构建增强后的 system prompt
	enrichedPrompt := personaPrompt
	if memoryContext != "" {
		enrichedPrompt += "\n" + memoryContext
	}

	// 创建 ChatModel
	chatModel, err := s.createChatModel(ctx)
	if err != nil {
		return fmt.Errorf("创建 ChatModel 失败: %w", err)
	}

	// 构建消息列表
	messages := make([]*schema.Message, 0, len(history)+1)
	messages = append(messages, history...)
	messages = append(messages, &schema.Message{Role: schema.User, Content: req.Question})

	// 获取所有工具并包装安全层（支持工具结果回调到 SSE）
	rawTools := s.toolMgr.GetAllTools(ctx)
	safeTools := make([]tool.BaseTool, 0, len(rawTools))
	for _, t := range rawTools {
		wt, err := wrapToolWithCallback(t, s.riskEval,
			func(ctx context.Context, action, toolName, reason string, riskLevel agentmodel.RiskLevel, allowed bool, args string, result string) {
				s.auditEvent(ctx, action, toolName, reason, riskLevel, allowed, args, result, req.SessionID, userID, "")
			},
			func(toolCallID, toolName, args string) {
				_ = s.writeSSEEvent(writer, "tool_call", map[string]interface{}{
					"id":        toolCallID,
					"name":      toolName,
					"arguments": args,
				})
			},
			func(toolCallID, toolName, result, status string) {
				_ = s.writeSSEEvent(writer, "tool_result", map[string]interface{}{
					"id":     toolCallID,
					"name":   toolName,
					"status": status,
					"result": result,
				})
			},
		)
		if err != nil {
			s.logger.Warn("wrap tool failed, using original", zap.Error(err))
			safeTools = append(safeTools, t)
			continue
		}
		safeTools = append(safeTools, wt)
	}

	// 使用 react.Agent 流式执行（标准 OpenAI function calling）
	finalContent, err := runAgentStream(
		ctx, chatModel, safeTools, messages, writer, s.writeSSEEvent,
		req.SessionID, enrichedPrompt,
	)
	if err != nil {
		s.logger.Error("Agent Stream 执行失败", zap.Error(err))
		return fmt.Errorf("Agent Stream 失败: %w", err)
	}

	// Nudge: record turn and check for memory review (streaming path)
	if s.nudgeReviewer != nil {
		toolCallCount := 0 // streaming path doesn't expose tool call count
		s.nudgeReviewer.RecordTurn(toolCallCount)
		if shouldMemory, _ := s.nudgeReviewer.ShouldNudge(toolCallCount); shouldMemory {
			conversationText := buildConversationText(messages, finalContent)
			go s.nudgeReviewer.Review(context.Background(), conversationText)
			s.nudgeReviewer.ResetMemory()
		}
	}

	// 审计: 会话完成
	s.auditEvent(ctx, agentaudit.ActionComplete, "", "", "", true, "", finalContent, req.SessionID, userID, "")

	// 持久化助手消息
	if finalContent != "" {
		if err := s.dao.CreateMessage(ctx, &model.AgentMessage{
			SessionID: req.SessionID,
			Role:      model.AgentMessageRoleAssistant,
			Content:   finalContent,
		}); err != nil {
			s.logger.Warn("保存流式助手消息失败", zap.Error(err))
		}
		if err := s.dao.IncrementSessionCounts(ctx, sessionID, 1, 0); err != nil {
			s.logger.Warn("更新会话统计失败", zap.Error(err))
		}
	}

	return nil
}

// writeSSEEvent 将 SSE 事件写入 writer
func (s *agentService) writeSSEEvent(writer io.Writer, event string, data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		s.logger.Warn("[SSE] JSON 序列化失败", zap.String("event", event), zap.Error(err))
		return err
	}
	s.logger.Info("[SSE] 推送事件到前端",
		zap.String("event", event),
		zap.String("data", truncateStringForLog(string(jsonData), 500)),
	)
	_, err = fmt.Fprintf(writer, "event: %s\ndata: %s\n\n", event, string(jsonData))
	if err != nil {
		s.logger.Warn("[SSE] 写入失败", zap.String("event", event), zap.Error(err))
	}
	return err
}

// truncateStringForLog 截断字符串用于日志输出，避免日志过大
func truncateStringForLog(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "...(truncated)"
}

// loadHistory 从数据库加载最近的历史消息并转换为 Eino 格式
func (s *agentService) loadHistory(ctx context.Context, sessionID string) []*schema.Message {
	maxHistory := s.cfg.MaxHistory
	if maxHistory <= 0 {
		maxHistory = 20
	}

	req := &model.ListAgentMessagesReq{
		SessionID: sessionID,
		ListReq: model.ListReq{
			Page: 1,
			Size: maxHistory,
		},
	}

	msgs, _, err := s.dao.ListMessages(ctx, req)
	if err != nil {
		s.logger.Warn("加载历史消息失败",
			zap.String("session_id", sessionID), zap.Error(err))
		return nil
	}

	result := make([]*schema.Message, 0, len(msgs))
	for _, msg := range msgs {
		result = append(result, convertToEinoMessage(msg))
	}
	return result
}

// convertToEinoMessage 将数据库消息转换为 Eino schema.Message
func convertToEinoMessage(msg *model.AgentMessage) *schema.Message {
	switch msg.Role {
	case model.AgentMessageRoleUser:
		return &schema.Message{Role: schema.User, Content: msg.Content}
	case model.AgentMessageRoleAssistant:
		m := &schema.Message{Role: schema.Assistant, Content: msg.Content}
		if msg.ToolCalls != nil {
			if raw, err := json.Marshal(msg.ToolCalls); err == nil {
				var calls []schema.ToolCall
				if err := json.Unmarshal(raw, &calls); err == nil {
					m.ToolCalls = calls
				}
			}
		}
		return m
	case model.AgentMessageRoleSystem:
		return &schema.Message{Role: schema.System, Content: msg.Content}
	case model.AgentMessageRoleTool:
		return schema.ToolMessage(msg.Content, msg.ToolCallID, schema.WithToolName(""))
	default:
		return &schema.Message{Role: schema.User, Content: msg.Content}
	}
}

// ==================== 会话管理 ====================

// CreateSession 创建新会话
func (s *agentService) CreateSession(ctx context.Context, req *model.CreateAgentSessionReq, userID int) (*model.AgentSession, error) {
	title := req.Title
	if title == "" {
		title = "新对话"
	}
	modelName := req.Model
	if modelName == "" {
		modelName = s.cfg.LLM.Model
	}

	tools := s.toolMgr.GetAllTools(ctx)

	session := &model.AgentSession{
		UserID:     userID,
		SessionKey: req.SessionKey,
		Title:      title,
		ModelName:  modelName,
		ToolCount:  len(tools),
		Status:     model.AgentSessionStatusActive,
	}

	if err := s.dao.CreateSession(ctx, session); err != nil {
		return nil, fmt.Errorf("创建会话失败: %w", err)
	}

	return session, nil
}

// ListSessions 获取会话列表
func (s *agentService) ListSessions(ctx context.Context, req *model.ListAgentSessionsReq, userID int) (model.ListResp[*model.AgentSession], error) {
	sessions, total, err := s.dao.ListSessions(ctx, req)
	if err != nil {
		return model.ListResp[*model.AgentSession]{}, fmt.Errorf("查询会话列表失败: %w", err)
	}

	return model.ListResp[*model.AgentSession]{
		Items: sessions,
		Total: total,
	}, nil
}

// GetSession 获取会话详情
func (s *agentService) GetSession(ctx context.Context, id int) (*model.AgentSession, error) {
	session, err := s.dao.GetSession(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("获取会话失败: %w", err)
	}
	return session, nil
}

func (s *agentService) GetSessionByKey(ctx context.Context, key string) (*model.AgentSession, error) {
	session, err := s.dao.GetSessionByKey(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("获取会话失败: %w", err)
	}
	return session, nil
}

// DeleteSession 删除会话
func (s *agentService) DeleteSession(ctx context.Context, id int) error {
	if err := s.dao.DeleteSession(ctx, id); err != nil {
		return fmt.Errorf("删除会话失败: %w", err)
	}
	return nil
}

// UpdateSession 更新会话
func (s *agentService) UpdateSession(ctx context.Context, session *model.AgentSession) error {
	if err := s.dao.UpdateSession(ctx, session); err != nil {
		return fmt.Errorf("更新会话失败: %w", err)
	}
	return nil
}

// ListMessages 获取消息列表
func (s *agentService) ListMessages(ctx context.Context, req *model.ListAgentMessagesReq) (model.ListResp[*model.AgentMessage], error) {
	msgs, total, err := s.dao.ListMessages(ctx, req)
	if err != nil {
		return model.ListResp[*model.AgentMessage]{}, fmt.Errorf("查询消息列表失败: %w", err)
	}

	return model.ListResp[*model.AgentMessage]{
		Items: msgs,
		Total: total,
	}, nil
}

// ListTools 获取所有可用工具列表
func (s *agentService) ListTools(ctx context.Context) ([]map[string]interface{}, error) {
	tools := s.toolMgr.GetAllTools(ctx)
	result := make([]map[string]interface{}, 0, len(tools))
	for _, t := range tools {
		if it, ok := t.(tool.InvokableTool); ok {
			info, err := it.Info(ctx)
			if err != nil {
				s.logger.Warn("获取工具信息失败", zap.Error(err))
				continue
			}
			result = append(result, map[string]interface{}{
				"name":        info.Name,
				"description": info.Desc,
				"params":      info.ParamsOneOf,
			})
		}
	}
	return result, nil
}

// buildConversationText 将消息列表和回复转换为对话文本，用于记忆审查
func buildConversationText(messages []*schema.Message, answer string) string {
	var sb strings.Builder
	for _, msg := range messages {
		role := string(msg.Role)
		content := msg.Content
		if content != "" {
			sb.WriteString(fmt.Sprintf("[%s]: %s\n", role, content))
		}
		for _, tc := range msg.ToolCalls {
			sb.WriteString(fmt.Sprintf("[%s tool_call %s]: %s\n", role, tc.Function.Name, tc.Function.Arguments))
		}
	}
	if answer != "" {
		sb.WriteString(fmt.Sprintf("[assistant]: %s\n", answer))
	}
	return sb.String()
}
