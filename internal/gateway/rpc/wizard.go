package rpc

import (
	"context"
	"encoding/json"

	"github.com/rizxfrog/VanPanelBackend/internal/gateway"
)

func init() {
	gateway.RegisterMethod("wizard.start", string(gateway.ScopeRead), handleWizardStart)
	gateway.RegisterMethod("wizard.next", string(gateway.ScopeRead), handleWizardNext)
	gateway.RegisterMethod("wizard.cancel", string(gateway.ScopeRead), handleWizardCancel)
	gateway.RegisterMethod("wizard.status", string(gateway.ScopeRead), handleWizardStatus)
}

func handleWizardStart(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"ok": true}, nil
}

func handleWizardNext(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"ok": true}, nil
}

func handleWizardCancel(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"ok": true}, nil
}

func handleWizardStatus(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"ok": true}, nil
}
