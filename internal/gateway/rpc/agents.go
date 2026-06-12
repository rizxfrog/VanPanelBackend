package rpc

import (
	"context"
	"encoding/json"

	"github.com/rizxfrog/VanPanelBackend/internal/gateway"
)

func init() {
	gateway.RegisterMethod("agents.list", string(gateway.ScopeRead), handleAgentsList)
	gateway.RegisterMethod("agents.create", string(gateway.ScopeAdmin), handleAgentsOK)
	gateway.RegisterMethod("agents.update", string(gateway.ScopeAdmin), handleAgentsOK)
	gateway.RegisterMethod("agents.delete", string(gateway.ScopeAdmin), handleAgentsOK)
	gateway.RegisterMethod("agents.files.list", string(gateway.ScopeRead), handleAgentsFilesList)
	gateway.RegisterMethod("agents.files.get", string(gateway.ScopeRead), handleAgentsFilesGet)
	gateway.RegisterMethod("agents.files.set", string(gateway.ScopeAdmin), handleAgentsOK)
}

func handleAgentsList(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{
		"agents": []map[string]interface{}{
			{
				"id":     "main",
				"name":   "VanPanel Agent",
				"model":  "gpt-4o",
				"status": "ready",
			},
		},
	}, nil
}

func handleAgentsFilesList(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	// Parse agentId from params
	var req struct {
		AgentID string `json:"agentId"`
	}
	json.Unmarshal(params, &req)
	agentID := req.AgentID
	if agentID == "" {
		agentID = "main"
	}

	return map[string]interface{}{
		"agentId": agentID,
		"files":   []interface{}{},
	}, nil
}

func handleAgentsFilesGet(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{
		"content": "",
	}, nil
}

func handleAgentsOK(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]bool{"ok": true}, nil
}
