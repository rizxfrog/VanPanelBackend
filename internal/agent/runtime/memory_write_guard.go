package runtime

import (
	"strings"
	"time"

	"go.uber.org/zap"
)

// ToolCall 工具调用（本地定义，避免循环导入）
type ToolCall struct {
	Name string
	Args map[string]any
}

// MemoryWriteGuard 记忆写入安全守卫
type MemoryWriteGuard struct {
	logger *zap.Logger
}

// NewMemoryWriteGuard 创建记忆写入守卫
func NewMemoryWriteGuard(logger *zap.Logger) *MemoryWriteGuard {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &MemoryWriteGuard{logger: logger}
}

// Review 审查工具结果的记忆写入请求
func (g *MemoryWriteGuard) Review(call ToolCall, result *SanitizedResult) *MemoryCandidate {
	candidate := &MemoryCandidate{
		Content:    result.SafeContent,
		Source:     "tool_result",
		MemoryType: "short_term",
		TTL:        30 * time.Minute,
	}

	// 1. 检测到注入 — 直接拒绝
	if result.InjectDetected {
		candidate.Approved = false
		candidate.RejectReason = "工具结果包含指令型内容，禁止写入记忆"
		return candidate
	}

	// 2. 检测敏感信息 — 直接拒绝
	if containsSensitiveInfo(result.SafeContent) {
		candidate.Approved = false
		candidate.RejectReason = "工具结果包含敏感信息，禁止写入记忆"
		return candidate
	}

	// 3. 运维诊断结果 — 允许写短期记忆
	if isDiagnosticToolCall(call) {
		candidate.Approved = true
		candidate.MemoryType = "short_term"
		candidate.TTL = 30 * time.Minute
		return candidate
	}

	// 4. 其他工具结果 — 默认不写记忆
	candidate.Approved = false
	candidate.RejectReason = "工具结果默认不写入记忆"
	return candidate
}

func containsSensitiveInfo(content string) bool {
	sensitive := []string{
		"password", "secret", "token", "api_key", "private_key",
		"BEGIN PRIVATE KEY", "AKIA", "jwt", "bearer",
	}
	lower := strings.ToLower(content)
	for _, s := range sensitive {
		if strings.Contains(lower, s) {
			return true
		}
	}
	return false
}

func isDiagnosticToolCall(call ToolCall) bool {
	if !isDiagnosticToolName(call.Name) {
		return false
	}

	// 仅允许明确的运维诊断命令，避免把普通文件列表写入记忆。
	if call.Name == "shell.exec" {
		command, _ := call.Args["command"].(string)
		if command == "" {
			return false
		}
		if strings.Contains(command, "top") || strings.Contains(command, "ps") || strings.Contains(command, "free") || strings.Contains(command, "df") || strings.Contains(command, "du") {
			return true
		}
		return false
	}

	return true
}

func isDiagnosticToolName(name string) bool {
	diagnosticTools := []string{
		"shell.exec", "k8s.get", "k8s.describe", "k8s.logs",
		"svc.status", "monitor.query", "file.read",
	}
	for _, t := range diagnosticTools {
		if name == t {
			return true
		}
	}
	return false
}
