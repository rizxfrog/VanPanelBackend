package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rizxfrog/VanPanelBackend/internal/gateway"
	"github.com/rizxfrog/VanPanelBackend/internal/model"
)

// subHub manages session and message subscriptions for real-time push.
var subHub *gateway.SubscriptionHub

// SetSubscriptionHub sets the SubscriptionHub for gateway session handlers.
func SetSubscriptionHub(hub *gateway.SubscriptionHub) {
	subHub = hub
}

func init() {
	gateway.RegisterMethod("sessions.list", string(gateway.ScopeRead), handleSessionsList)
	gateway.RegisterMethod("sessions.create", string(gateway.ScopeWrite), handleSessionsCreate)
	gateway.RegisterMethod("sessions.send", string(gateway.ScopeWrite), handleSessionsOK)
	gateway.RegisterMethod("sessions.abort", string(gateway.ScopeWrite), handleSessionsOK)
	gateway.RegisterMethod("sessions.delete", string(gateway.ScopeAdmin), handleSessionsDelete)
	gateway.RegisterMethod("sessions.patch", string(gateway.ScopeAdmin), handleSessionsPatch)
	gateway.RegisterMethod("sessions.reset", string(gateway.ScopeAdmin), handleSessionsReset)
	gateway.RegisterMethod("sessions.compact", string(gateway.ScopeAdmin), handleSessionsCompact)
	gateway.RegisterMethod("sessions.compaction.list", string(gateway.ScopeRead), handleSessionsCompactionList)
	gateway.RegisterMethod("sessions.compaction.get", string(gateway.ScopeRead), handleSessionsCompactionGet)
	gateway.RegisterMethod("sessions.compaction.branch", string(gateway.ScopeWrite), handleSessionsCompactionBranch)
	gateway.RegisterMethod("sessions.compaction.restore", string(gateway.ScopeAdmin), handleSessionsCompactionRestore)
	gateway.RegisterMethod("sessions.subscribe", string(gateway.ScopeRead), handleSessionsSubscribe)
	gateway.RegisterMethod("sessions.unsubscribe", string(gateway.ScopeRead), handleSessionsUnsubscribe)
	gateway.RegisterMethod("sessions.messages.subscribe", string(gateway.ScopeRead), handleSessionsMessagesSubscribe)
	gateway.RegisterMethod("sessions.messages.unsubscribe", string(gateway.ScopeRead), handleSessionsMessagesUnsubscribe)
	gateway.RegisterMethod("sessions.preview", string(gateway.ScopeRead), handleSessionsPreview)
	gateway.RegisterMethod("sessions.describe", string(gateway.ScopeRead), handleSessionsDescribe)
	gateway.RegisterMethod("sessions.cleanup", string(gateway.ScopeAdmin), handleSessionsOK)
	gateway.RegisterMethod("sessions.pluginPatch", string(gateway.ScopeAdmin), handleSessionsOK)
}

func handleSessionsList(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	if agentSvc == nil {
		// fallback: empty result when AgentService is not ready
		return map[string]interface{}{
			"ts":         time.Now().UnixMilli(),
			"path":       "sessions",
			"count":      0,
			"totalCount": 0,
			"defaults": map[string]interface{}{
				"modelProvider": nil,
				"model":         nil,
				"contextTokens": nil,
			},
			"sessions": []interface{}{},
		}, nil
	}

	var reqParams struct {
		Search        string `json:"search"`
		Limit         int    `json:"limit"`
		Offset        int    `json:"offset"`
		AgentID       string `json:"agentId"`
		ActiveMinutes int    `json:"activeMinutes"`
	}
	json.Unmarshal(params, &reqParams)

	page := 1
	size := 50
	if reqParams.Limit > 0 {
		size = reqParams.Limit
		if size > 100 {
			size = 100
		}
	}
	if reqParams.Offset > 0 {
		page = (reqParams.Offset / size) + 1
	}

	result, err := agentSvc.ListSessions(ctx, &model.ListAgentSessionsReq{
		ListReq: model.ListReq{
			Page:   page,
			Size:   size,
			Search: reqParams.Search,
		},
	}, defaultUserID)
	if err != nil {
		return nil, fmt.Errorf("查询会话列表失败: %w", err)
	}

	sessions := make([]interface{}, 0, len(result.Items))
	for _, s := range result.Items {
		sessions = append(sessions, sessionToRow(s))
	}

	return map[string]interface{}{
		"ts":         time.Now().UnixMilli(),
		"path":       "sessions",
		"count":      len(sessions),
		"totalCount": result.Total,
		"defaults": map[string]interface{}{
			"modelProvider": nil,
			"model":         nil,
			"contextTokens": nil,
		},
		"sessions": sessions,
	}, nil
}

func handleSessionsCreate(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	if agentSvc == nil {
		return nil, fmt.Errorf("AgentService 未初始化")
	}

	var reqParams struct {
		Label string `json:"label"`
		Kind  string `json:"kind"`
	}
	json.Unmarshal(params, &reqParams)

	title := reqParams.Label
	if title == "" {
		title = "新对话"
	}

	session, err := agentSvc.CreateSession(ctx, &model.CreateAgentSessionReq{
		Title: title,
	}, defaultUserID)
	if err != nil {
		return nil, fmt.Errorf("创建会话失败: %w", err)
	}

	row := sessionToRow(session)
	notifySessionChanged("agent:main:"+strconv.Itoa(session.ID), "create", row)
	return row, nil
}

func handleSessionsDelete(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	if agentSvc == nil {
		return nil, fmt.Errorf("AgentService 未初始化")
	}

	var reqParams struct {
		Key string `json:"key"`
	}
	json.Unmarshal(params, &reqParams)

	id, err := parseSessionKey(reqParams.Key)
	if err != nil {
		return nil, fmt.Errorf("解析会话 key 失败: %w", err)
	}

	if err := agentSvc.DeleteSession(ctx, id); err != nil {
		return nil, fmt.Errorf("删除会话失败: %w", err)
	}

	notifySessionChanged(reqParams.Key, "delete", nil)
	return map[string]bool{"ok": true}, nil
}

func handleSessionsPatch(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	if agentSvc == nil {
		return map[string]bool{"ok": true}, nil
	}

	var reqParams struct {
		Key      string  `json:"key"`
		Label    *string `json:"label"`
		Archived *bool   `json:"archived"`
	}
	if err := json.Unmarshal(params, &reqParams); err != nil {
		return nil, fmt.Errorf("解析参数失败: %w", err)
	}

	id, err := parseSessionKey(reqParams.Key)
	if err != nil {
		return nil, fmt.Errorf("解析会话 key 失败: %w", err)
	}

	existing, err := agentSvc.GetSession(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("获取会话失败: %w", err)
	}

	if reqParams.Label != nil {
		existing.Title = *reqParams.Label
	}
	if reqParams.Archived != nil {
		if *reqParams.Archived {
			existing.Status = model.AgentSessionStatusArchived
		} else {
			existing.Status = model.AgentSessionStatusActive
		}
	}

	if err := agentSvc.UpdateSession(ctx, existing); err != nil {
		return nil, fmt.Errorf("更新会话失败: %w", err)
	}

	row := sessionToRow(existing)
	notifySessionChanged("agent:main:"+strconv.Itoa(existing.ID), "patch", row)
	return map[string]interface{}{
		"ok":    true,
		"entry": row,
	}, nil
}

func handleSessionsSubscribe(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	if subHub == nil {
		return buildOKResponse(map[string]interface{}{
			"subscribed": false,
		}), nil
	}

	subHub.SubscribeSession(conn.ID)
	return buildOKResponse(map[string]interface{}{
		"subscribed": true,
	}), nil
}

func handleSessionsUnsubscribe(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	if subHub == nil {
		return buildOKResponse(map[string]interface{}{
			"subscribed": false,
		}), nil
	}

	subHub.UnsubscribeSession(conn.ID)
	return buildOKResponse(map[string]interface{}{
		"subscribed": false,
	}), nil
}

func handleSessionsMessagesSubscribe(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	if subHub == nil {
		return buildOKResponse(map[string]interface{}{
			"subscribed": false,
		}), nil
	}

	var req struct {
		Key     string `json:"key"`
		AgentID string `json:"agentId,omitempty"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, fmt.Errorf("解析参数失败: %w", err)
	}

	sessionKey := req.Key
	if sessionKey == "" {
		return nil, fmt.Errorf("会话 key 不能为空")
	}

	subHub.SubscribeMessages(conn.ID, sessionKey)
	return buildOKResponse(map[string]interface{}{
		"subscribed": true,
		"key":        sessionKey,
	}), nil
}

func handleSessionsMessagesUnsubscribe(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	if subHub == nil {
		return buildOKResponse(map[string]interface{}{
			"subscribed": false,
		}), nil
	}

	var req struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, fmt.Errorf("解析参数失败: %w", err)
	}

	sessionKey := req.Key
	if sessionKey == "" {
		return nil, fmt.Errorf("会话 key 不能为空")
	}

	subHub.UnsubscribeMessages(conn.ID, sessionKey)
	return buildOKResponse(map[string]interface{}{
		"subscribed": false,
		"key":        sessionKey,
	}), nil
}

func handleSessionsReset(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	if agentSvc == nil {
		return nil, fmt.Errorf("AgentService 未初始化")
	}

	var req struct {
		Key     string `json:"key"`
		AgentID string `json:"agentId,omitempty"`
		Reason  string `json:"reason,omitempty"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, fmt.Errorf("解析参数失败: %w", err)
	}

	sessionKey := req.Key
	if sessionKey == "" {
		return nil, fmt.Errorf("会话 key 不能为空")
	}

	session, err := agentSvc.ResetSession(ctx, sessionKey, defaultUserID)
	if err != nil {
		return nil, fmt.Errorf("重置会话失败: %w", err)
	}

	row := sessionToRow(session)
	notifySessionChanged(sessionKey, "reset", row)
	return buildOKResponse(map[string]interface{}{
		"key":   sessionKey,
		"entry": row,
	}), nil
}

func handleSessionsCompact(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	if agentSvc == nil {
		return nil, fmt.Errorf("AgentService 未初始化")
	}

	var req struct {
		Key      string `json:"key"`
		AgentID  string `json:"agentId,omitempty"`
		MaxLines int    `json:"maxLines,omitempty"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, fmt.Errorf("解析参数失败: %w", err)
	}

	sessionKey := req.Key
	if sessionKey == "" {
		return nil, fmt.Errorf("会话 key 不能为空")
	}

	maxMessages := req.MaxLines
	if maxMessages <= 0 {
		maxMessages = 50
	}

	if err := agentSvc.CompactSession(ctx, sessionKey, maxMessages, defaultUserID); err != nil {
		return nil, fmt.Errorf("压缩会话失败: %w", err)
	}

	notifySessionChanged(sessionKey, "compact", nil)
	return buildOKResponse(map[string]interface{}{
		"key":       sessionKey,
		"compacted": true,
	}), nil
}

// --- Compaction Handlers ---

func handleSessionsCompactionList(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	var req struct {
		Key string `json:"key"`
	}
	json.Unmarshal(params, &req)

	if req.Key == "" {
		return []interface{}{}, nil
	}

	compactionMu.Lock()
	defer compactionMu.Unlock()

	branches := compactionStore[req.Key]
	result := make([]interface{}, 0, len(branches))
	for _, b := range branches {
		result = append(result, map[string]interface{}{
			"id":           b.ID,
			"sessionKey":   b.SessionKey,
			"label":        b.Label,
			"messageCount": b.MessageCount,
			"tokenCount":   b.TokenCount,
			"createdAt":    b.CreatedAt,
			"summary":      b.Summary,
		})
	}
	return result, nil
}

func handleSessionsCompactionGet(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	var req struct {
		Key      string `json:"key"`
		BranchID string `json:"branchId"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, fmt.Errorf("解析参数失败: %w", err)
	}

	compactionMu.Lock()
	defer compactionMu.Unlock()

	for _, b := range compactionStore[req.Key] {
		if b.ID == req.BranchID {
			return map[string]interface{}{
				"id":           b.ID,
				"sessionKey":   b.SessionKey,
				"label":        b.Label,
				"messageCount": b.MessageCount,
				"tokenCount":   b.TokenCount,
				"createdAt":    b.CreatedAt,
				"summary":      b.Summary,
			}, nil
		}
	}
	return nil, nil
}

func handleSessionsCompactionBranch(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	var req struct {
		Key   string `json:"key"`
		Label string `json:"label,omitempty"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, fmt.Errorf("解析参数失败: %w", err)
	}
	if req.Key == "" {
		return nil, fmt.Errorf("会话 key 不能为空")
	}

	// Count current messages for this session
	messageCount := 0
	if agentSvc != nil {
		id, err := parseSessionKey(req.Key)
		if err == nil {
			messages, listErr := agentSvc.ListMessages(ctx, &model.ListAgentMessagesReq{
				SessionID: strconv.Itoa(id),
				ListReq:   model.ListReq{Page: 1, Size: 1},
			})
			if listErr == nil {
				messageCount = int(messages.Total)
			}
		}
	}

	label := req.Label
	if label == "" {
		label = fmt.Sprintf("快照 %s", time.Now().Format("2006-01-02 15:04:05"))
	}

	branchID := fmt.Sprintf("branch-%d", time.Now().UnixNano())
	branch := compactionBranch{
		ID:           branchID,
		SessionKey:   req.Key,
		Label:        label,
		MessageCount: messageCount,
		CreatedAt:    time.Now().UnixMilli(),
	}

	compactionMu.Lock()
	compactionStore[req.Key] = append(compactionStore[req.Key], branch)
	compactionMu.Unlock()

	return map[string]interface{}{
		"ok":       true,
		"branchId": branchID,
	}, nil
}

func handleSessionsCompactionRestore(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	var req struct {
		Key      string `json:"key"`
		BranchID string `json:"branchId"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, fmt.Errorf("解析参数失败: %w", err)
	}
	if req.Key == "" || req.BranchID == "" {
		return nil, fmt.Errorf("key 和 branchId 不能为空")
	}

	compactionMu.Lock()
	branches := compactionStore[req.Key]
	found := false
	for _, b := range branches {
		if b.ID == req.BranchID {
			found = true
			break
		}
	}
	compactionMu.Unlock()

	if !found {
		return nil, fmt.Errorf("压缩分支 %s 不存在", req.BranchID)
	}

	if err := requireAgentSvc(); err != nil {
		return nil, err
	}
	session, err := agentSvc.ResetSession(ctx, req.Key, defaultUserID)
	if err != nil {
		return nil, fmt.Errorf("恢复会话失败: %w", err)
	}

	notifySessionChanged(req.Key, "restore", sessionToRow(session))
	return map[string]interface{}{
		"ok":        true,
		"sessionId": strconv.Itoa(session.ID),
	}, nil
}

// --- Preview / Describe Handlers ---

func handleSessionsPreview(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	var req struct {
		Key     string `json:"key"`
		AgentID string `json:"agentId,omitempty"`
	}
	json.Unmarshal(params, &req)

	if req.Key == "" || agentSvc == nil {
		return nil, nil
	}

	id, err := parseSessionKey(req.Key)
	if err != nil {
		return nil, nil
	}

	session, err := agentSvc.GetSession(ctx, id)
	if err != nil {
		return nil, nil
	}

	messages, err := agentSvc.ListMessages(ctx, &model.ListAgentMessagesReq{
		SessionID: strconv.Itoa(id),
		ListReq:   model.ListReq{Page: 1, Size: 3},
	})
	if err != nil || len(messages.Items) == 0 {
		return nil, nil
	}

	var previews []map[string]interface{}
	for i := len(messages.Items) - 1; i >= 0; i-- {
		msg := messages.Items[i]
		previews = append(previews, map[string]interface{}{
			"role":    msg.Role,
			"content": truncateString(msg.Content, 200),
		})
	}

	return map[string]interface{}{
		"key":      req.Key,
		"label":    session.Title,
		"messages": previews,
		"status":   session.Status,
	}, nil
}

func handleSessionsDescribe(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	var req struct {
		Key string `json:"key"`
	}
	json.Unmarshal(params, &req)

	if req.Key == "" || agentSvc == nil {
		return nil, nil
	}

	id, err := parseSessionKey(req.Key)
	if err != nil {
		return nil, nil
	}

	session, err := agentSvc.GetSession(ctx, id)
	if err != nil {
		return nil, nil
	}

	messages, err := agentSvc.ListMessages(ctx, &model.ListAgentMessagesReq{
		SessionID: strconv.Itoa(id),
		ListReq:   model.ListReq{Page: 1, Size: 1},
	})
	if err != nil {
		return nil, nil
	}

	msgCount := messages.Total

	return map[string]interface{}{
		"key":          req.Key,
		"label":        session.Title,
		"status":       session.Status,
		"messageCount": msgCount,
		"createdAt":    session.CreatedAt.UnixMilli(),
		"updatedAt":    session.UpdatedAt.UnixMilli(),
		"modelName":    session.ModelName,
	}, nil
}

func handleSessionsOK(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]bool{"ok": true}, nil
}

func handleSessionsEmptyArray(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return []interface{}{}, nil
}

func handleSessionsNull(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return nil, nil
}

// sessionToRow converts an AgentSession to a GatewaySessionRow map.
func sessionToRow(session *model.AgentSession) map[string]interface{} {
	idStr := strconv.Itoa(session.ID)
	key := "agent:main:" + idStr

	row := map[string]interface{}{
		"key":         key,
		"kind":        "direct",
		"label":       session.Title,
		"displayName": session.Title,
		"sessionId":   idStr,
		"model":       session.ModelName,
		"updatedAt":   session.UpdatedAt.UnixMilli(),
	}

	if session.Status == model.AgentSessionStatusArchived {
		row["archived"] = true
	}

	return row
}

// parseSessionKey extracts the numeric session ID from a key like "agent:main:42".
func parseSessionKey(key string) (int, error) {
	parts := strings.Split(key, ":")
	if len(parts) < 3 {
		return 0, fmt.Errorf("无效的会话 key 格式: %s", key)
	}
	id, err := strconv.Atoi(parts[2])
	if err != nil {
		return 0, fmt.Errorf("无效的会话 ID: %s", parts[2])
	}
	if id <= 0 {
		return 0, fmt.Errorf("无效的会话 ID: %d", id)
	}
	return id, nil
}

// --- Event Broadcasting ---

// notifySessionChanged broadcasts a sessions.changed event to all connections
// subscribed to session list updates.
func notifySessionChanged(key, reason string, session map[string]interface{}) {
	if broadcastMgr == nil || subHub == nil {
		return
	}
	payload := map[string]interface{}{
		"key":    key,
		"reason": reason,
	}
	if session != nil {
		payload["session"] = session
	}
	for _, connID := range subHub.GetSessionSubscribers() {
		broadcastMgr.BroadcastTo(connID, "sessions.changed", payload)
	}
}

// notifySessionMessage broadcasts a chat event to all connections subscribed
// to message events for a given session (excluding the originating connection).
func notifySessionMessage(sessionKey string, connID string, event gateway.ChatEvent) {
	if broadcastMgr == nil || subHub == nil {
		return
	}
	for _, subConnID := range subHub.GetMessageSubscribers(sessionKey) {
		if subConnID == connID {
			continue // skip the originating connection
		}
		broadcastMgr.BroadcastTo(subConnID, "chat", event)
	}
}

// --- Compaction Store ---

// compactionStore holds session compaction branches in memory.
// map[sessionKey] → []branch
var compactionStore = make(map[string][]compactionBranch)
var compactionMu sync.Mutex

type compactionBranch struct {
	ID           string `json:"id"`
	SessionKey   string `json:"sessionKey"`
	Label        string `json:"label"`
	MessageCount int    `json:"messageCount"`
	TokenCount   int    `json:"tokenCount,omitempty"`
	CreatedAt    int64  `json:"createdAt"`
	Summary      string `json:"summary,omitempty"`
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
