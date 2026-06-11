package rpc

import (
	"context"
	"encoding/json"

	"github.com/rizxfrog/VanPanelBackend/internal/gateway"
)

func init() {
	gateway.RegisterMethod("device.pair.start", string(gateway.ScopePairing), handleDevicePairStart)
	gateway.RegisterMethod("device.pair.stop", string(gateway.ScopePairing), handleDevicePairStop)
	gateway.RegisterMethod("device.pair.status", string(gateway.ScopePairing), handleDevicePairStatus)
	gateway.RegisterMethod("device.token.create", string(gateway.ScopePairing), handleDeviceTokenCreate)
	gateway.RegisterMethod("device.token.revoke", string(gateway.ScopePairing), handleDeviceTokenRevoke)
	gateway.RegisterMethod("device.token.list", string(gateway.ScopePairing), handleDeviceTokenList)
}

func handleDevicePairStart(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"ok": true}, nil
}

func handleDevicePairStop(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"ok": true}, nil
}

func handleDevicePairStatus(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"ok": true}, nil
}

func handleDeviceTokenCreate(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"ok": true}, nil
}

func handleDeviceTokenRevoke(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"ok": true}, nil
}

func handleDeviceTokenList(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"tokens": []interface{}{}}, nil
}
