package rpc

import (
	"context"
	"encoding/json"

	"github.com/rizxfrog/VanPanelBackend/internal/gateway"
)

func init() {
	gateway.RegisterMethod("health", string(gateway.ScopeRead), handleHealth)
	gateway.RegisterMethod("status", string(gateway.ScopeRead), handleStatus)
	gateway.RegisterMethod("diagnostics.stability", string(gateway.ScopeRead), handleDiagnosticsStability)
	gateway.RegisterMethod("logs.tail", string(gateway.ScopeRead), handleLogsTail)
}

func handleHealth(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{
		"status":  "healthy",
		"version": "vanpanel-0.1.0",
		"uptime":  "0h 0m",
	}, nil
}

func handleStatus(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return handleHealth(ctx, conn, params)
}

func handleDiagnosticsStability(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]bool{"stable": true}, nil
}

func handleLogsTail(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{
		"entries": []interface{}{},
		"cursor":  "",
	}, nil
}
