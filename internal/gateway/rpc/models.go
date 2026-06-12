package rpc

import (
	"context"
	"encoding/json"

	"github.com/rizxfrog/VanPanelBackend/internal/gateway"
)

func init() {
	gateway.RegisterMethod("models.list", string(gateway.ScopeRead), handleModelsList)
	gateway.RegisterMethod("models.authStatus", string(gateway.ScopeRead), handleModelsAuthStatus)
	gateway.RegisterMethod("models.authLogout", string(gateway.ScopeAdmin), handleModelsOK)
}

func handleModelsList(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{
		"providers": map[string]interface{}{
			"openai": map[string]interface{}{
				"models": []interface{}{
					map[string]interface{}{
						"id":            "gpt-4o",
						"name":          "GPT-4o",
						"contextWindow": 128000,
					},
				},
			},
		},
	}, nil
}

func handleModelsAuthStatus(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{
		"openai": map[string]string{"status": "active"},
	}, nil
}

func handleModelsOK(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]bool{"ok": true}, nil
}
