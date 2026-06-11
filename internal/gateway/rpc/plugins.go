package rpc

import (
	"context"
	"encoding/json"

	"github.com/rizxfrog/VanPanelBackend/internal/gateway"
)

func init() {
	gateway.RegisterMethod("plugins.uiDescriptors", string(gateway.ScopeRead), handlePluginsUIDescriptors)
	gateway.RegisterMethod("plugins.sessionAction", string(gateway.ScopeAdmin), handlePluginsSessionAction)
}

func handlePluginsUIDescriptors(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"actions": []interface{}{}}, nil
}
func handlePluginsSessionAction(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]bool{"ok": true}, nil
}
