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
		command:        "df -h",
		windowsCommand: `Get-PSDrive -PSProvider FileSystem | Where-Object Used -gt 0 | Select-Object Name,@{N='Total(GB)';E={[math]::Round(($_.Used+$_.Free)/1GB,2)}},@{N='Used(GB)';E={[math]::Round($_.Used/1GB,2)}},@{N='Free(GB)';E={[math]::Round($_.Free/1GB,2)}},@{N='Use%';E={[math]::Round($_.Used/($_.Used+$_.Free)*100,1)}} | Format-Table -AutoSize`,
		timeout:        30 * time.Second,
		maxOutput:      65536,
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
	var p struct {
		Args string `json:"args"`
	}
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
		command:        "du -sh",
		windowsCommand: `$path="{args}"; if (-not $path) { $path="." }; $size=(Get-ChildItem -Path $path -Recurse -ErrorAction SilentlyContinue | Measure-Object -Property Length -Sum).Sum; if ($size -gt 1GB) { Write-Output ("{0:N2} GB " + $path) -f ($size / 1GB) } elseif ($size -gt 1MB) { Write-Output ("{0:N2} MB " + $path) -f ($size / 1MB) } else { Write-Output ("{0:N2} KB " + $path) -f ($size / 1KB) }`,
		timeout:        30 * time.Second,
		maxOutput:      65536,
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
	var p struct {
		Args string `json:"args"`
	}
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
		command:        "iostat",
		windowsCommand: `Write-Output "--- Disk I/O Stats ---"; Get-Counter '\PhysicalDisk(*)\Disk Reads/sec','\PhysicalDisk(*)\Disk Writes/sec','\PhysicalDisk(*)\Disk Read Bytes/sec','\PhysicalDisk(*)\Disk Write Bytes/sec','\PhysicalDisk(*)\Avg. Disk sec/Read','\PhysicalDisk(*)\Avg. Disk sec/Write' | Select-Object -ExpandProperty CounterSamples | Where-Object InstanceName -ne "_total" | Format-Table InstanceName,@{N='Metric';E={$_.Path.Split('\')[-1]}},@{N='Value';E={[math]::Round($_.CookedValue,2)}} -AutoSize`,
		timeout:        30 * time.Second,
		maxOutput:      65536,
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
	var p struct {
		Args string `json:"args"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &p); err != nil {
		return t.runCommand(ctx, "")
	}
	return t.runCommand(ctx, p.Args)
}
