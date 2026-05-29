package builtin

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/GoSimplicity/AI-CloudOps/internal/agent/tools"
)

type memoryUsageTool struct{}

func MemoryUsageTool() tools.Tool { return memoryUsageTool{} }

func (memoryUsageTool) Name() string { return "system.memory.usage" }

func (memoryUsageTool) Description() string {
	return "获取系统内存使用情况：总内存、可用内存、已用内存、使用率"
}

func (memoryUsageTool) Parameters() tools.ToolSchema {
	return tools.ToolSchema{Type: "object", Properties: map[string]tools.Property{}}
}

func (memoryUsageTool) Execute(_ context.Context, _ map[string]any) (map[string]any, error) {
	// Go 进程内存
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	result := map[string]any{
		"goHeapAlloc": m.HeapAlloc,
		"goHeapInuse": m.HeapInuse,
		"goSys":       m.Sys,
		"goNumGC":     m.NumGC,
	}

	// 系统内存
	total, available, err := readSystemMemory()
	if err != nil {
		result["note"] = "system memory info not available on this platform"
		return result, nil
	}

	used := total - available
	var percent float64
	if total > 0 {
		percent = float64(used) / float64(total) * 100
	}
	result["totalBytes"] = total
	result["availableBytes"] = available
	result["usedBytes"] = used
	result["usedPercent"] = fmt.Sprintf("%.1f%%", percent)
	return result, nil
}

func readSystemMemory() (total, available uint64, err error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()

	var memTotal, memAvailable uint64
	found := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() && found < 2 {
		line := scanner.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			memTotal, err = parseMemInfoKB(line)
			if err != nil {
				return 0, 0, err
			}
			found++
		} else if strings.HasPrefix(line, "MemAvailable:") {
			memAvailable, err = parseMemInfoKB(line)
			if err != nil {
				return 0, 0, err
			}
			found++
		}
	}
	if memTotal == 0 {
		return 0, 0, fmt.Errorf("MemTotal not found in /proc/meminfo")
	}
	return memTotal * 1024, memAvailable * 1024, nil
}

func parseMemInfoKB(line string) (uint64, error) {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return 0, fmt.Errorf("unexpected meminfo line: %s", line)
	}
	val, err := strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		return 0, err
	}
	return val, nil
}
