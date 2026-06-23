package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"go.uber.org/zap"

	agentService "github.com/rizxfrog/VanPanelBackend/internal/agent/service"
	"github.com/rizxfrog/VanPanelBackend/internal/gateway"
	"github.com/rizxfrog/VanPanelBackend/internal/gateway/adapter"
	"github.com/rizxfrog/VanPanelBackend/internal/model"
)

// agentSvc is set during server initialization to inject AgentService into chat handlers.
var agentSvc agentService.AgentService
var defaultUserID = 1 // default user for gateway chat

// runTracker tracks active chat runs for abort support.
var runTracker *gateway.RunTracker

// broadcastMgr is set during server initialization for event broadcasting.
var broadcastMgr *gateway.BroadcastManager

// SetAgentService sets the AgentService for gateway chat handlers.
func SetAgentService(svc agentService.AgentService) {
	agentSvc = svc
}

// SetRunTracker sets the RunTracker for gateway chat handlers.
func SetRunTracker(rt *gateway.RunTracker) {
	runTracker = rt
}

// SetBroadcastManager sets the BroadcastManager for gateway RPC handlers.
func SetBroadcastManager(bm *gateway.BroadcastManager) {
	broadcastMgr = bm
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

	runID := "run-" + time.Now().Format("20060102150405.000")

	// Ensure a session exists for this key and get the numeric ID
	sessionID, err := ensureSession(ctx, req.SessionKey)
	if err != nil {
		return nil, fmt.Errorf("创建/查找会话失败: %w", err)
	}

	// Create cancellable context for this run
	runCtx, runCancel := context.WithCancel(conn.Context())

	// Register the run with RunTracker for abort support
	if runTracker != nil {
		runTracker.Register(runID, req.SessionKey, conn.ID, runCancel)
	}

	// Start streaming agent query in background goroutine
	go func() {
		defer func() {
			if runTracker != nil {
				runTracker.Unregister(runID)
			}
		}()

		chatAdapter := adapter.NewChatStreamAdapter(runCtx, conn, runID, req.SessionKey, req.AgentID, broadcastMgr, subHub)

		agentReq := &model.AgentQueryReq{
			SessionID: strconv.Itoa(sessionID),
			Question:  req.Message,
		}

		if err := agentSvc.QueryStream(runCtx, agentReq, defaultUserID, chatAdapter); err != nil {
			// Don't send error if context was cancelled (abort)
			if runCtx.Err() != nil {
				conn.SendEvent("chat", gateway.ChatEvent{
					RunID:      runID,
					SessionKey: req.SessionKey,
					AgentID:    req.AgentID,
					State:      gateway.ChatStateAborted,
				})
				return
			}
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

// ensureSession finds or creates a session for the given gateway sessionKey.
// Returns the AgentSession.ID to use for storing messages.
func ensureSession(ctx context.Context, sessionKey string) (int, error) {
	if agentSvc == nil {
		return 0, fmt.Errorf("AgentService 未初始化")
	}

	// Try numeric ID first (format: "agent:main:<id>")
	if id, err := parseSessionKey(sessionKey); err == nil && id > 0 {
		// Session exists if GetSession succeeds
		if _, err := agentSvc.GetSession(ctx, id); err == nil {
			return id, nil
		}
	}
	// Try SessionKey lookup for non-numeric keys (e.g. "agent:main:global")
	if session, err := agentSvc.GetSessionByKey(ctx, sessionKey); err == nil {
		return session.ID, nil
	}

	// Create new session with the SessionKey set
	title := sessionKey
	if id, err := parseSessionKey(sessionKey); err == nil && id > 0 {
		title = "会话 " + strconv.Itoa(id)
	}
	session, err := agentSvc.CreateSession(ctx, &model.CreateAgentSessionReq{
		Title:      title,
		SessionKey: sessionKey,
	}, defaultUserID)
	if err != nil {
		return 0, err
	}
	return session.ID, nil
}

func handleChatAbort(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	if runTracker == nil {
		return buildOKResponse(map[string]interface{}{
			"aborted": false,
			"runIds":  []string{},
		}), nil
	}

	var req struct {
		SessionKey string `json:"sessionKey"`
		AgentID    string `json:"agentId,omitempty"`
		RunID      string `json:"runId,omitempty"`
	}
	json.Unmarshal(params, &req)

	var abortedRunIDs []string

	if req.RunID != "" {
		// Abort specific run
		entry, ok := runTracker.Get(req.RunID)
		if ok {
			entry.Cancel()
			abortedRunIDs = append(abortedRunIDs, req.RunID)
		}
	} else if req.SessionKey != "" {
		// Abort all runs for this session
		entries := runTracker.GetBySession(req.SessionKey)
		for _, entry := range entries {
			entry.Cancel()
			abortedRunIDs = append(abortedRunIDs, entry.RunID)
		}
	}

	return buildOKResponse(map[string]interface{}{
		"aborted": len(abortedRunIDs) > 0,
		"runIds":  abortedRunIDs,
	}), nil
}

func handleChatHistory(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	if agentSvc == nil {
		return map[string]interface{}{"messages": []interface{}{}, "hasMore": false}, nil
	}

	var req struct {
		SessionKey string `json:"sessionKey"`
		Limit      int    `json:"limit"`
	}
	json.Unmarshal(params, &req)

	// Resolve the database session ID from the gateway sessionKey.
	sessionID, err := ensureSession(ctx, req.SessionKey)
	if err != nil {
		// Fallback: most recent session (backward compatibility)
		sessions, _ := agentSvc.ListSessions(ctx, &model.ListAgentSessionsReq{
			ListReq: model.ListReq{Page: 1, Size: 1},
		}, defaultUserID)
		if len(sessions.Items) == 0 {
			return map[string]interface{}{"messages": []interface{}{}, "hasMore": false}, nil
		}
		sessionID = sessions.Items[0].ID
	}

	limit := 50
	if req.Limit > 0 && req.Limit <= 200 {
		limit = req.Limit
	}

	messages, err := agentSvc.ListMessages(ctx, &model.ListAgentMessagesReq{
		SessionID: strconv.Itoa(sessionID),
		ListReq:   model.ListReq{Page: 1, Size: limit},
	})
	if err != nil {
		return map[string]interface{}{"messages": []interface{}{}, "hasMore": false}, nil
	}

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
	var req struct {
		SessionKey string `json:"sessionKey"`
	}
	json.Unmarshal(params, &req)

	sessionKey := req.SessionKey
	if sessionKey == "" {
		sessionKey = "agent:main:global"
	}

	// Ensure a session row exists
	ensureSession(ctx, sessionKey)

	// Return empty messages and the session key; frontend will call chat.history for full load
	return map[string]interface{}{
		"messages":   []interface{}{},
		"sessionKey": sessionKey,
	}, nil
}

func handleChatMetadata(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	if agentSvc == nil {
		return map[string]interface{}{
			"commands": []interface{}{},
			"models":   []interface{}{},
		}, nil
	}

	var req struct {
		AgentID string `json:"agentId,omitempty"`
	}
	json.Unmarshal(params, &req)

	catalog, err := agentSvc.GetModelCatalog(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取模型目录失败: %w", err)
	}

	return map[string]interface{}{
		"commands": []interface{}{},
		"models":   catalog,
	}, nil
}

func handleChatMessageGet(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	if agentSvc == nil {
		return buildOKResponse(map[string]interface{}{
			"ok":               false,
			"unavailableReason": "not_found",
		}), nil
	}

	var req struct {
		SessionKey string `json:"sessionKey"`
		MessageID  string `json:"messageId"`
		MaxChars   int    `json:"maxChars,omitempty"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, fmt.Errorf("解析参数失败: %w", err)
	}

	// Parse session ID from sessionKey
	sessionID, err := ensureSession(ctx, req.SessionKey)
	if err != nil {
		return buildOKResponse(map[string]interface{}{
			"ok":               false,
			"unavailableReason": "not_found",
		}), nil
	}

	// Parse message ID
	messageID, err := strconv.ParseInt(req.MessageID, 10, 64)
	if err != nil {
		return buildOKResponse(map[string]interface{}{
			"ok":               false,
			"unavailableReason": "not_found",
		}), nil
	}

	// Get message from DAO
	msg, err := agentSvc.GetMessage(ctx, strconv.Itoa(sessionID), messageID)
	if err != nil {
		return nil, fmt.Errorf("获取消息失败: %w", err)
	}
	if msg == nil {
		return buildOKResponse(map[string]interface{}{
			"ok":               false,
			"unavailableReason": "not_found",
		}), nil
	}

	// Truncate content if MaxChars specified
	content := msg.Content
	if req.MaxChars > 0 && len(content) > req.MaxChars {
		content = content[:req.MaxChars]
	}

	return buildOKResponse(map[string]interface{}{
		"message": map[string]interface{}{
			"id":        strconv.FormatInt(msg.ID, 10),
			"role":      msg.Role,
			"content":   []map[string]string{{"type": "text", "text": content}},
			"timestamp": msg.CreatedAt.UnixMilli(),
		},
	}), nil
}
