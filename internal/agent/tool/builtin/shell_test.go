package builtin

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestRunCommand_Windows_ProcTop(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("只在 Windows 上运行")
	}

	tool := NewTopTool()
	ctx := context.Background()

	output, err := tool.runCommand(ctx, "")
	if err != nil {
		t.Fatalf("proc.top 执行失败: %v", err)
	}
	if output == "" {
		t.Fatal("proc.top 输出为空")
	}
	t.Logf("proc.top 输出:\n%s", output)

	// 基本验证
	if !strings.Contains(output, "CPU") {
		t.Log("警告: 输出未包含 'CPU'")
	}
}

func TestRunCommand_Windows_ProcPS(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("只在 Windows 上运行")
	}

	tool := NewPSTool()
	ctx := context.Background()

	output, err := tool.runCommand(ctx, "")
	if err != nil {
		t.Fatalf("proc.ps 执行失败: %v", err)
	}
	if output == "" {
		t.Fatal("proc.ps 输出为空")
	}
	t.Logf("proc.ps 输出:\n%s", output)
}

func TestRunCommand_Windows_SysFree(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("只在 Windows 上运行")
	}

	tool := NewFreeTool()
	ctx := context.Background()

	output, err := tool.runCommand(ctx, "")
	if err != nil {
		t.Fatalf("sys.free 执行失败: %v", err)
	}
	if output == "" {
		t.Fatal("sys.free 输出为空")
	}
	t.Logf("sys.free 输出:\n%s", output)

	if !strings.Contains(output, "G") && !strings.Contains(output, "MB") {
		t.Log("警告: 输出未包含内存信息")
	}
}

func TestRunCommand_Windows_DiskDF(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("只在 Windows 上运行")
	}

	tool := NewDFTool()
	ctx := context.Background()

	output, err := tool.runCommand(ctx, "")
	if err != nil {
		t.Fatalf("disk.df 执行失败: %v", err)
	}
	if output == "" {
		t.Fatal("disk.df 输出为空")
	}
	t.Logf("disk.df 输出:\n%s", output)
}

func TestRunCommand_Windows_SvcUptime(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("只在 Windows 上运行")
	}

	tool := NewUptimeTool()
	ctx := context.Background()

	output, err := tool.runCommand(ctx, "")
	if err != nil {
		t.Fatalf("svc.uptime 执行失败: %v", err)
	}
	if output == "" {
		t.Fatal("svc.uptime 输出为空")
	}
	t.Logf("svc.uptime 输出:\n%s", output)

	if !strings.Contains(output, "day") && !strings.Contains(output, "hour") && !strings.Contains(output, "minute") {
		t.Log("警告: 输出不符合预期格式")
	}
}

func TestRunCommand_Windows_ShellExec(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("只在 Windows 上运行")
	}

	tool := NewShellExecTool()
	ctx := context.Background()

	// 测试简单的 PowerShell 命令
	output, err := tool.runCommand(ctx, "Write-Output 'hello from powershell'")
	if err != nil {
		t.Fatalf("shell.exec 执行失败: %v", err)
	}
	t.Logf("shell.exec 输出: %s", output)

	if !strings.Contains(output, "hello from powershell") {
		t.Errorf("预期输出包含 'hello from powershell'，实际: %s", output)
	}
}

func TestRunCommand_Windows_NetNetstat(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("只在 Windows 上运行")
	}

	tool := NewNetstatTool()
	ctx := context.Background()

	output, err := tool.runCommand(ctx, "")
	if err != nil {
		t.Fatalf("net.netstat 执行失败: %v", err)
	}
	if output == "" {
		t.Fatal("net.netstat 输出为空")
	}
	t.Logf("net.netstat 输出:\n%s", output)
}

func TestRunCommand_Windows_LogTail(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("只在 Windows 上运行")
	}

	tool := NewTailTool()
	ctx := context.Background()

	// 测试读取一个存在的文件
	output, err := tool.runCommand(ctx, "C:\\Windows\\System32\\drivers\\etc\\hosts")
	if err != nil {
		t.Fatalf("log.tail 执行失败: %v", err)
	}
	if output == "" {
		t.Fatal("log.tail 输出为空")
	}
	t.Logf("log.tail 输出:\n%s", output)
}

func TestRunCommand_Windows_SysUname(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("只在 Windows 上运行")
	}

	tool := NewUnameTool()
	ctx := context.Background()

	output, err := tool.runCommand(ctx, "")
	if err != nil {
		t.Fatalf("sys.uname 执行失败: %v", err)
	}
	if output == "" {
		t.Fatal("sys.uname 输出为空")
	}
	t.Logf("sys.uname 输出:\n%s", output)
}

func TestRunCommandTimeout(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("只在 Windows 上运行")
	}

	tool := NewShellExecTool()
	tool.timeout = 100 * time.Millisecond
	ctx := context.Background()

	_, err := tool.runCommand(ctx, "Start-Sleep -Seconds 10")
	if err == nil {
		t.Fatal("预期超时错误，但命令执行成功")
	}
	t.Logf("超时错误（符合预期）: %v", err)
}
