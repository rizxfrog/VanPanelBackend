package runtime

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"go.uber.org/zap"
)

// PolicyDecision 策略决策
type PolicyDecision struct {
	Allowed          bool
	RequiresApproval bool
	Reason           string
	Constraints      *ExecutionConstraints
}

// PolicyRule 策略规则接口
type PolicyRule interface {
	Evaluate(ctx context.Context, call ToolCall) (*PolicyDecision, error)
	Priority() int
	Name() string
}

// PolicyEngine 策略引擎
type PolicyEngine struct {
	rules  []PolicyRule
	logger *zap.Logger
}

// NewPolicyEngine 创建策略引擎
func NewPolicyEngine(logger *zap.Logger) *PolicyEngine {
	if logger == nil {
		logger = zap.NewNop()
	}
	pe := &PolicyEngine{logger: logger}
	// 注册内置规则
	pe.rules = []PolicyRule{
		&dangerousCommandRule{},
		&protectedPathRule{},
		&approvalToolRule{},
	}
	return pe
}

// Evaluate 评估工具调用
func (pe *PolicyEngine) Evaluate(ctx context.Context, call ToolCall) *PolicyDecision {
	for _, rule := range pe.rules {
		decision, err := rule.Evaluate(ctx, call)
		if err != nil {
			pe.logger.Warn("策略规则执行失败", zap.String("rule", rule.Name()), zap.Error(err))
			continue
		}
		if !decision.Allowed || decision.RequiresApproval {
			return decision
		}
	}
	return &PolicyDecision{Allowed: true}
}

// dangerousCommandRule 危险命令规则
type dangerousCommandRule struct{}

func (r *dangerousCommandRule) Name() string { return "dangerous_command" }
func (r *dangerousCommandRule) Priority() int {
	return 0
}
func (r *dangerousCommandRule) Evaluate(ctx context.Context, call ToolCall) (*PolicyDecision, error) {
	if call.Name != "shell.exec" {
		return &PolicyDecision{Allowed: true}, nil
	}
	cmd, _ := call.Args["command"].(string)
	if strings.TrimSpace(cmd) == "" {
		return &PolicyDecision{Allowed: true}, nil
	}
	dangerous := regexp.MustCompile(`(?i)(\brm\s+-rf\s+/|\bdd\s+if=|\bmkfs\.|\bshutdown\b|\breboot\b|:\(\)\{\:\|\:&\};:)`)
	if dangerous.MatchString(cmd) {
		return &PolicyDecision{
			Allowed: false,
			Reason:  "危险命令被拦截",
		}, nil
	}
	return &PolicyDecision{Allowed: true}, nil
}

// protectedPathRule 受保护路径规则
type protectedPathRule struct{}

func (r *protectedPathRule) Name() string { return "protected_path" }
func (r *protectedPathRule) Priority() int {
	return 1
}
func (r *protectedPathRule) Evaluate(ctx context.Context, call ToolCall) (*PolicyDecision, error) {
	protectedPaths := []string{"/boot", "/etc", "/root", "/usr", "/var/lib/docker"}
	path, _ := call.Args["path"].(string)
	if path == "" {
		path, _ = call.Args["command"].(string)
	}
	if path == "" {
		return &PolicyDecision{Allowed: true}, nil
	}
	for _, p := range protectedPaths {
		if isProtectedPath(path, p) {
			return &PolicyDecision{
				Allowed: false,
				Reason:  fmt.Sprintf("受保护路径: %s", p),
			}, nil
		}
	}
	return &PolicyDecision{Allowed: true}, nil
}

// approvalToolRule 需审批工具规则
type approvalToolRule struct{}

func (r *approvalToolRule) Name() string { return "approval_tool" }
func (r *approvalToolRule) Priority() int {
	return 2
}
func (r *approvalToolRule) Evaluate(ctx context.Context, call ToolCall) (*PolicyDecision, error) {
	approvalTools := map[string]bool{
		"container.restart": true, "container.stop": true,
		"service.restart": true, "file.delete": true,
	}
	if approvalTools[call.Name] {
		return &PolicyDecision{
			Allowed:          true,
			RequiresApproval: true,
			Reason:           "需要用户审批",
		}, nil
	}
	return &PolicyDecision{Allowed: true}, nil
}

func isProtectedPath(path, protected string) bool {
	clean := filepath.Clean(path)
	p := filepath.Clean(protected)
	return clean == p || strings.HasPrefix(clean, p+"/")
}
