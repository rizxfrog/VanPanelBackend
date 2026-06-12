package rpc

import (
	"context"
	"encoding/json"

	"github.com/rizxfrog/VanPanelBackend/internal/gateway"
)

func init() {
	gateway.RegisterMethod("usage.status", string(gateway.ScopeRead), handleUsageStatus)
	gateway.RegisterMethod("usage.cost", string(gateway.ScopeRead), handleUsageCost)
}

func handleUsageStatus(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]float64{"totalTokens": 0, "totalCost": 0}, nil
}
func handleUsageCost(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]float64{"cost": 0}, nil
}
