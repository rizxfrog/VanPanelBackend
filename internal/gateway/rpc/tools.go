package rpc

import (
	"context"
	"encoding/json"

	"github.com/rizxfrog/VanPanelBackend/internal/gateway"
)

func init() {
	gateway.RegisterMethod("tools.catalog", string(gateway.ScopeRead), handleToolsCatalog)
	gateway.RegisterMethod("tools.effective", string(gateway.ScopeRead), handleToolsEffective)
	gateway.RegisterMethod("tools.invoke", string(gateway.ScopeWrite), handleToolsInvoke)
}

type toolDef struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Parameters  toolParams `json:"parameters"`
}

type toolParams struct {
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties"`
	Required   []string               `json:"required"`
}

func listGatewayTools() []toolDef {
	return []toolDef{
		{"shell.exec", "Execute a shell command", toolParams{Type: "object", Properties: map[string]interface{}{}, Required: []string{}}},
		{"shell.suggest", "Suggest shell commands", toolParams{Type: "object", Properties: map[string]interface{}{}, Required: []string{}}},
		{"sys.free", "Show memory usage", toolParams{Type: "object", Properties: map[string]interface{}{}, Required: []string{}}},
		{"sys.vmstat", "Virtual memory statistics", toolParams{Type: "object", Properties: map[string]interface{}{}, Required: []string{}}},
		{"sys.uname", "System information", toolParams{Type: "object", Properties: map[string]interface{}{}, Required: []string{}}},
		{"sys.systemctl", "Systemd service control", toolParams{Type: "object", Properties: map[string]interface{}{}, Required: []string{}}},
		{"sys.uptime", "System uptime", toolParams{Type: "object", Properties: map[string]interface{}{}, Required: []string{}}},
		{"sys.inspect", "Inspect system state", toolParams{Type: "object", Properties: map[string]interface{}{}, Required: []string{}}},
		{"proc.ps", "List processes", toolParams{Type: "object", Properties: map[string]interface{}{}, Required: []string{}}},
		{"proc.top", "Top processes", toolParams{Type: "object", Properties: map[string]interface{}{}, Required: []string{}}},
		{"proc.pgrep", "Search processes", toolParams{Type: "object", Properties: map[string]interface{}{}, Required: []string{}}},
		{"net.lsof", "List open files", toolParams{Type: "object", Properties: map[string]interface{}{}, Required: []string{}}},
		{"net.ss", "Socket statistics", toolParams{Type: "object", Properties: map[string]interface{}{}, Required: []string{}}},
		{"net.netstat", "Network statistics", toolParams{Type: "object", Properties: map[string]interface{}{}, Required: []string{}}},
		{"disk.df", "Disk free", toolParams{Type: "object", Properties: map[string]interface{}{}, Required: []string{}}},
		{"disk.du", "Disk usage", toolParams{Type: "object", Properties: map[string]interface{}{}, Required: []string{}}},
		{"disk.iostat", "IO statistics", toolParams{Type: "object", Properties: map[string]interface{}{}, Required: []string{}}},
		{"log.journalctl", "Query systemd journal", toolParams{Type: "object", Properties: map[string]interface{}{}, Required: []string{}}},
		{"log.dmesg", "Kernel ring buffer", toolParams{Type: "object", Properties: map[string]interface{}{}, Required: []string{}}},
		{"log.tail", "Tail log files", toolParams{Type: "object", Properties: map[string]interface{}{}, Required: []string{}}},
		{"file.scan", "Scan directory contents", toolParams{Type: "object", Properties: map[string]interface{}{}, Required: []string{}}},
		{"container.inspect", "Inspect container", toolParams{Type: "object", Properties: map[string]interface{}{}, Required: []string{}}},
		{"prometheus.query", "Query Prometheus metrics", toolParams{Type: "object", Properties: map[string]interface{}{}, Required: []string{}}},
	}
}

func handleToolsCatalog(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return listGatewayTools(), nil
}

func handleToolsEffective(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{
		"tools":        listGatewayTools(),
		"defaultTools": []interface{}{},
		"sessionTools": map[string]interface{}{},
	}, nil
}

func handleToolsInvoke(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{
		"result": "tool execution stub",
		"error":  nil,
	}, nil
}
