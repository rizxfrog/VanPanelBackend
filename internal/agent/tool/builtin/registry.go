package builtin

import "github.com/cloudwego/eino/components/tool"

// NewBuiltinTools 返回所有内置运维工具。
// 共 18 个工具，覆盖网络、日志、进程、磁盘、系统和 Shell 六大类。
func NewBuiltinTools() []tool.BaseTool {
	return []tool.BaseTool{
		// 网络工具
		NewLsofTool(),
		NewSSTool(),
		NewNetstatTool(),
		// 日志工具
		NewJournalctlTool(),
		NewDmesgTool(),
		NewTailTool(),
		// 进程工具
		NewPSTool(),
		NewTopTool(),
		NewPgrepTool(),
		// 磁盘工具
		NewDFTool(),
		NewDUTool(),
		NewIOStatTool(),
		// 系统工具
		NewFreeTool(),
		NewVMStatTool(),
		NewUnameTool(),
		// 服务工具
		NewSystemctlTool(),
		NewUptimeTool(),
		// Shell 执行器
		NewShellExecTool(),
	}
}
