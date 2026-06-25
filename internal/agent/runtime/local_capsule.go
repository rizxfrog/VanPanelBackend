package runtime

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
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
		if errors.Is(err, io.ErrShortWrite) {
			err = nil
		} else if exitErr, ok := err.(*exec.ExitError); ok {
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
	if e.runUID > 0 && e.runUID != os.Getuid() {
		if err := os.Chown(workDir, e.runUID, e.runGID); err != nil {
			// 当前进程无权限切换用户时，保留隔离目录并继续执行。
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
	if _, err = lw.w.Write(p); err != nil {
		return 0, err
	}
	return len(p), nil
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
