package di

import (
	"os"
	"strconv"
	"time"

	agentaudit "github.com/GoSimplicity/AI-CloudOps/internal/agent/audit"
	agentplanner "github.com/GoSimplicity/AI-CloudOps/internal/agent/planner"
	agentrisk "github.com/GoSimplicity/AI-CloudOps/internal/agent/risk"
	agentservice "github.com/GoSimplicity/AI-CloudOps/internal/agent/service"
	agenttools "github.com/GoSimplicity/AI-CloudOps/internal/agent/tools"
	"go.uber.org/zap"
)

func ProvideAgentPlanner() agentplanner.Planner {
	baseURL := os.Getenv("VAN_AGENT_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	var timeout time.Duration
	if s := os.Getenv("VAN_AGENT_TIMEOUT"); s != "" {
		if secs, err := strconv.Atoi(s); err == nil {
			timeout = time.Duration(secs) * time.Second
		}
	}
	return agentplanner.NewHTTPPlanner(baseURL, timeout)
}

func ProvideAgentRiskGuard() *agentrisk.Guard {
	return agentrisk.NewGuard()
}

func ProvideAgentToolRegistry() *agenttools.Registry {
	return agenttools.NewRegistry()
}

func ProvideAgentAuditStore() agentaudit.Store {
	return agentaudit.NewMemoryStore()
}

func ProvideAgentApprovalStore() *agentservice.ApprovalStore {
	return agentservice.NewApprovalStore()
}

func ProvideAgentService(
	planner agentplanner.Planner,
	guard *agentrisk.Guard,
	registry *agenttools.Registry,
	auditStore agentaudit.Store,
	approvalStore *agentservice.ApprovalStore,
	logger *zap.Logger,
) *agentservice.Service {
	return agentservice.NewService(planner, guard, registry, auditStore, approvalStore, logger)
}
