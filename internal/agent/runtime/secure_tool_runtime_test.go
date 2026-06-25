package runtime

import (
	"context"
	"testing"
	"time"
)

func TestSecureToolRuntime_ExecutesSafeCommand(t *testing.T) {
	runtime := newTestRuntime(t)
	call := ToolCall{Name: "shell.exec", Args: map[string]any{"command": "echo hello"}}
	result, err := runtime.Execute(context.Background(), "session-1", call)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if result.Blocked {
		t.Fatal("safe command should not be blocked")
	}
	if result.Output == "" {
		t.Fatal("expected output")
	}
}

func TestSecureToolRuntime_BlocksDangerousCommand(t *testing.T) {
	runtime := newTestRuntime(t)
	call := ToolCall{Name: "shell.exec", Args: map[string]any{"command": "rm -rf /"}}
	result, err := runtime.Execute(context.Background(), "session-1", call)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if !result.Blocked {
		t.Fatal("dangerous command should be blocked")
	}
}

func TestSecureToolRuntime_SanitizesOutput(t *testing.T) {
	runtime := newTestRuntime(t)
	// echo 命令输出不会包含敏感信息，但验证管道正常工作
	call := ToolCall{Name: "shell.exec", Args: map[string]any{"command": "echo 'safe output'"}}
	result, err := runtime.Execute(context.Background(), "session-1", call)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if result.Untrusted != true {
		t.Fatal("result should be marked as untrusted")
	}
}

func newTestRuntime(t *testing.T) *SecureToolRuntime {
	t.Helper()
	exec, err := NewLocalCapsuleExecutor(LocalCapsuleConfig{
		RunUser:          "nobody",
		WorkspaceRoot:    "/tmp/agent-test-runtime",
		MaxExecutionTime: 5 * time.Second,
		MaxOutputBytes:   1024 * 1024,
	})
	if err != nil {
		t.Fatalf("failed to create executor: %v", err)
	}
	return NewSecureToolRuntime(
		nil, // guardChain
		NewPolicyEngine(nil),
		NewApprovalManager(true),
		exec,
		NewToolResultSanitizer(nil),
		NewMemoryWriteGuard(nil),
		nil,
	)
}
