package risk

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	agentmodel "github.com/rizxfrog/VanPanelBackend/internal/agent/model"
)

type Guard struct {
	dangerousCommand *regexp.Regexp
	protectedPaths   []string
	protectedService map[string]struct{}
	approvalTools    map[string]struct{}
}

func NewGuard() *Guard {
	return &Guard{
		dangerousCommand: regexp.MustCompile(`(?i)(\brm\s+-rf\s+/|\bdd\s+if=|\bmkfs\.|\bshutdown\b|\breboot\b|:\(\)\{:\|:&\};:)`),
		protectedPaths: []string{
			"/boot",
			"/etc",
			"/root",
			"/usr",
			"/var/lib/docker",
			"C:\\Windows",
			"C:\\Program Files",
		},
		protectedService: map[string]struct{}{
			"firewalld": {},
			"sshd":      {},
			"ssh":       {},
			"docker":    {},
			"kubelet":   {},
		},
		approvalTools: map[string]struct{}{
			"container.restart":  {},
			"container.stop":     {},
			"service.restart":    {},
			"file.delete":        {},
			"file.move_to_trash": {},
		},
	}
}

func (g *Guard) Evaluate(call agentmodel.ToolCall) agentmodel.RiskDecision {
	if g == nil {
		g = NewGuard()
	}

	if reason := g.blockReason(call); reason != "" {
		return agentmodel.RiskDecision{
			Level:   agentmodel.RiskHigh,
			Allowed: false,
			Reason:  reason,
		}
	}

	if _, ok := g.approvalTools[call.Name]; ok {
		return agentmodel.RiskDecision{
			Level:            agentmodel.RiskLow,
			Allowed:          true,
			RequiresApproval: true,
			Reason:           "operation requires explicit user approval",
		}
	}

	return agentmodel.RiskDecision{
		Level:   agentmodel.RiskSafe,
		Allowed: true,
		Reason:  "read-only or suggestion operation",
	}
}

func (g *Guard) blockReason(call agentmodel.ToolCall) string {
	switch call.Name {
	case "terminal.suggest":
		command := strings.TrimSpace(fmt.Sprint(call.Args["command"]))
		if command != "" && g.dangerousCommand.MatchString(command) {
			return "dangerous terminal command is blocked"
		}
	case "file.delete", "file.move_to_trash":
		path := strings.TrimSpace(fmt.Sprint(call.Args["path"]))
		if path != "" && g.isProtectedPath(path) {
			return "protected system path is blocked"
		}
	case "service.restart":
		service := strings.ToLower(strings.TrimSpace(fmt.Sprint(call.Args["service"])))
		if _, ok := g.protectedService[service]; ok {
			return "protected service operation is blocked"
		}
	}
	return ""
}

func (g *Guard) isProtectedPath(path string) bool {
	clean := filepath.Clean(path)
	lower := strings.ToLower(clean)
	for _, protected := range g.protectedPaths {
		p := strings.ToLower(filepath.Clean(protected))
		if lower == p || strings.HasPrefix(lower, p+string(filepath.Separator)) || strings.HasPrefix(lower, p+"/") {
			return true
		}
	}
	return false
}
