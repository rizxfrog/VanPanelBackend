package builtin

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"time"
)

// baseCommandTool 是所有命令行工具的公共基础结构，
// 封装了跨平台命令执行、超时控制和输出截断逻辑。
type baseCommandTool struct {
	name        string
	description string
	command     string
	timeout     time.Duration
	maxOutput   int
}

// runCommand 在当前平台执行命令并返回截断后的标准输出。
// Linux 使用 bash -c，Windows 使用 powershell -Command。
func (t *baseCommandTool) runCommand(ctx context.Context, args string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		if args != "" {
			cmd = exec.CommandContext(ctx, "powershell", "-Command", t.command+" "+args)
		} else {
			cmd = exec.CommandContext(ctx, "powershell", "-Command", t.command)
		}
	} else {
		if args != "" {
			cmd = exec.CommandContext(ctx, "bash", "-c", t.command+" "+args)
		} else {
			cmd = exec.CommandContext(ctx, "bash", "-c", t.command)
		}
	}

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
