package runtime

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// SecureToolRuntime 安全工具执行运行时
// 调用链: GuardChain → PolicyEngine → ApprovalManager → CapsuleExecutor → ToolResultSanitizer → MemoryWriteGuard
type SecureToolRuntime struct {
	guardChain *guardChainAdapter
	policy     *PolicyEngine
	approver   *ApprovalManager
	executor   CapsuleExecutor
	sanitizer  *ToolResultSanitizer
	memGuard   *MemoryWriteGuard
	logger     *zap.Logger
}

// guardChainAdapter 适配 guard.Chain 接口
type guardChainAdapter struct {
	evaluateFunc func(ctx context.Context, toolName string, toolArgs map[string]any) *PolicyDecision
}

func (a *guardChainAdapter) Evaluate(ctx context.Context, toolName string, toolArgs map[string]any) *PolicyDecision {
	if a == nil || a.evaluateFunc == nil {
		return &PolicyDecision{Allowed: true}
	}
	return a.evaluateFunc(ctx, toolName, toolArgs)
}

// NewSecureToolRuntime 创建安全工具运行时
func NewSecureToolRuntime(
	guardChainEvaluate func(ctx context.Context, toolName string, toolArgs map[string]any) *PolicyDecision,
	policy *PolicyEngine,
	approver *ApprovalManager,
	executor CapsuleExecutor,
	sanitizer *ToolResultSanitizer,
	memGuard *MemoryWriteGuard,
	logger *zap.Logger,
) (*SecureToolRuntime, error) {
	if guardChainEvaluate == nil {
		return nil, fmt.Errorf("guardChainEvaluate is required")
	}
	if policy == nil {
		return nil, fmt.Errorf("policy is required")
	}
	if approver == nil {
		return nil, fmt.Errorf("approver is required")
	}
	if executor == nil {
		return nil, fmt.Errorf("executor is required")
	}
	if sanitizer == nil {
		return nil, fmt.Errorf("sanitizer is required")
	}
	if memGuard == nil {
		return nil, fmt.Errorf("memGuard is required")
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &SecureToolRuntime{
		guardChain: &guardChainAdapter{evaluateFunc: guardChainEvaluate},
		policy:     policy,
		approver:   approver,
		executor:   executor,
		sanitizer:  sanitizer,
		memGuard:   memGuard,
		logger:     logger,
	}, nil
}

// Execute 执行安全的工具调用管道
func (r *SecureToolRuntime) Execute(ctx context.Context, sessionID string, call ToolCall) (*SafeResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	start := time.Now()

	// 1. GuardChain 审查
	decision := r.guardChain.Evaluate(ctx, call.Name, call.Args)
	if !decision.Allowed {
		return &SafeResult{
			Blocked:  true,
			Reason:   decision.Reason,
			Duration: time.Since(start),
		}, nil
	}

	// 2. PolicyEngine 审查
	policyDecision := r.policy.Evaluate(ctx, call)
	if !policyDecision.Allowed {
		return &SafeResult{
			Blocked:  true,
			Reason:   policyDecision.Reason,
			Duration: time.Since(start),
		}, nil
	}

	// 3. 审批判断
	if policyDecision.RequiresApproval {
		approval, err := r.approver.Request(ctx, sessionID, call, policyDecision)
		if err != nil {
			return nil, fmt.Errorf("审批请求失败: %w", err)
		}
		switch approval.Status {
		case "rejected":
			return &SafeResult{
				Blocked:  true,
				Reason:   "用户拒绝审批",
				Duration: time.Since(start),
			}, nil
		case "pending":
			return &SafeResult{
				Pending:    true,
				ApprovalID: approval.ID,
				Duration:   time.Since(start),
			}, nil
		}
	}

	// 4. CapsuleExecutor 隔离执行
	rawOutput, err := r.executor.Execute(ctx, call)
	if err != nil {
		return nil, fmt.Errorf("工具执行失败: %w", err)
	}

	// 5. ToolResultSanitizer 脱敏 + 注入检测
	sanitized := r.sanitizer.Sanitize(ctx, rawOutput.Stdout+rawOutput.Stderr)

	// 6. MemoryWriteGuard 记忆写入审查
	memCandidate := r.memGuard.Review(call, sanitized)

	return &SafeResult{
		Output:       sanitized.SafeContent,
		Untrusted:    true,
		MemCandidate: memCandidate,
		Duration:     time.Since(start),
	}, nil
}
