package rpc

import (
	"context"
	"encoding/json"

	"github.com/rizxfrog/VanPanelBackend/internal/gateway"
)

func init() {
	gateway.RegisterMethod("message.action", string(gateway.ScopeRead), handleMessageAction)
	gateway.RegisterMethod("send", string(gateway.ScopeRead), handleSend)
	gateway.RegisterMethod("agent", string(gateway.ScopeRead), handleAgent)
	gateway.RegisterMethod("agent.identity.get", string(gateway.ScopeRead), handleAgentIdentityGet)
	gateway.RegisterMethod("agent.wait", string(gateway.ScopeRead), handleAgentWait)
	gateway.RegisterMethod("wake", string(gateway.ScopeRead), handleWake)
}

func handleMessageAction(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]bool{"ok": true}, nil
}
func handleSend(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]bool{"ok": true}, nil
}
func handleAgent(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]bool{"ok": true}, nil
}
func handleAgentIdentityGet(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	var req struct {
		AgentID string `json:"agentId"`
	}
	json.Unmarshal(params, &req)
	agentID := req.AgentID
	if agentID == "" {
		agentID = "main"
	}

	name := agentID
	if agentSvc != nil {
		agent, err := agentSvc.GetAgent(ctx, agentID)
		if err == nil && agent != nil {
			name = agent.Name
		}
	}

	return map[string]interface{}{
		"agentId": agentID,
		"name":    name,
	}, nil
}
func handleAgentWait(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{}, nil
}
func handleWake(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]bool{"ok": true}, nil
}
