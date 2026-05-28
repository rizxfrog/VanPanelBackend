package builtin

import (
	"context"
	"encoding/json"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// DFTool 查看文件系统磁盘空间使用情况。
type DFTool struct{ baseCommandTool }

func NewDFTool() *DFTool {
	return &DFTool{baseCommandTool{
		name: "disk.df", description: "查看文件系统磁盘空间使用情况（排查磁盘满问题）",
		command: "df -h", timeout: 30 * time.Second, maxOutput: 65536,
	}}
}

func (t *DFTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: t.name, Desc: t.description,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"args": {Type: schema.String, Desc: "df 参数，如 -T / -i"},
		}),
	}, nil
}

func (t *DFTool) InvokableRun(ctx context.Context, argsJSON string, _ ...tool.Option) (string, error) {
	var p struct{ Args string `json:"args"` }
	if err := json.Unmarshal([]byte(argsJSON), &p); err != nil {
		return t.runCommand(ctx, "")
	}
	return t.runCommand(ctx, p.Args)
}

// DUTool 查看目录磁盘占用。
type DUTool struct{ baseCommandTool }

func NewDUTool() *DUTool {
	return &DUTool{baseCommandTool{
		name: "disk.du", description: "查看目录磁盘占用（定位大目录）",
		command: "du -sh", timeout: 30 * time.Second, maxOutput: 65536,
	}}
}

func (t *DUTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: t.name, Desc: t.description,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"args": {Type: schema.String, Desc: "du 参数及路径，如 /var/* 或 --max-depth=1 /home"},
		}),
	}, nil
}

func (t *DUTool) InvokableRun(ctx context.Context, argsJSON string, _ ...tool.Option) (string, error) {
	var p struct{ Args string `json:"args"` }
	if err := json.Unmarshal([]byte(argsJSON), &p); err != nil {
		return t.runCommand(ctx, "")
	}
	return t.runCommand(ctx, p.Args)
}

// IOStatTool 查看磁盘 I/O 统计。
type IOStatTool struct{ baseCommandTool }

func NewIOStatTool() *IOStatTool {
	return &IOStatTool{baseCommandTool{
		name: "disk.iostat", description: "查看磁盘 I/O 统计（排查 I/O 瓶颈）",
		command: "iostat", timeout: 30 * time.Second, maxOutput: 65536,
	}}
}

func (t *IOStatTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: t.name, Desc: t.description,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"args": {Type: schema.String, Desc: "iostat 参数，如 -x 1 3 或 -d"},
		}),
	}, nil
}

func (t *IOStatTool) InvokableRun(ctx context.Context, argsJSON string, _ ...tool.Option) (string, error) {
	var p struct{ Args string `json:"args"` }
	if err := json.Unmarshal([]byte(argsJSON), &p); err != nil {
		return t.runCommand(ctx, "")
	}
	return t.runCommand(ctx, p.Args)
}
