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
	HighRiskPatterns []string
	ProtectedPaths   []string
	Shell            ShellConfig
}

// Evaluator evaluates tool calls for risk based on configurable patterns and rules.
type Evaluator struct {
	highRiskPatterns []*regexp.Regexp
	protectedPaths   []string
	shellBlacklist   []*regexp.Regexp
	shellWhitelist   []*regexp.Regexp
}

// NewEvaluator creates an Evaluator from the given configuration.
func NewEvaluator(cfg *EvaluatorConfig) *Evaluator {
	e := &Evaluator{
		protectedPaths: cfg.ProtectedPaths,
	}
	for _, p := range cfg.HighRiskPatterns {
		if re, err := regexp.Compile(p); err == nil {
			e.highRiskPatterns = append(e.highRiskPatterns, re)
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
	case strings.HasPrefix(name, "svc."):
		if strings.Contains(args, "restart") || strings.Contains(args, "stop") || strings.Contains(args, "start") {
			return EvalResult{Level: RiskLevelLow, Reason: "服务管理操作需要审批"}
		}
		return EvalResult{Level: RiskLevelSafe}
	default:
		return EvalResult{Level: RiskLevelSafe}
	}
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
