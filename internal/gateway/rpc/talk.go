package rpc

import (
	"context"
	"encoding/json"

	"github.com/rizxfrog/VanPanelBackend/internal/gateway"
)

func init() {
	gateway.RegisterMethod("talk.start", string(gateway.ScopeRead), handleTalkStart)
	gateway.RegisterMethod("talk.stop", string(gateway.ScopeRead), handleTalkStop)
	gateway.RegisterMethod("talk.send", string(gateway.ScopeRead), handleTalkSend)
}

func handleTalkStart(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"ok": false, "error": "voice not configured"}, nil
}

func handleTalkStop(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"ok": false, "error": "voice not configured"}, nil
}

func handleTalkSend(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"ok": false, "error": "voice not configured"}, nil
}
