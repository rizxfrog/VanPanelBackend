package runtime

import "time"

// SafeResult 安全的工具执行结果
type SafeResult struct {
	Blocked      bool
	Pending      bool
	ApprovalID   string
	Output       string
	Untrusted    bool
	Reason       string
	MemCandidate *MemoryCandidate
	Duration     time.Duration
}

// CapsuleOutput 胶囊执行器的原始输出
type CapsuleOutput struct {
	Stdout    string
	Stderr    string
	ExitCode  int
	Duration  time.Duration
	Truncated bool
}

// ExecutionConstraints 执行约束
type ExecutionConstraints struct {
	MaxExecutionTime time.Duration
	MaxOutputBytes   int
	AllowedPaths     []string
	DeniedPaths      []string
	NetworkAccess    bool
}

// FirewallDecision 防火墙审查决策
type FirewallDecision struct {
	Allowed bool
	Action  string
	Reason  string
	Details map[string]any
}

// MemoryCandidate 记忆写入候选
type MemoryCandidate struct {
	Content      string
	Source       string
	MemoryType   string
	TTL          time.Duration
	Approved     bool
	RejectReason string
}
