package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"go.uber.org/zap"

	agentService "github.com/rizxfrog/VanPanelBackend/internal/agent/service"
	"github.com/rizxfrog/VanPanelBackend/internal/model"
	"github.com/rizxfrog/VanPanelBackend/internal/gateway"
	"github.com/rizxfrog/VanPanelBackend/internal/gateway/adapter"
)

// agentSvc is set during server initialization to inject AgentService into chat handlers.
var agentSvc agentService.AgentService
var defaultUserID = 1 // default user for gateway chat

// SetAgentService sets the AgentService for gateway chat handlers.
func SetAgentService(svc agentService.AgentService) {
	agentSvc = svc
}

func init() {
	gateway.RegisterMethod("chat.send", string(gateway.ScopeWrite), handleChatSend)
	gateway.RegisterMethod("chat.abort", string(gateway.ScopeWrite), handleChatAbort)
	gateway.RegisterMethod("chat.history", string(gateway.ScopeRead), handleChatHistory)
	gateway.RegisterMethod("chat.startup", string(gateway.ScopeRead), handleChatStartup)
	gateway.RegisterMethod("chat.metadata", string(gateway.ScopeRead), handleChatMetadata)
	gateway.RegisterMethod("chat.message.get", string(gateway.ScopeRead), handleChatMessageGet)
}

func handleChatSend(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	var req struct {
		SessionKey string `json:"sessionKey"`
		Message    string `json:"message"`
		AgentID    string `json:"agentId,omitempty"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, fmt.Errorf("解析chat.send参数失败: %w", err)
	}

	if agentSvc == nil {
		zap.L().Warn("chat.send: AgentService 未初始化，使用模拟回复")
		return map[string]interface{}{
			"runId":  "sim-" + time.Now().Format("20060102150405"),
			"status": "started",
		}, nil
	}

	runID := "run-" + time.Now().Format("20060102150405")

	// Auto-create session for the gateway sessionKey if not exists
	session, err := ensureSession(ctx, req.SessionKey)
	if err != nil {
		return nil, fmt.Errorf("创建会话失败: %w", err)
	}

	// Start streaming agent query in background goroutine
	go func() {
		adapter := adapter.NewChatStreamAdapter(context.Background(), conn, runID, req.SessionKey, req.AgentID)

		agentReq := &model.AgentQueryReq{
			SessionID: strconv.Itoa(session.ID),
			Question:  req.Message,
		}

		if err := agentSvc.QueryStream(context.Background(), agentReq, defaultUserID, adapter); err != nil {
			zap.L().Error("Agent query stream failed", zap.Error(err))
			conn.SendEvent("chat", gateway.ChatEvent{
				RunID:      runID,
				SessionKey: req.SessionKey,
				AgentID:    req.AgentID,
				State:      gateway.ChatStateError,
				ErrorMsg:   err.Error(),
				ErrorKind:  "agent_error",
			})
		}
	}()

	return map[string]interface{}{
		"runId":  runID,
		"status": "started",
	}, nil
}

// ensureSession creates or retrieves a session for the gateway sessionKey
func ensureSession(ctx context.Context, sessionKey string) (*model.AgentSession, error) {
	if agentSvc == nil {
		return nil, fmt.Errorf("AgentService 未初始化")
	}
	// Try to find existing session by listing recent sessions
	sessions, err := agentSvc.ListSessions(ctx, &model.ListAgentSessionsReq{
		ListReq: model.ListReq{
			Page: 1,
			Size: 1,
		},
	}, defaultUserID)
	if err != nil {
		return nil, fmt.Errorf("查询会话列表失败: %w", err)
	}
	if len(sessions.Items) > 0 {
		return sessions.Items[0], nil
	}
	// Create new session
	return agentSvc.CreateSession(ctx, &model.CreateAgentSessionReq{
		Title: "Gateway Chat",
	}, defaultUserID)
}

func handleChatAbort(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]bool{"ok": true}, nil
}

func handleChatHistory(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	if agentSvc == nil {
		return map[string]interface{}{"messages": []interface{}{}, "hasMore": false}, nil
	}

	// Try to get recent messages from the most recent session
	sessions, _ := agentSvc.ListSessions(ctx, &model.ListAgentSessionsReq{
		ListReq: model.ListReq{Page: 1, Size: 1},
	}, defaultUserID)
	if len(sessions.Items) == 0 {
		return map[string]interface{}{"messages": []interface{}{}, "hasMore": false}, nil
	}

	messages, _ := agentSvc.ListMessages(ctx, &model.ListAgentMessagesReq{
		SessionID: strconv.Itoa(sessions.Items[0].ID),
		ListReq:   model.ListReq{Page: 1, Size: 50},
	})

	// Convert to OpenClaw format (chronological order)
	var openclawMsgs []map[string]interface{}
	for i := len(messages.Items) - 1; i >= 0; i-- {
		msg := messages.Items[i]
		openclawMsgs = append(openclawMsgs, map[string]interface{}{
			"role":      msg.Role,
			"content":   []map[string]string{{"type": "text", "text": msg.Content}},
			"timestamp": msg.CreatedAt.UnixMilli(),
		})
	}

	return map[string]interface{}{
		"messages": openclawMsgs,
		"hasMore":  false,
	}, nil
}

func handleChatStartup(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{
		"messages":   []interface{}{},
		"sessionKey": "agent:main:global",
	}, nil
}

func handleChatMetadata(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{
		"availableModels": []string{"gpt-4o"},
		"defaultModel":    "gpt-4o",
	}, nil
}

func handleChatMessageGet(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	var req struct {
		SessionKey string `json:"sessionKey"`
		MessageID  string `json:"messageId"`
	}
	json.Unmarshal(params, &req)

	return map[string]interface{}{
		"message": map[string]interface{}{
			"role":    "assistant",
			"content": []map[string]string{{"type": "text", "text": "Message content"}},
		},
	}, nil
}
