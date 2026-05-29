package builtin

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// bashPaths Windows 上 Git Bash 的常见路径
var bashPaths = []string{
	`C:\Program Files\Git\bin\bash.exe`,
	`C:\Program Files (x86)\Git\bin\bash.exe`,
}

// findShell 返回当前平台的 shell 及其执行参数。
// 返回 (shellPath, shellFlag)，如 bash => -c, powershell.exe => -Command。
func findShell() (string, string, error) {
	if runtime.GOOS != "windows" {
		return "bash", "-c", nil
	}

	// Windows: 优先使用 PowerShell
	if p, err := exec.LookPath("powershell.exe"); err == nil {
		return p, "-NoProfile", nil
	}

	// 回退：Git Bash
	for _, p := range bashPaths {
		if _, err := os.Stat(p); err == nil {
			return p, "-c", nil
		}
	}
	if p, err := exec.LookPath("bash"); err == nil {
		return p, "-c", nil
	}

	// 最后回退：cmd
	if p, err := exec.LookPath("cmd.exe"); err == nil {
		return p, "/C", nil
	}

	return "", "", fmt.Errorf("未找到可用的 shell（请安装 Git for Windows 或 PowerShell）")
}

// baseCommandTool 是所有命令行工具的公共基础结构，
// 封装了跨平台命令执行、超时控制和输出截断逻辑。
type baseCommandTool struct {
	name           string
	description    string
	command        string // Linux 命令（bash -c "<command>"）
	windowsCommand string // Windows PowerShell 命令（可选；为空时返回 not-available 信息）
	timeout        time.Duration
	maxOutput      int
}

// runCommand 执行命令并返回截断后的标准输出。
// 根据运行平台自动选择 Linux (bash) 或 Windows (PowerShell) 命令。
func (t *baseCommandTool) runCommand(ctx context.Context, args string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()

	shell, shellFlag, err := findShell()
	if err != nil {
		return "", err
	}

	isWindows := runtime.GOOS == "windows"

	if isWindows && t.windowsCommand != "" {
		return t.runWindowsCommand(ctx, shell, args)
	} else if isWindows && t.windowsCommand == "" {
		return "", fmt.Errorf("当前工具 (%s) 不支持 Windows 平台，请在 Linux 环境使用", t.name)
	}

	// Linux bash 命令
	cmdLine := t.command
	if args != "" {
		cmdLine = t.command + " " + args
	}

	cmd := exec.CommandContext(ctx, shell, shellFlag, cmdLine)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("命令执行失败: %s: %w", stderr.String(), err)
	}

	output := stdout.String()
	if t.maxOutput > 0 && len(output) > t.maxOutput {
		output = output[:t.maxOutput] + "\n... (输出已截断)"
	}
	return output, nil
}

// runWindowsCommand 在 Windows 上使用 PowerShell 执行命令。
// 将每个 flag 和命令作为独立参数传给 exec.Cmd，避免 shell 解析错误。
func (t *baseCommandTool) runWindowsCommand(ctx context.Context, shell string, args string) (string, error) {
	cmdLine := t.windowsCommand
	if args != "" {
		cmdLine = strings.ReplaceAll(cmdLine, "{args}", args)
	} else {
		cmdLine = strings.ReplaceAll(cmdLine, "{args}", "")
	}

	// 使用 -NoProfile 加快启动，-Command 执行内联脚本
	cmd := exec.CommandContext(ctx, shell, "-NoProfile", "-Command", cmdLine)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("命令执行失败: %s: %w", stderr.String(), err)
	}

	output := stdout.String()
	if t.maxOutput > 0 && len(output) > t.maxOutput {
		output = output[:t.maxOutput] + "\n... (输出已截断)"
	}
	return output, nil
}
