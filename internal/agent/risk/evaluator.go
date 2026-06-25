package risk

import (
	"regexp"
	"strings"
)

// RiskLevel represents the severity level of a risk evaluation.
type RiskLevel string

const (
	RiskLevelSafe RiskLevel = "safe"
	RiskLevelLow  RiskLevel = "low"
	RiskLevelHigh RiskLevel = "high"
)

// EvalResult holds the result of a risk evaluation.
type EvalResult struct {
	Level   RiskLevel
	Reason  string
	Blocked bool
}

// ShellConfig holds shell-specific risk configuration.
type ShellConfig struct {
	DefaultRisk string
	Blacklist   []string
	Whitelist   []string
}

// EvaluatorConfig holds the risk evaluator configuration.
// This mirrors di.AgentRiskConfig but is defined locally to avoid import cycles.
type EvaluatorConfig struct {
	HighRiskPatterns  []string
	ProtectedPaths    []string
	Shell             ShellConfig
	DangerousCommands []string
	ProtectedServices []string
	ApprovalTools     []string
}

// Evaluator evaluates tool calls for risk based on configurable patterns and rules.
type Evaluator struct {
	highRiskPatterns  []*regexp.Regexp
	protectedPaths    []string
	shellBlacklist    []*regexp.Regexp
	shellWhitelist    []*regexp.Regexp
	dangerousCommands []*regexp.Regexp
	protectedServices []string
	approvalTools     []string
}

// NewEvaluator creates an Evaluator from the given configuration.
func NewEvaluator(cfg *EvaluatorConfig) *Evaluator {
	e := &Evaluator{
		protectedPaths:    cfg.ProtectedPaths,
		protectedServices: cfg.ProtectedServices,
		approvalTools:     cfg.ApprovalTools,
	}
	for _, p := range cfg.HighRiskPatterns {
		if re, err := regexp.Compile(p); err == nil {
			e.highRiskPatterns = append(e.highRiskPatterns, re)
		}
	}
	for _, p := range cfg.DangerousCommands {
		if re, err := regexp.Compile(p); err == nil {
			e.dangerousCommands = append(e.dangerousCommands, re)
		}
	}
	for _, p := range cfg.Shell.Blacklist {
		if re, err := regexp.Compile(p); err == nil {
			e.shellBlacklist = append(e.shellBlacklist, re)
		}
	}
	for _, p := range cfg.Shell.Whitelist {
		if re, err := regexp.Compile(p); err == nil {
			e.shellWhitelist = append(e.shellWhitelist, re)
		}
	}
	return e
}

func (e *Evaluator) Evaluate(toolName string, args string) EvalResult {
	if toolName != "shell.exec" {
		return e.evaluateNamedTool(toolName, args)
	}
	return e.evaluateShell(args)
}

func (e *Evaluator) evaluateNamedTool(name string, args string) EvalResult {
	switch {
	case name == "terminal.suggest":
		for _, re := range e.dangerousCommands {
			if re.MatchString(args) {
				return EvalResult{Level: RiskLevelHigh, Reason: "危险命令被拦截", Blocked: true}
			}
		}
	case name == "file.delete", name == "file.move_to_trash":
		for _, p := range e.protectedPaths {
			if strings.Contains(args, p) {
				return EvalResult{Level: RiskLevelHigh, Reason: "受保护路径: " + p, Blocked: true}
			}
		}
	case name == "service.restart":
		for _, svc := range e.protectedServices {
			if strings.Contains(args, svc) {
				return EvalResult{Level: RiskLevelHigh, Reason: "受保护服务: " + svc, Blocked: true}
			}
		}
	case strings.HasPrefix(name, "svc."):
		if strings.Contains(args, "restart") || strings.Contains(args, "stop") || strings.Contains(args, "start") {
			return EvalResult{Level: RiskLevelLow, Reason: "服务管理操作需要审批"}
		}
	}
	for _, tool := range e.approvalTools {
		if name == tool {
			return EvalResult{Level: RiskLevelLow, Reason: "需要用户审批"}
		}
	}
	return EvalResult{Level: RiskLevelSafe}
}

func (e *Evaluator) evaluateShell(args string) EvalResult {
	// Check high-risk patterns
	for _, re := range e.highRiskPatterns {
		if re.MatchString(args) {
			return EvalResult{Level: RiskLevelHigh, Reason: "匹配高风险命令模式", Blocked: true}
		}
	}
	// Check protected paths
	for _, p := range e.protectedPaths {
		if strings.Contains(args, p) {
			return EvalResult{Level: RiskLevelHigh, Reason: "涉及受保护路径: " + p, Blocked: true}
		}
	}
	// Check blacklist
	for _, re := range e.shellBlacklist {
		if re.MatchString(args) {
			return EvalResult{Level: RiskLevelHigh, Reason: "匹配黑名单模式", Blocked: true}
		}
	}
	// Check whitelist (if non-empty, only allow matching commands)
	if len(e.shellWhitelist) > 0 {
		allowed := false
		for _, re := range e.shellWhitelist {
			if re.MatchString(args) {
				allowed = true
				break
			}
		}
		if !allowed {
			return EvalResult{Level: RiskLevelHigh, Reason: "不在白名单中", Blocked: true}
		}
	}
	// Shell commands default to low risk (needs approval)
	return EvalResult{Level: RiskLevelLow, Reason: "Shell 命令需要人工审批"}
}
