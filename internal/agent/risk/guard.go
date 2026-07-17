package risk

import (
	"encoding/json"
	"strings"

	agentmodel "github.com/rizxfrog/VanPanelBackend/internal/agent/model"
)

type Guard struct {
	evaluator *Evaluator
}

func NewGuard(evaluator *Evaluator) *Guard {
	return &Guard{evaluator: evaluator}
}

func (g *Guard) Evaluate(call agentmodel.ToolCall) agentmodel.RiskDecision {
	if g == nil || g.evaluator == nil {
		g = NewGuard(defaultGuardEvaluator())
	}

	argsStr := toolArgsToString(call.Args)
	result := g.evaluator.Evaluate(call.Name, argsStr)
	return guardDecisionFromEvalResult(result)
}

func defaultGuardEvaluator() *Evaluator {
	return NewEvaluator(&EvaluatorConfig{
		DangerousCommands: []string{`(?i)(\brm\s+-rf\s+/|\bdd\s+if=|\bmkfs\.|\bshutdown\b|\breboot\b|:\(\)\{:\|:&\};:)`},
		ProtectedPaths: []string{
			"/boot",
			"/etc",
			"/root",
			"/usr",
			"/var/lib/docker",
			"C:\\Windows",
			"C:\\Program Files",
		},
		ProtectedServices: []string{"firewalld", "sshd", "ssh", "docker", "kubelet"},
		ApprovalTools: []string{
			"container.restart",
			"container.stop",
			"service.restart",
			"file.delete",
			"file.move_to_trash",
		},
	})
}

func toolArgsToString(args map[string]any) string {
	if cmd, ok := args["command"]; ok {
		if cmdStr, ok := cmd.(string); ok {
			return cmdStr
		}
		return ""
	}
	b, err := json.Marshal(args)
	if err != nil {
		return ""
	}
	return string(b)
}

func guardDecisionFromEvalResult(result EvalResult) agentmodel.RiskDecision {
	allowed := !result.Blocked
	if result.Level == RiskLevelLow && !result.Blocked {
		allowed = true
	}

	return agentmodel.RiskDecision{
		Level:            agentmodel.RiskLevel(result.Level),
		Allowed:          allowed,
		RequiresApproval: result.Level == RiskLevelLow && !result.Blocked,
		Reason:           guardReason(result),
	}
}

func guardReason(result EvalResult) string {
	switch result.Reason {
	case "危险命令被拦截":
		return "dangerous terminal command is blocked"
	case "需要用户审批":
		return "operation requires explicit user approval"
	case "":
		return "read-only or suggestion operation"
	}
	if strings.HasPrefix(result.Reason, "受保护路径:") {
		return "protected system path is blocked"
	}
	if strings.HasPrefix(result.Reason, "受保护服务:") {
		return "protected service operation is blocked"
	}
	return result.Reason
}
