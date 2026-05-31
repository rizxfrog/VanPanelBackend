package builtin

import (
	"github.com/cloudwego/eino/components/tool"
	"github.com/rizxfrog/VanPanelBackend/internal/model"
)

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

// BuiltinToolDefs 返回所有内置工具的数据库记录定义，用于持久化到 cl_agent_builtin_tools 表。
func BuiltinToolDefs() []*model.BuiltinTool {
	return []*model.BuiltinTool{
		// 网络工具
		{Name: "net.lsof", DisplayName: "lsof", Description: "列出打开的文件和网络连接（排查端口占用、文件句柄泄漏）", Category: "network", Enabled: true},
		{Name: "net.ss", DisplayName: "ss", Description: "查看套接字统计信息（替代 netstat，更快更详细）", Category: "network", Enabled: true},
		{Name: "net.netstat", DisplayName: "netstat", Description: "网络连接、路由表、接口统计", Category: "network", Enabled: true},
		// 日志工具
		{Name: "log.journalctl", DisplayName: "journalctl", Description: "查询 systemd 日志（支持按 unit、优先级、时间范围过滤）", Category: "log", Enabled: true},
		{Name: "log.dmesg", DisplayName: "dmesg", Description: "查看内核环形缓冲区日志（排查硬件错误、OOM、驱动问题）", Category: "log", Enabled: true},
		{Name: "log.tail", DisplayName: "tail", Description: "追踪日志文件末尾内容（实时查看日志更新）", Category: "log", Enabled: true},
		// 进程工具
		{Name: "proc.ps", DisplayName: "ps", Description: "列出系统进程（按内存排序，排查内存占用高的进程）", Category: "process", Enabled: true},
		{Name: "proc.top", DisplayName: "top", Description: "获取进程资源占用快照（CPU、内存、负载等实时指标）", Category: "process", Enabled: true},
		{Name: "proc.pgrep", DisplayName: "pgrep", Description: "按名称查找进程（快速定位特定进程 PID）", Category: "process", Enabled: true},
		// 磁盘工具
		{Name: "disk.df", DisplayName: "df", Description: "查看文件系统磁盘空间使用情况（排查磁盘满问题）", Category: "disk", Enabled: true},
		{Name: "disk.du", DisplayName: "du", Description: "查看目录磁盘占用（定位大目录）", Category: "disk", Enabled: true},
		{Name: "disk.iostat", DisplayName: "iostat", Description: "查看磁盘 I/O 统计（排查 I/O 瓶颈）", Category: "disk", Enabled: true},
		// 系统工具
		{Name: "sys.free", DisplayName: "free", Description: "查看内存和交换空间使用情况（排查内存不足问题）", Category: "system", Enabled: true},
		{Name: "sys.vmstat", DisplayName: "vmstat", Description: "查看虚拟内存、CPU 和 I/O 统计", Category: "system", Enabled: true},
		{Name: "sys.uname", DisplayName: "uname", Description: "查看系统信息（内核版本、架构、主机名等）", Category: "system", Enabled: true},
		// 服务工具
		{Name: "svc.systemctl", DisplayName: "systemctl", Description: "systemd 服务管理（查看状态、启停、重启服务）", Category: "service", Enabled: true},
		{Name: "svc.uptime", DisplayName: "uptime", Description: "查看系统运行时间和平均负载", Category: "service", Enabled: true},
		// Shell 执行器
		{Name: "shell.exec", DisplayName: "Shell Exec", Description: "执行 Shell 命令（通用执行器，可运行任意命令）", Category: "shell", Enabled: true},
	}
}
