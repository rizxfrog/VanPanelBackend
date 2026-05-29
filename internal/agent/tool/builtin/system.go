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
		command:        "free -h",
		windowsCommand: `$os=Get-CimInstance Win32_OperatingSystem; $total=[math]::Round($os.TotalVisibleMemorySize/1MB,2); $free=[math]::Round($os.FreePhysicalMemory/1MB,2); $used=$total-$free; $avail=[math]::Round($os.FreeVirtualMemory/1MB,2); Write-Output "              total        used        free      available"; Write-Output ("Mem:       {0,8} {1,8} {2,8} {3,8}" -f "$($total)G","$($used)G","$($free)G","$($avail)G")`,
		timeout:        30 * time.Second,
		maxOutput:      65536,
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
	var p struct {
		Args string `json:"args"`
	}
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
		command:        "vmstat",
		windowsCommand: `Write-Output "--- Memory ---"; Get-Counter '\Memory\Available MBytes','\Memory\Committed Bytes','\Memory\Pages/sec' | Select-Object -ExpandProperty CounterSamples | Format-Table InstanceName,@{N='Counter';E={$_.Path.Split('\')[-1]}},CookedValue -AutoSize; Write-Output ""; Write-Output "--- CPU ---"; Get-Counter '\Processor(_Total)\% Processor Time' | Select-Object -ExpandProperty CounterSamples | Select-Object InstanceName,@{N='Counter';E={$_.Path.Split('\')[-1]}},@{N='Value';E={[math]::Round($_.CookedValue,2)}} | Format-Table -AutoSize`,
		timeout:        30 * time.Second,
		maxOutput:      65536,
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
	var p struct {
		Args string `json:"args"`
	}
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
		command:        "uname -a",
		windowsCommand: `$ci=Get-ComputerInfo; Write-Output ("Windows $($ci.WindowsProductName) $($ci.WindowsVersion) $($ci.OsArchitecture)"); Write-Output ("Hostname: $env:COMPUTERNAME"); Write-Output ("Processors: $($ci.CsProcessors.Count) x $($ci.CsProcessors[0].Name)"); Write-Output ("Total Memory: $([math]::Round($ci.CsTotalPhysicalMemory/1GB,2)) GB")`,
		timeout:        30 * time.Second,
		maxOutput:      65536,
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
	var p struct {
		Args string `json:"args"`
	}
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
		command:        "systemctl",
		windowsCommand: `$svc=Get-Service -Name "{args}" -ErrorAction SilentlyContinue; if ($svc) { $svc | Format-List Name,DisplayName,Status,StartType } else { Write-Output "服务列表："; Get-Service | Select-Object Name,DisplayName,Status | Format-Table -AutoSize }`,
		timeout:        30 * time.Second,
		maxOutput:      65536,
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
	var p struct {
		Args string `json:"args"`
	}
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
		command:        "uptime",
		windowsCommand: `$boot=(gcim Win32_OperatingSystem).LastBootUpTime; $uptime=New-TimeSpan -Start $boot -End (Get-Date); Write-Output ("Up $($uptime.Days) days $($uptime.Hours) hours $($uptime.Minutes) minutes"); Write-Output ("Boot time: $($boot.ToString('yyyy-MM-dd HH:mm:ss'))")`,
		timeout:        30 * time.Second,
		maxOutput:      65536,
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
	var p struct {
		Args string `json:"args"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &p); err != nil {
		return t.runCommand(ctx, "")
	}
	return t.runCommand(ctx, p.Args)
}
