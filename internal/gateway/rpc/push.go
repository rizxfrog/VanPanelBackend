package rpc

import (
	"context"
	"encoding/json"

	"github.com/rizxfrog/VanPanelBackend/internal/gateway"
)

func init() {
	gateway.RegisterMethod("push.web.vapidPublicKey", string(gateway.ScopeRead), handlePushWebVapidPublicKey)
	gateway.RegisterMethod("push.test", string(gateway.ScopeAdmin), handlePushTest)
	gateway.RegisterMethod("push.web.subscribe", string(gateway.ScopeAdmin), handlePushWebSubscribe)
	gateway.RegisterMethod("push.web.unsubscribe", string(gateway.ScopeAdmin), handlePushWebUnsubscribe)
	gateway.RegisterMethod("push.web.test", string(gateway.ScopeAdmin), handlePushWebTest)
}

func handlePushTest(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]bool{"ok": true}, nil
}
func handlePushWebVapidPublicKey(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]string{"publicKey": ""}, nil
}
func handlePushWebSubscribe(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]bool{"ok": true}, nil
}
func handlePushWebUnsubscribe(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]bool{"ok": true}, nil
}
func handlePushWebTest(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]bool{"ok": true}, nil
}
