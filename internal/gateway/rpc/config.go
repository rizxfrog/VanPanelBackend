package rpc

import (
	"context"
	"encoding/json"

	"github.com/rizxfrog/VanPanelBackend/internal/gateway"
)

func init() {
	gateway.RegisterMethod("config.get", string(gateway.ScopeRead), handleConfigGet)
	gateway.RegisterMethod("config.set", string(gateway.ScopeAdmin), handleConfigOK)
	gateway.RegisterMethod("config.apply", string(gateway.ScopeAdmin), handleConfigOK)
	gateway.RegisterMethod("config.patch", string(gateway.ScopeAdmin), handleConfigOK)
	gateway.RegisterMethod("config.schema", string(gateway.ScopeAdmin), handleConfigSchema)
	gateway.RegisterMethod("config.schema.lookup", string(gateway.ScopeRead), handleConfigSchemaLookup)
	gateway.RegisterMethod("config.openFile", string(gateway.ScopeAdmin), handleConfigOK)
}

func handleConfigGet(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]string{
		"raw": "# VanPanel Config\nagent:\n  llm:\n    model: gpt-4o\n    provider: openai\n",
	}, nil
}

func handleConfigSchema(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{
		"schema":      map[string]interface{}{},
		"uiHints":     map[string]interface{}{},
		"version":     "1",
		"generatedAt": "",
	}, nil
}

func handleConfigSchemaLookup(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return []interface{}{}, nil
}

func handleConfigOK(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]bool{"ok": true}, nil
}
