package builtin

import (
	"context"

	"github.com/GoSimplicity/AI-CloudOps/internal/agent/tools"
)

type diskUsageTool struct{}

func DiskUsageTool() tools.Tool { return diskUsageTool{} }

func (diskUsageTool) Name() string { return "system.disk.usage" }

func (diskUsageTool) Description() string {
	return "获取指定路径的磁盘使用情况：总空间、已用空间、可用空间、使用率"
}

func (diskUsageTool) Parameters() tools.ToolSchema {
	return tools.ToolSchema{
		Type: "object",
		Properties: map[string]tools.Property{
			"path": {Type: "string", Description: "要查询的磁盘路径，Linux 默认 /，Windows 默认 C:\\", Default: "/"},
		},
	}
}

func (diskUsageTool) Execute(_ context.Context, args map[string]any) (map[string]any, error) {
	path := defaultDiskPath()
	if p, ok := args["path"].(string); ok && p != "" {
		path = p
	}
	return platformDiskUsage(path)
}
