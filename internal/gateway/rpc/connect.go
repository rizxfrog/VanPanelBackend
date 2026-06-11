package rpc

import (
	"context"
	"encoding/json"

	"github.com/rizxfrog/VanPanelBackend/internal/gateway"
)

// All RPC method handlers are registered via init() functions in their respective files.
// The gateway.RegisterMethod() function is called with the method name, required scope,
// and handler function.

// init registers the connect method (handled at handshake level, but registered for completeness).
func init() {
	gateway.RegisterMethod("connect", string(gateway.ScopeAdmin), handleConnect)
}

func handleConnect(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]string{"status": "already_connected"}, nil
}
