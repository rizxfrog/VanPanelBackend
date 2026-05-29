package risk

import (
	"testing"
)

func TestEvaluator_ShellBlocked(t *testing.T) {
	e := NewEvaluator(&EvaluatorConfig{
		HighRiskPatterns: []string{`rm\s+-rf`, `mkfs`},
		ProtectedPaths:   []string{"/boot", "/etc"},
		Shell: ShellConfig{
			DefaultRisk: "low",
			Blacklist:   []string{`dd\s+if=`},
		},
	})

	tests := []struct {
		name   string
		tool   string
		args   string
		expect RiskLevel
		block  bool
	}{
		{"safe tool", "net.lsof", "-i :8080", RiskLevelSafe, false},
		{"svc tool", "svc.systemctl", "restart nginx", RiskLevelLow, false},
		{"svc read", "svc.systemctl", "status nginx", RiskLevelSafe, false},
		{"shell safe", "shell.exec", "ls -la /tmp", RiskLevelLow, false},
		{"shell rm -rf", "shell.exec", "rm -rf /", RiskLevelHigh, true},
		{"shell protected", "shell.exec", "cat /etc/shadow", RiskLevelHigh, true},
		{"shell blacklist", "shell.exec", "dd if=/dev/zero of=/dev/sda", RiskLevelHigh, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := e.Evaluate(tt.tool, tt.args)
			if result.Level != tt.expect {
				t.Errorf("expected level %s, got %s", tt.expect, result.Level)
			}
			if result.Blocked != tt.block {
				t.Errorf("expected blocked=%v, got %v", tt.block, result.Blocked)
			}
		})
	}
}

func TestEvaluator_Whitelist(t *testing.T) {
	e := NewEvaluator(&EvaluatorConfig{
		Shell: ShellConfig{
			DefaultRisk: "low",
			Whitelist:   []string{`^ls\s`, `^cat\s`},
		},
	})

	// Should be blocked (not in whitelist)
	result := e.Evaluate("shell.exec", "rm -rf /tmp")
	if !result.Blocked {
		t.Error("expected blocked for non-whitelisted command")
	}

	// Should be allowed (in whitelist, low risk)
	result = e.Evaluate("shell.exec", "ls -la")
	if result.Blocked {
		t.Error("expected not blocked for whitelisted command")
	}
	if result.Level != RiskLevelLow {
		t.Errorf("expected low risk, got %s", result.Level)
	}
}
