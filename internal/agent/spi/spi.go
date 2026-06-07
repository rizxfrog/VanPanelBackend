package spi

import "context"

// IntentResult 意图分析结果
type IntentResult struct {
	Intent      string   // 意图分类: query / inspect / execute / dangerous
	RiskTags    []string // 风险标签: prompt_injection / privilege_escalation
	Confidence  float64  // 置信度 0-1
	BlockReason string   // 如果不通过，拦截原因
}

// IntentAnalyzer 意图分析 + 注入防护
type IntentAnalyzer interface {
	Analyze(ctx context.Context, userInput string) (*IntentResult, error)
}

// MemoryEntry 记忆条目
type MemoryEntry struct {
	Source   string         // "short_term" / "long_term" / "rag"
	Content  string
	Score    float64        // 相关性评分
	Metadata map[string]any
}

// MemoryProvider 记忆提供者
type MemoryProvider interface {
	Retrieve(ctx context.Context, query string, sessionID string) ([]MemoryEntry, error)
}

// GuardDecision 审查决策
type GuardDecision struct {
	Allowed  bool
	Decision string // "approve" / "reject" / "needs_approval"
	Reason   string
}

// GuardRule 自定义安全规则，注入 GuardChain
type GuardRule interface {
	Evaluate(ctx context.Context, toolName string, toolArgs map[string]any) (*GuardDecision, error)
	Priority() int // 执行优先级，数字越小越先执行
}

// Notifier 告警通知通道
type Notifier interface {
	Send(ctx context.Context, title string, body string, level string) error
}

// AuditWriter 自定义审计日志格式
type AuditWriter interface {
	Write(ctx context.Context, event map[string]any) error
}

// ToolResolver 自定义工具发现机制
type ToolResolver interface {
	Resolve(ctx context.Context, toolName string) (any, error)
}
