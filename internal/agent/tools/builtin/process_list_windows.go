//go:build windows

package builtin

import (
	"os/exec"
	"strconv"
	"strings"
)

func platformProcessList() ([]procInfo, error) {
	// 使用 PowerShell 获取进程列表，按内存排序
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command",
		`Get-Process | Sort-Object -Property WS -Descending | Select-Object -First 50 Id, ProcessName, WS | ConvertTo-CSV -NoTypeInformation`)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return parsePSProcessList(string(out))
}

func parsePSProcessList(csv string) ([]procInfo, error) {
	lines := strings.Split(strings.TrimSpace(csv), "\n")
	if len(lines) < 2 {
		return nil, nil
	}

	var procs []procInfo
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// CSV 格式: "Id","ProcessName","WS"
		fields := strings.Split(line, ",")
		if len(fields) < 3 {
			continue
		}
		pidStr := strings.Trim(fields[0], "\"")
		name := strings.Trim(fields[1], "\"")
		wsStr := strings.Trim(fields[2], "\"")

		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			continue
		}
		ws, err := strconv.ParseUint(wsStr, 10, 64)
		if err != nil {
			continue
		}
		procs = append(procs, procInfo{PID: pid, Name: name, RSSBytes: ws})
	}
	return procs, nil
}
