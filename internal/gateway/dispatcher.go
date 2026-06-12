package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"

	"go.uber.org/zap"
)

// RPCFunc is the handler function signature for registered RPC methods.
type RPCFunc func(ctx context.Context, conn *GatewayConnection, params json.RawMessage) (interface{}, error)

// MethodSpec defines a registered RPC method with its required scope.
type MethodSpec struct {
	Name          string
	RequiredScope string
	Handler       RPCFunc
}

// MethodRegistry manages registered RPC methods in a thread-safe map.
type MethodRegistry struct {
	mu      sync.RWMutex
	methods map[string]*MethodSpec
}

// registry is the package-level singleton used by init() functions in rpc/.
var registry = &MethodRegistry{
	methods: make(map[string]*MethodSpec),
}

// RegisterMethod registers an RPC method for dispatch (thread-safe).
// Intended to be called from init() functions in the rpc/ subdirectory.
func RegisterMethod(name, scope string, handler RPCFunc) {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	registry.methods[name] = &MethodSpec{
		Name:          name,
		RequiredScope: scope,
		Handler:       handler,
	}
}

// GetRegisteredMethods returns the names of all registered methods, sorted.
func GetRegisteredMethods() []string {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	names := make([]string, 0, len(registry.methods))
	for name := range registry.methods {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// GatewayEvents lists all possible event types pushed by the server.
var GatewayEventList = []string{
	"tick", "chat", "presence", "health",
	"shutdown", "connect.challenge",
	"session.tool", "sessions.changed",
	"agent",
	"device.pair.requested", "device.pair.resolved",
	"node.pair.requested", "node.pair.resolved",
	"exec.approval.requested", "exec.approval.resolved",
	"plugin.approval.requested", "plugin.approval.resolved",
	"voicewake.changed", "voicewake.routing.changed",
	"update.available", "talk.mode", "talk.event",
	"heartbeat",
}

// GetRegisteredEvents returns the list of all event types supported by the gateway.
func GetRegisteredEvents() []string {
	return GatewayEventList
}

// Dispatch routes an incoming request frame to the matching registered method.
//
// Steps:
//  1. Look up method in registry.
//  2. If not found, send an error response.
//  3. Check scope (skipped for "connect" which is handled during handshake).
//  4. If scope check fails, send an error response.
//  5. Marshal params to json.RawMessage.
//  6. Call handler with the connection context and parsed params.
//  7. Send the handler result or error back to the client.
//
// Each dispatch is logged at Debug level.
func Dispatch(conn *GatewayConnection, req *RequestFrame) {
	registry.mu.RLock()
	method, ok := registry.methods[req.Method]
	registry.mu.RUnlock()

	if !ok {
		conn.logger.Debug("method not found",
			zap.String("connID", conn.ID),
			zap.String("method", req.Method),
		)
		conn.SendError(req.ID, ErrCodeInvalidRequest, fmt.Sprintf("method not found: %s", req.Method))
		return
	}

	// "connect" scope check is performed during the handshake phase
	if req.Method != "connect" {
		if !conn.HasScope(method.RequiredScope) {
			conn.logger.Debug("insufficient permissions",
				zap.String("connID", conn.ID),
				zap.String("method", req.Method),
				zap.String("required", method.RequiredScope),
				zap.Strings("scopes", conn.Scopes),
			)
			conn.SendError(req.ID, "INSUFFICIENT_PERMISSIONS", "insufficient permissions")
			return
		}
	}

	var rawParams json.RawMessage
	if req.Params != nil {
		data, err := json.Marshal(req.Params)
		if err != nil {
			conn.logger.Debug("failed to marshal params",
				zap.String("connID", conn.ID),
				zap.String("method", req.Method),
				zap.Error(err),
			)
			conn.SendError(req.ID, ErrCodeInvalidRequest, fmt.Sprintf("invalid params: %v", err))
			return
		}
		rawParams = data
	}

	conn.logger.Debug("dispatching method",
		zap.String("connID", conn.ID),
		zap.String("method", req.Method),
	)

	result, err := method.Handler(conn.Context(), conn, rawParams)
	if err != nil {
		conn.logger.Debug("method handler error",
			zap.String("connID", conn.ID),
			zap.String("method", req.Method),
			zap.Error(err),
		)
		conn.SendError(req.ID, "RPC_ERROR", err.Error())
		return
	}

	conn.SendResponse(req.ID, result)
}
