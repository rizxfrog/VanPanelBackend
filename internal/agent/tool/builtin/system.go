package builtin

import (
	"context"
	"encoding/json"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// FreeTool 查看内存和交换空间使用情况。
type FreeTool struct{ baseCommandTool }

func NewFreeTool() *FreeTool {
	return &FreeTool{baseCommandTool{
		name: "sys.free", description: "查看内存和交换空间使用情况（排查内存不足问题）",
		command: "free -h", timeout: 30 * time.Second, maxOutput: 65536,
	}}
}

func (t *FreeTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: t.name, Desc: t.description,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"args": {Type: schema.String, Desc: "free 参数，如 -s 5 或 -w"},
		}),
	}, nil
}

func (t *FreeTool) InvokableRun(ctx context.Context, argsJSON string, _ ...tool.Option) (string, error) {
	var p struct{ Args string `json:"args"` }
	if err := json.Unmarshal([]byte(argsJSON), &p); err != nil {
		return t.runCommand(ctx, "")
	}
	return t.runCommand(ctx, p.Args)
}

// VMStatTool 查看虚拟内存、CPU 和 I/O 统计。
type VMStatTool struct{ baseCommandTool }

func NewVMStatTool() *VMStatTool {
	return &VMStatTool{baseCommandTool{
		name: "sys.vmstat", description: "查看虚拟内存、CPU 和 I/O 统计",
		command: "vmstat", timeout: 30 * time.Second, maxOutput: 65536,
	}}
}

func (t *VMStatTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: t.name, Desc: t.description,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"args": {Type: schema.String, Desc: "vmstat 参数，如 1 5 或 -w -S M"},
		}),
	}, nil
}

func (t *VMStatTool) InvokableRun(ctx context.Context, argsJSON string, _ ...tool.Option) (string, error) {
	var p struct{ Args string `json:"args"` }
	if err := json.Unmarshal([]byte(argsJSON), &p); err != nil {
		return t.runCommand(ctx, "")
	}
	return t.runCommand(ctx, p.Args)
}

// UnameTool 查看系统信息。
type UnameTool struct{ baseCommandTool }

func NewUnameTool() *UnameTool {
	return &UnameTool{baseCommandTool{
		name: "sys.uname", description: "查看系统信息（内核版本、架构、主机名等）",
		command: "uname -a", timeout: 30 * time.Second, maxOutput: 65536,
	}}
}

func (t *UnameTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: t.name, Desc: t.description,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"args": {Type: schema.String, Desc: "uname 参数，如 -r 或 -m"},
		}),
	}, nil
}

func (t *UnameTool) InvokableRun(ctx context.Context, argsJSON string, _ ...tool.Option) (string, error) {
	var p struct{ Args string `json:"args"` }
	if err := json.Unmarshal([]byte(argsJSON), &p); err != nil {
		return t.runCommand(ctx, "")
	}
	return t.runCommand(ctx, p.Args)
}

// SystemctlTool systemd 服务管理。
type SystemctlTool struct{ baseCommandTool }

func NewSystemctlTool() *SystemctlTool {
	return &SystemctlTool{baseCommandTool{
		name: "svc.systemctl", description: "systemd 服务管理（查看状态、启停、重启服务）",
		command: "systemctl", timeout: 30 * time.Second, maxOutput: 65536,
	}}
}

func (t *SystemctlTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: t.name, Desc: t.description,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"args": {Type: schema.String, Desc: "systemctl 参数，如 status nginx 或 list-units --type=service"},
		}),
	}, nil
}

func (t *SystemctlTool) InvokableRun(ctx context.Context, argsJSON string, _ ...tool.Option) (string, error) {
	var p struct{ Args string `json:"args"` }
	if err := json.Unmarshal([]byte(argsJSON), &p); err != nil {
		return t.runCommand(ctx, "")
	}
	return t.runCommand(ctx, p.Args)
}

// UptimeTool 查看系统运行时间和负载。
type UptimeTool struct{ baseCommandTool }

func NewUptimeTool() *UptimeTool {
	return &UptimeTool{baseCommandTool{
		name: "svc.uptime", description: "查看系统运行时间和平均负载",
		command: "uptime", timeout: 30 * time.Second, maxOutput: 65536,
	}}
}

func (t *UptimeTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: t.name, Desc: t.description,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"args": {Type: schema.String, Desc: "uptime 参数，如 -p"},
		}),
	}, nil
}

func (t *UptimeTool) InvokableRun(ctx context.Context, argsJSON string, _ ...tool.Option) (string, error) {
	var p struct{ Args string `json:"args"` }
	if err := json.Unmarshal([]byte(argsJSON), &p); err != nil {
		return t.runCommand(ctx, "")
	}
	return t.runCommand(ctx, p.Args)
}
