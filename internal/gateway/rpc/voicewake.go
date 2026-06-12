package rpc

import (
	"context"
	"encoding/json"

	"github.com/rizxfrog/VanPanelBackend/internal/gateway"
)

func init() {
	gateway.RegisterMethod("voicewake.get", string(gateway.ScopeRead), handleVoiceWakeGet)
	gateway.RegisterMethod("voicewake.routing.get", string(gateway.ScopeRead), handleVoiceWakeRoutingGet)
	gateway.RegisterMethod("voicewake.set", string(gateway.ScopeAdmin), handleVoiceWakeSet)
	gateway.RegisterMethod("voicewake.routing.set", string(gateway.ScopeAdmin), handleVoiceWakeRoutingSet)
}

func handleVoiceWakeGet(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"triggers": []interface{}{}}, nil
}
func handleVoiceWakeRoutingGet(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"routing": map[string]interface{}{}}, nil
}
func handleVoiceWakeSet(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]bool{"ok": true}, nil
}
func handleVoiceWakeRoutingSet(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]bool{"ok": true}, nil
}
