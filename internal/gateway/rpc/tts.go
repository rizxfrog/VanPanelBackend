package rpc

import (
	"context"
	"encoding/json"

	"github.com/rizxfrog/VanPanelBackend/internal/gateway"
)

func init() {
	gateway.RegisterMethod("tts.status", string(gateway.ScopeRead), handleTTSStatus)
	gateway.RegisterMethod("tts.providers", string(gateway.ScopeRead), handleTTSProviders)
	gateway.RegisterMethod("tts.personas", string(gateway.ScopeRead), handleTTSPersonas)
	gateway.RegisterMethod("tts.enable", string(gateway.ScopeAdmin), handleTTSEnable)
	gateway.RegisterMethod("tts.disable", string(gateway.ScopeAdmin), handleTTSDisable)
	gateway.RegisterMethod("tts.convert", string(gateway.ScopeAdmin), handleTTSConvert)
	gateway.RegisterMethod("tts.setProvider", string(gateway.ScopeAdmin), handleTTSSetProvider)
	gateway.RegisterMethod("tts.setPersona", string(gateway.ScopeAdmin), handleTTSSetPersona)
}

func handleTTSStatus(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]bool{"enabled": false}, nil
}
func handleTTSProviders(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"providers": []interface{}{}}, nil
}
func handleTTSPersonas(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"personas": []interface{}{}}, nil
}
func handleTTSEnable(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]bool{"ok": true}, nil
}
func handleTTSDisable(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]bool{"ok": true}, nil
}
func handleTTSConvert(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]bool{"ok": true}, nil
}
func handleTTSSetProvider(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]bool{"ok": true}, nil
}
func handleTTSSetPersona(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]bool{"ok": true}, nil
}
