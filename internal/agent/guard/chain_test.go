package guard

import (
	"context"
	"testing"

	agentmodel "github.com/rizxfrog/VanPanelBackend/internal/agent/model"
	"github.com/rizxfrog/VanPanelBackend/internal/agent/risk"
)

func TestChainEvaluateSafeCommand(t *testing.T) {
	cfg := &risk.EvaluatorConfig{
		HighRiskPatterns: []string{`rm\s+-rf`},
		ProtectedPaths:   []string{"/etc", "/boot"},
	}
	eval := risk.NewEvaluator(cfg)
	chain := NewChain(eval, nil)

	decision := chain.Evaluate(context.Background(), "shell.exec", map[string]any{
		"command": "ps aux | grep nginx",
	})

	if !decision.Allowed {
		t.Errorf("safe command should be allowed, got rejected: %s", decision.Reason)
	}
	if decision.Level != agentmodel.RiskSafe {
		t.Errorf("safe command should be risk safe, got: %s", decision.Level)
	}
}

func TestChainEvaluateDangerousCommand(t *testing.T) {
	cfg := &risk.EvaluatorConfig{
		HighRiskPatterns: []string{`rm\s+-rf`},
	}
	eval := risk.NewEvaluator(cfg)
	chain := NewChain(eval, nil)

	decision := chain.Evaluate(context.Background(), "shell.exec", map[string]any{
		"command": "rm -rf /var/log/*",
	})

	if decision.Allowed {
		t.Error("dangerous command should be blocked")
	}
	if decision.Level != agentmodel.RiskHigh {
		t.Errorf("dangerous command should be risk high, got: %s", decision.Level)
	}
}

func TestChainEvaluateProtectedPath(t *testing.T) {
	cfg := &risk.EvaluatorConfig{
		ProtectedPaths: []string{"/etc", "/boot"},
	}
	eval := risk.NewEvaluator(cfg)
	chain := NewChain(eval, nil)

	decision := chain.Evaluate(context.Background(), "shell.exec", map[string]any{
		"command": "chmod 777 /etc/passwd",
	})

	if decision.Allowed {
		t.Error("protected path should be blocked")
	}
}

func TestChainWithoutEvaluator(t *testing.T) {
	chain := NewChain(nil, nil)

	decision := chain.Evaluate(context.Background(), "shell.exec", map[string]any{
		"command": "rm -rf /",
	})

	if !decision.Allowed {
		t.Error("without evaluator, should default to allowed")
	}
}
