package rpc

import (
	"context"
	"encoding/json"

	"github.com/rizxfrog/VanPanelBackend/internal/gateway"
)

func init() {
	gateway.RegisterMethod("cron.status", string(gateway.ScopeRead), handleCronStatus)
	gateway.RegisterMethod("cron.list", string(gateway.ScopeRead), handleCronList)
	gateway.RegisterMethod("cron.get", string(gateway.ScopeRead), handleCronGet)
	gateway.RegisterMethod("cron.add", string(gateway.ScopeRead), handleCronAdd)
	gateway.RegisterMethod("cron.update", string(gateway.ScopeRead), handleCronUpdate)
	gateway.RegisterMethod("cron.remove", string(gateway.ScopeRead), handleCronRemove)
	gateway.RegisterMethod("cron.run", string(gateway.ScopeRead), handleCronRun)
	gateway.RegisterMethod("cron.runs", string(gateway.ScopeRead), handleCronRuns)
}

func handleCronStatus(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"running": false}, nil
}

func handleCronList(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"jobs": []interface{}{}}, nil
}

func handleCronGet(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"job": nil}, nil
}

func handleCronAdd(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"ok": true}, nil
}

func handleCronUpdate(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"ok": true}, nil
}

func handleCronRemove(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"ok": true}, nil
}

func handleCronRun(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"ok": true}, nil
}

func handleCronRuns(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"runs": []interface{}{}}, nil
}
