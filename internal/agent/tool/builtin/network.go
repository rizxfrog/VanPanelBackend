package builtin

import (
	"context"
	"encoding/json"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// LsofTool 列出打开的文件和网络连接。
type LsofTool struct{ baseCommandTool }

func NewLsofTool() *LsofTool {
	return &LsofTool{baseCommandTool{
		name: "net.lsof", description: "列出打开的文件和网络连接（排查端口占用、文件句柄泄漏）",
		command: "lsof", timeout: 30 * time.Second, maxOutput: 65536,
	}}
}

func (t *LsofTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: t.name, Desc: t.description,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"args": {Type: schema.String, Desc: "lsof 参数，如 -i :8080 -p 1234"},
		}),
	}, nil
}

func (t *LsofTool) InvokableRun(ctx context.Context, argsJSON string, _ ...tool.Option) (string, error) {
	var p struct{ Args string `json:"args"` }
	if err := json.Unmarshal([]byte(argsJSON), &p); err != nil {
		return t.runCommand(ctx, "")
	}
	return t.runCommand(ctx, p.Args)
}

// SSTool 查看套接字统计信息。
type SSTool struct{ baseCommandTool }

func NewSSTool() *SSTool {
	return &SSTool{baseCommandTool{
		name: "net.ss", description: "查看套接字统计信息（替代 netstat，更快更详细）",
		command: "ss", timeout: 30 * time.Second, maxOutput: 65536,
	}}
}

func (t *SSTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: t.name, Desc: t.description,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"args": {Type: schema.String, Desc: "ss 参数，如 -tlnp -s"},
		}),
	}, nil
}

func (t *SSTool) InvokableRun(ctx context.Context, argsJSON string, _ ...tool.Option) (string, error) {
	var p struct{ Args string `json:"args"` }
	if err := json.Unmarshal([]byte(argsJSON), &p); err != nil {
		return t.runCommand(ctx, "")
	}
	return t.runCommand(ctx, p.Args)
}

// NetstatTool 网络连接、路由表、接口统计。
type NetstatTool struct{ baseCommandTool }

func NewNetstatTool() *NetstatTool {
	return &NetstatTool{baseCommandTool{
		name: "net.netstat", description: "网络连接、路由表、接口统计",
		command: "netstat", timeout: 30 * time.Second, maxOutput: 65536,
	}}
}

func (t *NetstatTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: t.name, Desc: t.description,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"args": {Type: schema.String, Desc: "netstat 参数，如 -tlnp -r -i"},
		}),
	}, nil
}

func (t *NetstatTool) InvokableRun(ctx context.Context, argsJSON string, _ ...tool.Option) (string, error) {
	var p struct{ Args string `json:"args"` }
	if err := json.Unmarshal([]byte(argsJSON), &p); err != nil {
		return t.runCommand(ctx, "")
	}
	return t.runCommand(ctx, p.Args)
}
