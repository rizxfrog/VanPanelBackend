package builtin

import (
	"context"
	"encoding/json"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// PSTool 列出系统进程（按内存排序）。
type PSTool struct{ baseCommandTool }

func NewPSTool() *PSTool {
	return &PSTool{baseCommandTool{
		name: "proc.ps", description: "列出系统进程（按内存排序，排查内存占用高的进程）",
		command:        "ps aux --sort=-%mem",
		windowsCommand: `Get-Process | Sort-Object WorkingSet64 -Descending | Select-Object -First 30 Id,ProcessName,@{N='CPU(s)';E={$_.CPU}},@{N='Mem(MB)';E={[math]::Round($_.WorkingSet64/1MB,2)}},@{N='PM(MB)';E={[math]::Round($_.PagedMemorySize64/1MB,2)}} | Format-Table -AutoSize`,
		timeout:        30 * time.Second,
		maxOutput:      65536,
	}}
}

func (t *PSTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: t.name, Desc: t.description,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"args": {Type: schema.String, Desc: "额外 ps 参数，如 | head -20"},
		}),
	}, nil
}

func (t *PSTool) InvokableRun(ctx context.Context, argsJSON string, _ ...tool.Option) (string, error) {
	var p struct {
		Args string `json:"args"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &p); err != nil {
		return t.runCommand(ctx, "")
	}
	return t.runCommand(ctx, p.Args)
}

// TopTool 获取进程资源占用快照（非交互模式）。
type TopTool struct{ baseCommandTool }

func NewTopTool() *TopTool {
	return &TopTool{baseCommandTool{
		name: "proc.top", description: "获取进程资源占用快照（CPU、内存、负载等实时指标）",
		command:        "top -b -n1",
		windowsCommand: `$cpu=(Get-CimInstance Win32_Processor | Measure-Object -Property LoadPercentage -Average).Average; $mem=Get-CimInstance Win32_OperatingSystem; $memTotal=[math]::Round($mem.TotalVisibleMemorySize/1MB,2); $memFree=[math]::Round($mem.FreePhysicalMemory/1MB,2); Write-Output "CPU Usage: $cpu%"; Write-Output "Memory: Total=${memTotal}MB  Used=$([math]::Round($memTotal-$memFree,2))MB  Free=${memFree}MB"; Write-Output ""; Get-Process | Sort-Object CPU -Descending | Select-Object -First 15 ProcessName,Id,@{N='CPU(s)';E={$_.CPU}},@{N='Mem(MB)';E={[math]::Round($_.WorkingSet64/1MB,2)}} | Format-Table -AutoSize`,
		timeout:        30 * time.Second,
		maxOutput:      65536,
	}}
}

func (t *TopTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: t.name, Desc: t.description,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"args": {Type: schema.String, Desc: "额外 top 参数，如 -u root -o %CPU"},
		}),
	}, nil
}

func (t *TopTool) InvokableRun(ctx context.Context, argsJSON string, _ ...tool.Option) (string, error) {
	var p struct {
		Args string `json:"args"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &p); err != nil {
		return t.runCommand(ctx, "")
	}
	return t.runCommand(ctx, p.Args)
}

// PgrepTool 按名称查找进程。
type PgrepTool struct{ baseCommandTool }

func NewPgrepTool() *PgrepTool {
	return &PgrepTool{baseCommandTool{
		name: "proc.pgrep", description: "按名称查找进程（快速定位特定进程 PID）",
		command:        "pgrep -a",
		windowsCommand: `Get-Process -Name "{args}" -ErrorAction SilentlyContinue | Select-Object Id,ProcessName,Path,@{N='Mem(MB)';E={[math]::Round($_.WorkingSet64/1MB,2)}} | Format-Table -AutoSize`,
		timeout:        30 * time.Second,
		maxOutput:      65536,
	}}
}

func (t *PgrepTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: t.name, Desc: t.description,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"args": {Type: schema.String, Desc: "pgrep 参数/模式，如 nginx 或 -f 'python.*app'"},
		}),
	}, nil
}

func (t *PgrepTool) InvokableRun(ctx context.Context, argsJSON string, _ ...tool.Option) (string, error) {
	var p struct {
		Args string `json:"args"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &p); err != nil {
		return t.runCommand(ctx, "")
	}
	return t.runCommand(ctx, p.Args)
}
