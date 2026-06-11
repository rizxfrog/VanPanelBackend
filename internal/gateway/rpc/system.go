package rpc

import (
	"context"
	"encoding/json"

	"github.com/rizxfrog/VanPanelBackend/internal/gateway"
)

func init() {
	gateway.RegisterMethod("system-presence", string(gateway.ScopeRead), handleSystemPresence)
	gateway.RegisterMethod("system-event", string(gateway.ScopeRead), handleSystemEvent)
	gateway.RegisterMethod("last-heartbeat", string(gateway.ScopeRead), handleLastHeartbeat)
	gateway.RegisterMethod("set-heartbeats", string(gateway.ScopeRead), handleSetHeartbeats)
	gateway.RegisterMethod("update.status", string(gateway.ScopeRead), handleUpdateStatus)
	gateway.RegisterMethod("update.run", string(gateway.ScopeRead), handleUpdateRun)
	gateway.RegisterMethod("gateway.identity.get", string(gateway.ScopeRead), handleGatewayIdentityGet)
	gateway.RegisterMethod("gateway.restart.preflight", string(gateway.ScopeRead), handleGatewayRestartPreflight)
	gateway.RegisterMethod("gateway.restart.request", string(gateway.ScopeRead), handleGatewayRestartRequest)
	gateway.RegisterMethod("secrets.reload", string(gateway.ScopeRead), handleSecretsReload)
	gateway.RegisterMethod("secrets.resolve", string(gateway.ScopeRead), handleSecretsResolve)
}

func handleSystemPresence(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"presence": []interface{}{}}, nil
}

func handleSystemEvent(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"ok": true}, nil
}

func handleLastHeartbeat(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{}, nil
}

func handleSetHeartbeats(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"ok": true}, nil
}

func handleUpdateStatus(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"status": "up-to-date", "currentVersion": "0.1.0"}, nil
}

func handleUpdateRun(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"ok": true}, nil
}

func handleGatewayIdentityGet(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"name": "vanpanel-gateway"}, nil
}

func handleGatewayRestartPreflight(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"ready": true}, nil
}

func handleGatewayRestartRequest(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"ok": true}, nil
}

func handleSecretsReload(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"ok": true}, nil
}

func handleSecretsResolve(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"ok": true}, nil
}
