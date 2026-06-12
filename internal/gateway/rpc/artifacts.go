package rpc

import (
	"context"
	"encoding/json"

	"github.com/rizxfrog/VanPanelBackend/internal/gateway"
)

func init() {
	gateway.RegisterMethod("artifacts.list", string(gateway.ScopeRead), handleArtifactsList)
	gateway.RegisterMethod("artifacts.get", string(gateway.ScopeRead), handleArtifactsGet)
	gateway.RegisterMethod("artifacts.download", string(gateway.ScopeRead), handleArtifactsDownload)
}

func handleArtifactsList(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"artifacts": []interface{}{}}, nil
}
func handleArtifactsGet(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"artifact": nil}, nil
}
func handleArtifactsDownload(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]string{"url": ""}, nil
}
