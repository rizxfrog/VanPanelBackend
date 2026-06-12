package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/rizxfrog/VanPanelBackend/internal/gateway"
	"github.com/rizxfrog/VanPanelBackend/internal/model"
)

func init() {
	gateway.RegisterMethod("sessions.list", string(gateway.ScopeRead), handleSessionsList)
	gateway.RegisterMethod("sessions.create", string(gateway.ScopeWrite), handleSessionsCreate)
	gateway.RegisterMethod("sessions.send", string(gateway.ScopeWrite), handleSessionsOK)
	gateway.RegisterMethod("sessions.abort", string(gateway.ScopeWrite), handleSessionsOK)
	gateway.RegisterMethod("sessions.delete", string(gateway.ScopeAdmin), handleSessionsDelete)
	gateway.RegisterMethod("sessions.patch", string(gateway.ScopeAdmin), handleSessionsPatch)
	gateway.RegisterMethod("sessions.reset", string(gateway.ScopeAdmin), handleSessionsOK)
	gateway.RegisterMethod("sessions.compact", string(gateway.ScopeAdmin), handleSessionsOK)
	gateway.RegisterMethod("sessions.compaction.list", string(gateway.ScopeRead), handleSessionsEmptyArray)
	gateway.RegisterMethod("sessions.compaction.get", string(gateway.ScopeRead), handleSessionsNull)
	gateway.RegisterMethod("sessions.compaction.branch", string(gateway.ScopeWrite), handleSessionsOK)
	gateway.RegisterMethod("sessions.compaction.restore", string(gateway.ScopeAdmin), handleSessionsOK)
	gateway.RegisterMethod("sessions.subscribe", string(gateway.ScopeRead), handleSessionsOK)
	gateway.RegisterMethod("sessions.unsubscribe", string(gateway.ScopeRead), handleSessionsOK)
	gateway.RegisterMethod("sessions.messages.subscribe", string(gateway.ScopeRead), handleSessionsOK)
	gateway.RegisterMethod("sessions.messages.unsubscribe", string(gateway.ScopeRead), handleSessionsOK)
	gateway.RegisterMethod("sessions.preview", string(gateway.ScopeRead), handleSessionsNull)
	gateway.RegisterMethod("sessions.describe", string(gateway.ScopeRead), handleSessionsNull)
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

	return sessionToRow(session), nil
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

	return map[string]interface{}{
		"ok":    true,
		"entry": sessionToRow(existing),
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
