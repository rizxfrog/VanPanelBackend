package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// ShellExecTool 执行用户指定的任意 Shell 命令。
type ShellExecTool struct {
	baseCommandTool
}

// NewShellExecTool 创建 ShellExecTool 实例。
func NewShellExecTool() *ShellExecTool {
	return &ShellExecTool{baseCommandTool{
		name:           "shell.exec",
		description:    "执行 Shell 命令（通用执行器，可运行任意命令）",
		command:        "",
		windowsCommand: "{args}",
		timeout:        30 * time.Second,
		maxOutput:      65536,
	}}
}

func (t *ShellExecTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: t.name,
		Desc: t.description,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"command": {Type: schema.String, Desc: "要执行的完整命令", Required: true},
		}),
	}, nil
}

func (t *ShellExecTool) InvokableRun(ctx context.Context, argsJSON string, _ ...tool.Option) (string, error) {
	var params struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &params); err != nil {
		return "", fmt.Errorf("解析参数失败: %w", err)
	}
	if params.Command == "" {
		return "", fmt.Errorf("command 参数不能为空")
	}
	return t.runCommand(ctx, params.Command)
}
