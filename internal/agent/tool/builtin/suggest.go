package builtin

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// ShellSuggestTool 返回终端命令建议但不执行，用于安全的命令推荐。
type ShellSuggestTool struct{}

// NewShellSuggestTool 创建 ShellSuggestTool 实例。
func NewShellSuggestTool() *ShellSuggestTool {
	return &ShellSuggestTool{}
}

func (t *ShellSuggestTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "shell.suggest",
		Desc: "返回终端命令建议但不执行，用于安全的命令推荐（如排查故障时推荐下一步操作的命令）",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"command": {Type: schema.String, Desc: "推荐的命令", Required: true},
			"reason":  {Type: schema.String, Desc: "推荐原因"},
		}),
	}, nil
}

func (t *ShellSuggestTool) InvokableRun(_ context.Context, argsJSON string, _ ...tool.Option) (string, error) {
	var p struct {
		Command string `json:"command"`
		Reason  string `json:"reason"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &p); err != nil {
		return "", err
	}

	command := strings.TrimSpace(p.Command)
	if command == "" {
		command = "uname -a"
	}

	result := "建议执行命令: " + command + "\n注意: 此命令仅作推荐，未被实际执行"
	if p.Reason != "" {
		result += "\n原因: " + p.Reason
	}
	return result, nil
}
