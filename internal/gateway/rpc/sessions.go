package rpc

import (
	"context"
	"encoding/json"
	"time"

	"github.com/rizxfrog/VanPanelBackend/internal/gateway"
)

func init() {
	gateway.RegisterMethod("sessions.list", string(gateway.ScopeRead), handleSessionsList)
	gateway.RegisterMethod("sessions.create", string(gateway.ScopeWrite), handleSessionsCreate)
	gateway.RegisterMethod("sessions.send", string(gateway.ScopeWrite), handleSessionsOK)
	gateway.RegisterMethod("sessions.abort", string(gateway.ScopeWrite), handleSessionsOK)
	gateway.RegisterMethod("sessions.delete", string(gateway.ScopeAdmin), handleSessionsOK)
	gateway.RegisterMethod("sessions.patch", string(gateway.ScopeAdmin), handleSessionsOK)
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
	now := time.Now().UnixMilli()
	return map[string]interface{}{
		"sessions": []interface{}{
			map[string]interface{}{
				"key":          "agent:main:global",
				"agentId":      "main",
				"label":        "Default",
				"createdAt":    now,
				"updatedAt":    now,
				"messageCount": 0,
			},
		},
		"total": 1,
	}, nil
}

func handleSessionsCreate(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	now := time.Now().UnixMilli()
	return map[string]interface{}{
		"key":       "agent:main:global",
		"agentId":   "main",
		"label":     "Default",
		"createdAt": now,
		"updatedAt": now,
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
