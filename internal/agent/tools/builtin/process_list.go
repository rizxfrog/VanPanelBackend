package builtin

import (
	"context"
	"fmt"

	"github.com/rizxfrog/VanPanelBackend/internal/agent/tools"
)

type processListTool struct{}

func ProcessListTool() tools.Tool { return processListTool{} }

func (processListTool) Name() string { return "process.list" }

func (processListTool) Description() string {
	return "列出系统中占用内存最多的进程（Top N），返回进程名、PID、内存使用量"
}

func (processListTool) Parameters() tools.ToolSchema {
	return tools.ToolSchema{
		Type: "object",
		Properties: map[string]tools.Property{
			"limit": {Type: "integer", Description: "返回的进程数量，默认 10", Default: 10},
		},
	}
}

type procInfo struct {
	PID      int
	Name     string
	RSSBytes uint64
}

func (processListTool) Execute(_ context.Context, args map[string]any) (map[string]any, error) {
	limit := 10
	if n, ok := args["limit"].(float64); ok && n > 0 {
		limit = int(n)
		if limit > 50 {
			limit = 50
		}
	}

	procs, err := platformProcessList()
	if err != nil {
		return nil, fmt.Errorf("read processes: %w", err)
	}

	// 按内存排序，取 Top N
	sortByRSS(procs)
	if len(procs) > limit {
		procs = procs[:limit]
	}

	result := make([]map[string]any, len(procs))
	for i, p := range procs {
		result[i] = map[string]any{
			"pid":      p.PID,
			"name":     p.Name,
			"rssBytes": p.RSSBytes,
			"rssHuman": formatBytes(p.RSSBytes),
		}
	}
	return map[string]any{"processes": result, "count": len(result)}, nil
}

func sortByRSS(procs []procInfo) {
	for i := 1; i < len(procs); i++ {
		for j := i; j > 0 && procs[j].RSSBytes > procs[j-1].RSSBytes; j-- {
			procs[j], procs[j-1] = procs[j-1], procs[j]
		}
	}
}

func formatBytes(bytes uint64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)
	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1fGB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.1fMB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.1fKB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%dB", bytes)
	}
}
