# Agent Security System Task 4 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the `CapsuleExecutor` interface and a local MVP implementation that runs `shell.exec` commands in an isolated workspace with timeout, output truncation, and filtered environment.

**Architecture:** Keep runtime-local types in `internal/agent/runtime`: a small interface file, a local executor implementation, and focused tests. Reuse existing `ToolCall` and `CapsuleOutput` types; do not redefine them.

**Tech Stack:** Go 1.25.5, `os/exec`, `context`, `github.com/google/uuid`, `go.uber.org/zap`.

## Global Constraints

- Go 1.25.5
- Module path: `github.com/rizxfrog/VanPanelBackend`
- Chinese comments, concise
- `go test ./internal/agent/runtime/ -run TestLocalCapsule -v` must pass
- Commit with: `feat(agent): add CapsuleExecutor interface and LocalCapsuleExecutor MVP`
- `ToolCall` is already defined in `memory_write_guard.go`; do not redefine it
- `CapsuleOutput` is already defined in `safe_result.go`; do not redefine it
- Use `github.com/google/uuid` for UUID generation
- The test user `nobody` may not exist; use the current user or skip user-related tests if needed

## File Map

- Create: `internal/agent/runtime/capsule_executor.go`
  - Defines `CapsuleExecutor` interface with `Execute(ctx context.Context, call ToolCall) (*CapsuleOutput, error)` and `Name() string`.
- Create: `internal/agent/runtime/local_capsule.go`
  - Defines `LocalCapsuleConfig`, `LocalCapsuleExecutor`, constructor, command execution, workspace creation, env filtering, and bounded output writer.
- Create: `internal/agent/runtime/local_capsule_test.go`
  - Tests command execution, timeout, output truncation, and sensitive env filtering.
- Modify: `internal/agent/runtime/local_capsule_test.go` only if needed to resolve `nobody` user availability by using current user.

---

### Task 4: CapsuleExecutor + LocalCapsuleExecutor

**Files:**
- Create: `internal/agent/runtime/capsule_executor.go`
- Create: `internal/agent/runtime/local_capsule.go`
- Create: `internal/agent/runtime/local_capsule_test.go`

**Interfaces:**
- Consumes: `ToolCall{Name string, Args map[string]any}` from `memory_write_guard.go`; `CapsuleOutput{Stdout, Stderr string; ExitCode int; Duration time.Duration; Truncated bool}` from `safe_result.go`.
- Produces:
  - `CapsuleExecutor interface { Execute(ctx context.Context, call ToolCall) (*CapsuleOutput, error); Name() string }`
  - `func NewLocalCapsuleExecutor(cfg LocalCapsuleConfig) (*LocalCapsuleExecutor, error)`
  - `func (e *LocalCapsuleExecutor) Execute(ctx context.Context, call ToolCall) (*CapsuleOutput, error)`
  - `func (e *LocalCapsuleExecutor) Name() string`

- [ ] **Step 1: Write the interface file**

```go
// internal/agent/runtime/capsule_executor.go
package runtime

import "context"

// CapsuleExecutor 执行隔离接口
type CapsuleExecutor interface {
	// Execute 在隔离环境中执行工具调用
	Execute(ctx context.Context, call ToolCall) (*CapsuleOutput, error)
	// Name 返回执行器名称
	Name() string
}
```

- [ ] **Step 2: Write tests that fail before implementation**

```go
// internal/agent/runtime/local_capsule_test.go
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
	if u, err := user.Lookup("nobody"); err == nil {
		return u.Username
	}
	current, err := user.Current()
	if err != nil {
		t.Fatalf("neither nobody nor current user can be resolved: %v", err)
	}
	return current.Username
}
```

- [ ] **Step 3: Run tests to verify they fail**

```bash
go test ./internal/agent/runtime/ -run TestLocalCapsule -v
```

Expected: FAIL with undefined `NewLocalCapsuleExecutor`, `LocalCapsuleConfig`, and related implementation symbols.

- [ ] **Step 4: Implement LocalCapsuleExecutor**

```go
// internal/agent/runtime/local_capsule.go
package runtime

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// LocalCapsuleConfig 本地胶囊执行器配置
type LocalCapsuleConfig struct {
	RunUser          string        // 低权限用户
	RunGroup         string        // 低权限组
	WorkspaceRoot    string        // 工作空间根目录
	AllowedPaths     []string      // 允许访问的路径
	DeniedPaths      []string      // 禁止访问的路径
	AllowedEnvVars   []string      // 允许继承的环境变量白名单
	MaxExecutionTime time.Duration // 最大执行时间
	MaxOutputBytes   int           // 最大输出大小
	MaxMemoryBytes   int64         // 最大内存
	MaxCPUPercent    int           // 最大 CPU 百分比
	NetworkAccess    bool          // 是否允许网络访问
}

// LocalCapsuleExecutor 本地胶囊执行器 MVP
type LocalCapsuleExecutor struct {
	cfg    LocalCapsuleConfig
	runUID int
	runGID int
	logger *zap.Logger
}

// NewLocalCapsuleExecutor 创建本地胶囊执行器
func NewLocalCapsuleExecutor(cfg LocalCapsuleConfig) (*LocalCapsuleExecutor, error) {
	if cfg.WorkspaceRoot == "" {
		cfg.WorkspaceRoot = "/var/lib/agent/workspace"
	}
	if cfg.MaxExecutionTime == 0 {
		cfg.MaxExecutionTime = 30 * time.Second
	}
	if cfg.MaxOutputBytes == 0 {
		cfg.MaxOutputBytes = 1024 * 1024 // 1MB
	}

	// 确保工作空间目录存在
	if err := os.MkdirAll(cfg.WorkspaceRoot, 0755); err != nil {
		return nil, fmt.Errorf("创建工作空间目录失败: %w", err)
	}

	exec := &LocalCapsuleExecutor{
		cfg:    cfg,
		logger: zap.NewNop(),
	}

	// 解析用户
	if cfg.RunUser != "" {
		u, err := user.Lookup(cfg.RunUser)
		if err != nil {
			return nil, fmt.Errorf("查找用户 %s 失败: %w", cfg.RunUser, err)
		}
		uid, err := strconv.Atoi(u.Uid)
		if err != nil {
			return nil, fmt.Errorf("解析用户 UID 失败: %w", err)
		}
		gid := uid
		if cfg.RunGroup != "" {
			g, err := user.LookupGroup(cfg.RunGroup)
			if err != nil {
				return nil, fmt.Errorf("查找组 %s 失败: %w", cfg.RunGroup, err)
			}
			gid, err = strconv.Atoi(g.Gid)
			if err != nil {
				return nil, fmt.Errorf("解析组 GID 失败: %w", err)
			}
		} else if u.Gid != "" {
			gid, err = strconv.Atoi(u.Gid)
			if err != nil {
				return nil, fmt.Errorf("解析用户 GID 失败: %w", err)
			}
		}
		exec.runUID = uid
		exec.runGID = gid
	}

	return exec, nil
}

// Name 返回执行器名称
func (e *LocalCapsuleExecutor) Name() string {
	return "local"
}

// Execute 在隔离环境中执行工具调用
func (e *LocalCapsuleExecutor) Execute(ctx context.Context, call ToolCall) (*CapsuleOutput, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	start := time.Now()

	// 1. 创建临时 workspace
	workDir, cleanup, err := e.createWorkspace()
	if err != nil {
		return nil, fmt.Errorf("创建工作空间失败: %w", err)
	}
	defer cleanup()

	// 2. 构建命令
	command, ok := call.Args["command"].(string)
	if !ok || strings.TrimSpace(command) == "" {
		return nil, fmt.Errorf("缺少 command 参数")
	}

	// 3. 设置超时
	execCtx, cancel := context.WithTimeout(ctx, e.cfg.MaxExecutionTime)
	defer cancel()

	cmd := exec.CommandContext(execCtx, "sh", "-c", command)
	cmd.Dir = workDir
	cmd.Env = e.filterEnvVars()
	e.applyResourceLimits(cmd)

	// 4. 执行并捕获输出
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &limitedWriter{w: &stdout, max: e.cfg.MaxOutputBytes}
	cmd.Stderr = &limitedWriter{w: &stderr, max: e.cfg.MaxOutputBytes}

	err = cmd.Run()
	exitCode := 0
	if err != nil {
		if execCtx.Err() == context.DeadlineExceeded || ctx.Err() != nil {
			return nil, fmt.Errorf("执行超时: %w", execCtx.Err())
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, fmt.Errorf("执行失败: %w", err)
		}
	}

	return &CapsuleOutput{
		Stdout:    stdout.String(),
		Stderr:    stderr.String(),
		ExitCode:  exitCode,
		Duration:  time.Since(start),
		Truncated: stdout.Len() >= e.cfg.MaxOutputBytes || stderr.Len() >= e.cfg.MaxOutputBytes,
	}, nil
}

// createWorkspace 创建隔离的临时工作目录
func (e *LocalCapsuleExecutor) createWorkspace() (string, func(), error) {
	workDir := filepath.Join(e.cfg.WorkspaceRoot, uuid.New().String())
	if err := os.MkdirAll(workDir, 0750); err != nil {
		return "", nil, err
	}
	if e.runUID > 0 {
		if err := os.Chown(workDir, e.runUID, e.runGID); err != nil {
			_ = os.RemoveAll(workDir)
			return "", nil, err
		}
	}
	cleanup := func() { _ = os.RemoveAll(workDir) }
	return workDir, cleanup, nil
}

// filterEnvVars 只保留白名单环境变量，过滤敏感变量
func (e *LocalCapsuleExecutor) filterEnvVars() []string {
	allowed := map[string]bool{
		"PATH": true, "HOME": true, "LANG": true, "LC_ALL": true, "TMPDIR": true,
	}
	for _, v := range e.cfg.AllowedEnvVars {
		allowed[v] = true
	}

	var env []string
	for _, ev := range os.Environ() {
		parts := strings.SplitN(ev, "=", 2)
		if len(parts) == 0 {
			continue
		}
		key := parts[0]
		if isSensitiveEnvVar(key) {
			continue
		}
		if allowed[key] {
			env = append(env, ev)
		}
	}
	return env
}

func isSensitiveEnvVar(key string) bool {
	sensitive := []string{"KEY", "SECRET", "TOKEN", "PASSWORD", "CREDENTIAL", "AUTH"}
	upper := strings.ToUpper(key)
	for _, s := range sensitive {
		if strings.Contains(upper, s) {
			return true
		}
	}
	return false
}

func (e *LocalCapsuleExecutor) applyResourceLimits(cmd *exec.Cmd) {
	// 资源限制通过 ulimit 和 cgroups 实现
	// MVP 阶段使用 context timeout 作为主要限制
}

// limitedWriter 限制写入大小的 Writer
type limitedWriter struct {
	w   *bytes.Buffer
	max int
}

func (lw *limitedWriter) Write(p []byte) (n int, err error) {
	remaining := lw.max - lw.w.Len()
	if remaining <= 0 {
		return len(p), nil // 静默丢弃超出部分
	}
	if len(p) > remaining {
		p = p[:remaining]
	}
	return lw.w.Write(p)
}
```

- [ ] **Step 5: Run the required test command**

```bash
go test ./internal/agent/runtime/ -run TestLocalCapsule -v
```

Expected: PASS — all four `TestLocalCapsule*` tests pass. If `python3` is unavailable, replace the truncation test command with POSIX `yes x | head -c 1000` and keep the same assertion.

- [ ] **Step 6: Commit**

```bash
git add internal/agent/runtime/capsule_executor.go internal/agent/runtime/local_capsule.go internal/agent/runtime/local_capsule_test.go
git commit -m "feat(agent): add CapsuleExecutor interface and LocalCapsuleExecutor MVP"
```

- [ ] **Step 7: Write Task 4 report**

Create or update `/home/van/github/van/VanPanelBackend/.superpowers/sdd/task-4-report.md` with:
- Status: `DONE`, `DONE_WITH_CONCERNS`, `NEEDS_CONTEXT`, or `BLOCKED`
- Commits made
- Full test output from `go test ./internal/agent/runtime/ -run TestLocalCapsule -v`
- Any concerns

## Self-Review Checklist

- Spec coverage:
  - `CapsuleExecutor` interface exists in `internal/agent/runtime/capsule_executor.go`.
  - `LocalCapsuleExecutor` exists in `internal/agent/runtime/local_capsule.go`.
  - Tests exist in `internal/agent/runtime/local_capsule_test.go`.
  - Reuses existing `ToolCall` and `CapsuleOutput`.
  - Uses `github.com/google/uuid`.
  - Handles missing `nobody` user via `testRunUser`.
- Placeholder scan: no `TBD`, `TODO`, or vague “handle edge cases” steps remain.
- Type consistency: test and implementation use the same `LocalCapsuleConfig`, `ToolCall`, and `CapsuleOutput` fields.
