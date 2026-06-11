package rpc

import (
	"context"
	"encoding/json"

	"github.com/rizxfrog/VanPanelBackend/internal/gateway"
)

func init() {
	gateway.RegisterMethod("node.list", string(gateway.ScopeRead), handleNodeList)
	gateway.RegisterMethod("node.describe", string(gateway.ScopeRead), handleNodeDescribe)
	gateway.RegisterMethod("node.invoke", string(gateway.ScopeRead), handleNodeInvoke)
	gateway.RegisterMethod("node.pair.start", string(gateway.ScopePairing), handleNodePairStart)
	gateway.RegisterMethod("node.pair.stop", string(gateway.ScopePairing), handleNodePairStop)
	gateway.RegisterMethod("node.pair.status", string(gateway.ScopePairing), handleNodePairStatus)
	gateway.RegisterMethod("node.pending.list", string(gateway.ScopeRead), handleNodePendingList)
	gateway.RegisterMethod("node.pending.approve", string(gateway.ScopeRead), handleNodePendingApprove)
}

func handleNodeList(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"nodes": []interface{}{}}, nil
}

func handleNodeDescribe(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"node": nil}, nil
}

func handleNodeInvoke(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"ok": true}, nil
}

func handleNodePairStart(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"ok": true}, nil
}

func handleNodePairStop(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"ok": true}, nil
}

func handleNodePairStatus(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"ok": true}, nil
}

func handleNodePendingList(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"pending": []interface{}{}}, nil
}

func handleNodePendingApprove(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"ok": true}, nil
}
