package builtin

import (
	"context"
	"encoding/json"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// JournalctlTool 查询 systemd 日志。
type JournalctlTool struct{ baseCommandTool }

func NewJournalctlTool() *JournalctlTool {
	return &JournalctlTool{baseCommandTool{
		name: "log.journalctl", description: "查询 systemd 日志（支持按 unit、优先级、时间范围过滤）",
		command:        "journalctl",
		windowsCommand: `Get-WinEvent -LogName System -MaxEvents 100 | Select-Object TimeCreated,Id,LevelDisplayName,ProviderName,Message | Format-List`,
		timeout:        30 * time.Second,
		maxOutput:      65536,
	}}
}

func (t *JournalctlTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: t.name, Desc: t.description,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"args": {Type: schema.String, Desc: "journalctl 参数，如 -u nginx -n 50 --since '1 hour ago'"},
		}),
	}, nil
}

func (t *JournalctlTool) InvokableRun(ctx context.Context, argsJSON string, _ ...tool.Option) (string, error) {
	var p struct {
		Args string `json:"args"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &p); err != nil {
		return t.runCommand(ctx, "")
	}
	return t.runCommand(ctx, p.Args)
}

// DmesgTool 查看内核环形缓冲区日志。
type DmesgTool struct{ baseCommandTool }

func NewDmesgTool() *DmesgTool {
	return &DmesgTool{baseCommandTool{
		name: "log.dmesg", description: "查看内核环形缓冲区日志（排查硬件错误、OOM、驱动问题）",
		command:        "dmesg",
		windowsCommand: `Get-WinEvent -LogName System -MaxEvents 100 -FilterXPath "*[System[(Level=1 or Level=2)]]" | Select-Object TimeCreated,Id,LevelDisplayName,ProviderName,Message | Format-List`,
		timeout:        30 * time.Second,
		maxOutput:      65536,
	}}
}

func (t *DmesgTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: t.name, Desc: t.description,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"args": {Type: schema.String, Desc: "dmesg 参数，如 -T --level=err -n 100"},
		}),
	}, nil
}

func (t *DmesgTool) InvokableRun(ctx context.Context, argsJSON string, _ ...tool.Option) (string, error) {
	var p struct {
		Args string `json:"args"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &p); err != nil {
		return t.runCommand(ctx, "")
	}
	return t.runCommand(ctx, p.Args)
}

// TailTool 追踪日志文件末尾内容。
type TailTool struct{ baseCommandTool }

func NewTailTool() *TailTool {
	return &TailTool{baseCommandTool{
		name: "log.tail", description: "追踪日志文件末尾内容（实时查看日志更新）",
		command:        "tail",
		windowsCommand: `$path="{args}"; if (Test-Path $path) { Get-Content -Path $path -Tail 100 } else { Write-Output "文件不存在: $path" }`,
		timeout:        30 * time.Second,
		maxOutput:      65536,
	}}
}

func (t *TailTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: t.name, Desc: t.description,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"args": {Type: schema.String, Desc: "tail 参数，如 -n 100 /var/log/syslog"},
		}),
	}, nil
}

func (t *TailTool) InvokableRun(ctx context.Context, argsJSON string, _ ...tool.Option) (string, error) {
	var p struct {
		Args string `json:"args"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &p); err != nil {
		return t.runCommand(ctx, "")
	}
	return t.runCommand(ctx, p.Args)
}
