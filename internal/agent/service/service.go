package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	einoopenai "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"

	"github.com/rizxfrog/VanPanelBackend/internal/agent/dao"
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
	CreateSession(ctx context.Context, req *model.CreateAgentSessionReq, userID int) (*model.AgentSession, error)
	ListSessions(ctx context.Context, req *model.ListAgentSessionsReq, userID int) (model.ListResp[*model.AgentSession], error)
	GetSession(ctx context.Context, id int) (*model.AgentSession, error)
	DeleteSession(ctx context.Context, id int) error
	ListMessages(ctx context.Context, req *model.ListAgentMessagesReq) (model.ListResp[*model.AgentMessage], error)
	ListTools(ctx context.Context) ([]map[string]interface{}, error)
}

// agentService 智能体服务实现，集成 Eino ReAct Agent
type agentService struct {
	dao      dao.AgentDAO
	toolMgr  *manager.ToolManager
	riskEval *risk.Evaluator
	cfg      *Config
	logger   *zap.Logger
}

// NewAgentService 创建智能体服务实例
func NewAgentService(
	dao dao.AgentDAO,
	toolMgr *manager.ToolManager,
	riskEval *risk.Evaluator,
	cfg *Config,
	logger *zap.Logger,
) AgentService {
	return &agentService{
		dao:      dao,
		toolMgr:  toolMgr,
		riskEval: riskEval,
		cfg:      cfg,
		logger:   logger,
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

	// 加载历史消息
	history := s.loadHistory(ctx, req.SessionID)

	// 获取所有可用工具
	tools := s.toolMgr.GetAllTools(ctx)

	// 创建 ChatModel
	chatModel, err := s.createChatModel(ctx)
	if err != nil {
		return nil, fmt.Errorf("创建 ChatModel 失败: %w", err)
	}

	// 创建 ReAct Agent
	agent, err := react.NewAgent(ctx, &react.AgentConfig{
		ToolCallingModel: chatModel,
		ToolsConfig:      compose.ToolsNodeConfig{Tools: tools},
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

	// 加载历史消息
	history := s.loadHistory(ctx, req.SessionID)

	// 获取工具和创建模型
	tools := s.toolMgr.GetAllTools(ctx)
	chatModel, err := s.createChatModel(ctx)
	if err != nil {
		return fmt.Errorf("创建 ChatModel 失败: %w", err)
	}

	// 创建 Agent
	agent, err := react.NewAgent(ctx, &react.AgentConfig{
		ToolCallingModel: chatModel,
		ToolsConfig:      compose.ToolsNodeConfig{Tools: tools},
		MaxStep:          10,
		MessageModifier:  react.NewPersonaModifier(personaPrompt),
	})
	if err != nil {
		return fmt.Errorf("创建 Agent 失败: %w", err)
	}

	// 构建消息列表
	messages := make([]*schema.Message, 0, len(history)+1)
	messages = append(messages, history...)
	messages = append(messages, &schema.Message{Role: schema.User, Content: req.Question})

	// 流式执行
	sr, err := agent.Stream(ctx, messages)
	if err != nil {
		return fmt.Errorf("Agent Stream 失败: %w", err)
	}
	defer sr.Close()

	var finalContent string

	for {
		chunk, err := sr.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			s.logger.Error("Agent Stream 接收错误", zap.Error(err))
			s.writeSSEEvent(writer, "error", map[string]string{"error": err.Error()})
			break
		}

		// 记录最终内容用于持久化
		if chunk.Content != "" {
			finalContent = chunk.Content
		}

		// 发送 SSE delta 事件
		if err := s.writeSSEEvent(writer, "delta", chunk); err != nil {
			return fmt.Errorf("写入 SSE 事件失败: %w", err)
		}
	}

	// 发送完成事件
	if err := s.writeSSEEvent(writer, "done", map[string]string{"session_id": req.SessionID}); err != nil {
		return fmt.Errorf("写入 SSE 完成事件失败: %w", err)
	}

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
		return err
	}
	_, err = fmt.Fprintf(writer, "event: %s\ndata: %s\n\n", event, string(jsonData))
	return err
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
		UserID:    userID,
		Title:     title,
		ModelName: modelName,
		ToolCount: len(tools),
		Status:    model.AgentSessionStatusActive,
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

// DeleteSession 删除会话
func (s *agentService) DeleteSession(ctx context.Context, id int) error {
	if err := s.dao.DeleteSession(ctx, id); err != nil {
		return fmt.Errorf("删除会话失败: %w", err)
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
			})
		}
	}
	return result, nil
}
