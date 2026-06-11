package rpc

import (
	"context"
	"encoding/json"

	"github.com/rizxfrog/VanPanelBackend/internal/gateway"
)

func init() {
	gateway.RegisterMethod("channels.status", string(gateway.ScopeRead), handleChannelsStatus)
	gateway.RegisterMethod("channels.start", string(gateway.ScopeRead), handleChannelsStart)
	gateway.RegisterMethod("channels.stop", string(gateway.ScopeRead), handleChannelsStop)
	gateway.RegisterMethod("channels.logout", string(gateway.ScopeRead), handleChannelsLogout)
}

func handleChannelsStatus(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"channels": []interface{}{}, "globalState": "unconfigured"}, nil
}

func handleChannelsStart(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"ok": false, "error": "not configured"}, nil
}

func handleChannelsStop(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"ok": false, "error": "not configured"}, nil
}

func handleChannelsLogout(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"ok": false, "error": "not configured"}, nil
}
