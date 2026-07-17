package runtime

import (
	"context"
	"os/user"
	"testing"
	"time"
)

func TestLocalCapsule_ExecutesCommand(t *testing.T) {
	exec, err := NewLocalCapsuleExecutor(LocalCapsuleConfig{
		RunUser:          testRunUser(t),
		WorkspaceRoot:    t.TempDir(),
		MaxExecutionTime: 5 * time.Second,
		MaxOutputBytes:   1024 * 1024,
	})
	if err != nil {
		t.Fatalf("failed to create executor: %v", err)
	}
	call := ToolCall{Name: "shell.exec", Args: map[string]any{"command": "echo hello"}}
	output, err := exec.Execute(context.Background(), call)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if output.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", output.ExitCode)
	}
	if output.Stdout != "hello\n" {
		t.Fatalf("expected 'hello\\n', got %q", output.Stdout)
	}
}

func TestLocalCapsule_Timeout(t *testing.T) {
	exec, err := NewLocalCapsuleExecutor(LocalCapsuleConfig{
		RunUser:          testRunUser(t),
		WorkspaceRoot:    t.TempDir(),
		MaxExecutionTime: 100 * time.Millisecond,
		MaxOutputBytes:   1024 * 1024,
	})
	if err != nil {
		t.Fatalf("failed to create executor: %v", err)
	}
	call := ToolCall{Name: "shell.exec", Args: map[string]any{"command": "sleep 10"}}
	_, err = exec.Execute(context.Background(), call)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestLocalCapsule_OutputTruncated(t *testing.T) {
	exec, err := NewLocalCapsuleExecutor(LocalCapsuleConfig{
		RunUser:          testRunUser(t),
		WorkspaceRoot:    t.TempDir(),
		MaxExecutionTime: 5 * time.Second,
		MaxOutputBytes:   100,
	})
	if err != nil {
		t.Fatalf("failed to create executor: %v", err)
	}
	call := ToolCall{Name: "shell.exec", Args: map[string]any{"command": "python3 -c \"print('x'*1000)\""}}
	output, err := exec.Execute(context.Background(), call)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if !output.Truncated {
		t.Fatal("expected output to be truncated")
	}
}

func TestLocalCapsule_FiltersSensitiveEnv(t *testing.T) {
	exec, err := NewLocalCapsuleExecutor(LocalCapsuleConfig{
		RunUser:          testRunUser(t),
		WorkspaceRoot:    t.TempDir(),
		MaxExecutionTime: 5 * time.Second,
		MaxOutputBytes:   1024 * 1024,
	})
	if err != nil {
		t.Fatalf("failed to create executor: %v", err)
	}
	call := ToolCall{Name: "shell.exec", Args: map[string]any{"command": "env"}}
	output, err := exec.Execute(context.Background(), call)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	sensitiveVars := []string{"API_KEY", "SECRET", "TOKEN", "PASSWORD"}
	for _, v := range sensitiveVars {
		if contains(output.Stdout, v+"=") {
			t.Fatalf("output should not contain sensitive env var: %s", v)
		}
	}
}

func testRunUser(t *testing.T) string {
	t.Helper()
	if current, err := user.Current(); err == nil && current.Username != "" {
		return current.Username
	}
	if u, err := user.Lookup("nobody"); err == nil {
		return u.Username
	}
	t.Fatal("current user cannot be resolved")
	return ""
}
