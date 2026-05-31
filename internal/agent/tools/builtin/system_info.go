package builtin

import (
	"context"
	"os"
	"runtime"
	"time"

	"github.com/rizxfrog/VanPanelBackend/internal/agent/tools"
)

type systemInfoTool struct{}

func SystemInfoTool() tools.Tool { return systemInfoTool{} }

func (systemInfoTool) Name() string { return "system.info" }

func (systemInfoTool) Description() string {
	return "获取当前后端主机的运行时信息：主机名、操作系统、架构、Go 版本、PID、CPU 核数、当前时间"
}

func (systemInfoTool) Parameters() tools.ToolSchema {
	return tools.ToolSchema{Type: "object", Properties: map[string]tools.Property{}}
}

func (systemInfoTool) Execute(_ context.Context, _ map[string]any) (map[string]any, error) {
	hostname, _ := os.Hostname()
	return map[string]any{
		"hostname":  hostname,
		"os":        runtime.GOOS,
		"arch":      runtime.GOARCH,
		"goVersion": runtime.Version(),
		"pid":       os.Getpid(),
		"numCPU":    runtime.NumCPU(),
		"time":      time.Now().Format(time.RFC3339),
	}, nil
}
