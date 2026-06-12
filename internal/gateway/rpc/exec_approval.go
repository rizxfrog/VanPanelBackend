package rpc

import (
	"context"
	"encoding/json"

	"github.com/rizxfrog/VanPanelBackend/internal/gateway"
)

func init() {
	gateway.RegisterMethod("exec.approvals.get", string(gateway.ScopeApprovals), handleExecApprovalsGet)
	gateway.RegisterMethod("exec.approvals.set", string(gateway.ScopeApprovals), handleExecApprovalsSet)
	gateway.RegisterMethod("exec.approvals.node.get", string(gateway.ScopeApprovals), handleExecApprovalsNodeGet)
	gateway.RegisterMethod("exec.approvals.node.set", string(gateway.ScopeApprovals), handleExecApprovalsNodeSet)
	gateway.RegisterMethod("exec.approval.get", string(gateway.ScopeApprovals), handleExecApprovalGet)
	gateway.RegisterMethod("exec.approval.list", string(gateway.ScopeApprovals), handleExecApprovalList)
	gateway.RegisterMethod("exec.approval.request", string(gateway.ScopeApprovals), handleExecApprovalRequest)
	gateway.RegisterMethod("exec.approval.waitDecision", string(gateway.ScopeApprovals), handleExecApprovalWaitDecision)
	gateway.RegisterMethod("exec.approval.resolve", string(gateway.ScopeApprovals), handleExecApprovalResolve)
	gateway.RegisterMethod("plugin.approval.list", string(gateway.ScopeApprovals), handlePluginApprovalList)
	gateway.RegisterMethod("plugin.approval.request", string(gateway.ScopeApprovals), handlePluginApprovalRequest)
	gateway.RegisterMethod("plugin.approval.waitDecision", string(gateway.ScopeApprovals), handlePluginApprovalWaitDecision)
	gateway.RegisterMethod("plugin.approval.resolve", string(gateway.ScopeApprovals), handlePluginApprovalResolve)
}

func handleExecApprovalsGet(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"policy": nil}, nil
}

func handleExecApprovalsSet(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"ok": true}, nil
}

func handleExecApprovalsNodeGet(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"policy": nil}, nil
}

func handleExecApprovalsNodeSet(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"ok": true}, nil
}

func handleExecApprovalGet(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"approval": nil}, nil
}

func handleExecApprovalList(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"approvals": []interface{}{}}, nil
}

func handleExecApprovalRequest(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"ok": true}, nil
}

func handleExecApprovalWaitDecision(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"decision": nil}, nil
}

func handleExecApprovalResolve(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"ok": true}, nil
}

func handlePluginApprovalList(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"approvals": []interface{}{}}, nil
}

func handlePluginApprovalRequest(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"ok": true}, nil
}

func handlePluginApprovalWaitDecision(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"decision": nil}, nil
}

func handlePluginApprovalResolve(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"ok": true}, nil
}
