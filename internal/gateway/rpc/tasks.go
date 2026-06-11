package rpc

import (
	"context"
	"encoding/json"

	"github.com/rizxfrog/VanPanelBackend/internal/gateway"
)

func init() {
	gateway.RegisterMethod("tasks.list", string(gateway.ScopeRead), handleTasksList)
	gateway.RegisterMethod("tasks.get", string(gateway.ScopeRead), handleTasksGet)
	gateway.RegisterMethod("tasks.cancel", string(gateway.ScopeAdmin), handleTasksCancel)
}

func handleTasksList(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"tasks": []interface{}{}}, nil
}
func handleTasksGet(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"task": nil}, nil
}
func handleTasksCancel(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]bool{"ok": true}, nil
}
