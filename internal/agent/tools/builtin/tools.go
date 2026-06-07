package builtin

import "github.com/rizxfrog/VanPanelBackend/internal/agent/tools"

// NewTools 返回所有内置工具（Tier 1，无需外部依赖）
func NewTools() []tools.Tool {
	return []tools.Tool{
		SystemInfoTool(),
		CPULoadTool(),
		MemoryUsageTool(),
		DiskUsageTool(),
		ProcessListTool(),
	}
}
