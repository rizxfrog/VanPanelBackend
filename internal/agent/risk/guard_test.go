package risk

import (
	"testing"

	agentmodel "github.com/rizxfrog/VanPanelBackend/internal/agent/model"
)

func TestGuardBlocksDangerousTerminalCommand(t *testing.T) {
	decision := NewGuard().Evaluate(agentmodel.ToolCall{
		Name: "terminal.suggest",
		Args: map[string]any{"command": "rm -rf /"},
	})

	if decision.Allowed || decision.Level != agentmodel.RiskHigh {
		t.Fatalf("expected high risk block, got %+v", decision)
	}
}

func TestGuardBlocksProtectedFileOperation(t *testing.T) {
	decision := NewGuard().Evaluate(agentmodel.ToolCall{
		Name: "file.move_to_trash",
		Args: map[string]any{"path": "/etc/passwd"},
	})

	if decision.Allowed || decision.Level != agentmodel.RiskHigh {
		t.Fatalf("expected protected path block, got %+v", decision)
	}
}

func TestGuardBlocksProtectedServiceRestart(t *testing.T) {
	decision := NewGuard().Evaluate(agentmodel.ToolCall{
		Name: "service.restart",
		Args: map[string]any{"service": "firewalld"},
	})

	if decision.Allowed || decision.Level != agentmodel.RiskHigh {
		t.Fatalf("expected protected service block, got %+v", decision)
	}
}

func TestGuardAllowsSafeDiagnosticTool(t *testing.T) {
	decision := NewGuard().Evaluate(agentmodel.ToolCall{
		Name: "disk.analyze",
		Args: map[string]any{"path": "/var/log"},
	})

	if !decision.Allowed || decision.RequiresApproval || decision.Level != agentmodel.RiskSafe {
		t.Fatalf("expected safe diagnostic tool, got %+v", decision)
	}
}

func TestGuardRequiresApprovalForContainerRestart(t *testing.T) {
	decision := NewGuard().Evaluate(agentmodel.ToolCall{
		Name: "container.restart",
		Args: map[string]any{"id": "nginx"},
	})

	if !decision.Allowed || !decision.RequiresApproval || decision.Level != agentmodel.RiskLow {
		t.Fatalf("expected low risk approval, got %+v", decision)
	}
}
