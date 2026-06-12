package rpc

import (
	"context"
	"encoding/json"

	"github.com/rizxfrog/VanPanelBackend/internal/gateway"
)

func init() {
	gateway.RegisterMethod("environments.list", string(gateway.ScopeRead), handleEnvironmentsList)
	gateway.RegisterMethod("environments.status", string(gateway.ScopeRead), handleEnvironmentsStatus)
}

func handleEnvironmentsList(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"environments": []interface{}{}}, nil
}
func handleEnvironmentsStatus(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]string{"status": "unknown"}, nil
}
