package runtime

import (
	"context"
	"testing"
)

func TestPolicyEngine_BlocksDangerousCommand(t *testing.T) {
	pe := NewPolicyEngine(nil)
	call := ToolCall{Name: "shell.exec", Args: map[string]any{"command": "rm -rf /"}}
	decision := pe.Evaluate(context.Background(), call)
	if decision.Allowed {
		t.Fatal("dangerous command should be blocked")
	}
}

func TestPolicyEngine_RequiresApprovalForServiceRestart(t *testing.T) {
	pe := NewPolicyEngine(nil)
	call := ToolCall{Name: "service.restart", Args: map[string]any{"service": "nginx"}}
	decision := pe.Evaluate(context.Background(), call)
	if !decision.RequiresApproval {
		t.Fatal("service restart should require approval")
	}
}

func TestPolicyEngine_AllowsSafeCommand(t *testing.T) {
	pe := NewPolicyEngine(nil)
	call := ToolCall{Name: "shell.exec", Args: map[string]any{"command": "ls -la"}}
	decision := pe.Evaluate(context.Background(), call)
	if !decision.Allowed {
		t.Fatal("safe command should be allowed")
	}
}
