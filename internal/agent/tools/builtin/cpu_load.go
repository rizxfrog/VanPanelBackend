package builtin

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/rizxfrog/VanPanelBackend/internal/agent/tools"
)

type cpuLoadTool struct{}

func CPULoadTool() tools.Tool { return cpuLoadTool{} }

func (cpuLoadTool) Name() string { return "system.cpu.load" }

func (cpuLoadTool) Description() string {
	return "获取系统 CPU 负载：1 分钟、5 分钟、15 分钟平均负载及 CPU 核心数"
}

func (cpuLoadTool) Parameters() tools.ToolSchema {
	return tools.ToolSchema{Type: "object", Properties: map[string]tools.Property{}}
}

func (cpuLoadTool) Execute(_ context.Context, _ map[string]any) (map[string]any, error) {
	cores := runtime.NumCPU()
	load1, load5, load15, err := readLoadAvg()
	if err != nil {
		// Windows 或 /proc 不可用时返回核心数信息
		return map[string]any{
			"cores": cores,
			"note":  "load average not available on this platform",
		}, nil
	}
	return map[string]any{
		"load1":  load1,
		"load5":  load5,
		"load15": load15,
		"cores":  cores,
	}, nil
}

func readLoadAvg() (float64, float64, float64, error) {
	f, err := os.Open("/proc/loadavg")
	if err != nil {
		return 0, 0, 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		return 0, 0, 0, fmt.Errorf("empty /proc/loadavg")
	}
	fields := strings.Fields(scanner.Text())
	if len(fields) < 3 {
		return 0, 0, 0, fmt.Errorf("unexpected /proc/loadavg format")
	}
	load1, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, 0, 0, err
	}
	load5, err := strconv.ParseFloat(fields[1], 64)
	if err != nil {
		return 0, 0, 0, err
	}
	load15, err := strconv.ParseFloat(fields[2], 64)
	if err != nil {
		return 0, 0, 0, err
	}
	return load1, load5, load15, nil
}
