package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	agentaudit "github.com/GoSimplicity/AI-CloudOps/internal/agent/audit"
	agentmodel "github.com/GoSimplicity/AI-CloudOps/internal/agent/model"
	agentplanner "github.com/GoSimplicity/AI-CloudOps/internal/agent/planner"
	agentrisk "github.com/GoSimplicity/AI-CloudOps/internal/agent/risk"
	agenttools "github.com/GoSimplicity/AI-CloudOps/internal/agent/tools"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type Service struct {
	mu        sync.RWMutex
	sessions  map[string]agentmodel.AgentSession
	planner   agentplanner.Planner
	guard     *agentrisk.Guard
	tools     *agenttools.Registry
	audit     agentaudit.Store
	approvals *ApprovalStore
	logger    *zap.Logger
}

func NewService(
	planner agentplanner.Planner,
	guard *agentrisk.Guard,
	tools *agenttools.Registry,
	audit agentaudit.Store,
	approvals *ApprovalStore,
	logger *zap.Logger,
) *Service {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Service{
		sessions:  make(map[string]agentmodel.AgentSession),
		planner:   planner,
		guard:     guard,
		tools:     tools,
		audit:     audit,
		approvals: approvals,
		logger:    logger,
	}
}

func (s *Service) CreateSession(ctx context.Context, userID uint, username string) (agentmodel.AgentSession, error) {
	now := time.Now()
	session := agentmodel.AgentSession{
		ID:        uuid.NewString(),
		UserID:    userID,
		Username:  username,
		Title:     "New Chat",
		Messages:  []agentmodel.AgentMessage{},
		CreatedAt: now,
		UpdatedAt: now,
	}

	s.mu.Lock()
	s.sessions[session.ID] = session
	s.mu.Unlock()

	_, _ = s.audit.Append(ctx, agentmodel.AuditEvent{
		SessionID: session.ID,
		UserID:    userID,
		Username:  username,
		Action:    "agent.session.create",
		Risk:      agentmodel.RiskSafe,
		Allowed:   true,
	})
	return session, nil
}

func (s *Service) Query(ctx context.Context, userID uint, username string, req agentmodel.QueryRequest) (agentmodel.QueryResponse, error) {
	if req.Message == "" {
		return agentmodel.QueryResponse{}, fmt.Errorf("message is required")
	}

	session, err := s.ensureSession(ctx, userID, username, req.SessionID)
	if err != nil {
		return agentmodel.QueryResponse{}, err
	}
	session.Messages = append(session.Messages, agentmodel.AgentMessage{Role: "user", Content: req.Message, CreatedAt: time.Now()})

	plan, err := s.planner.Plan(ctx, req.Message, session.Messages)
	if err != nil {
		return agentmodel.QueryResponse{}, err
	}

	results := make([]agentmodel.ToolResult, 0, len(plan.ToolCalls))
	approvals := make([]agentmodel.Approval, 0)
	calls := make([]agentmodel.ToolCall, 0, len(plan.ToolCalls))

	for _, call := range plan.ToolCalls {
		if call.ID == "" {
			call.ID = uuid.NewString()
		}
		call.Status = "planned"
		decision := s.guard.Evaluate(call)
		_, _ = s.audit.Append(ctx, agentmodel.AuditEvent{
			SessionID: session.ID,
			UserID:    userID,
			Username:  username,
			Action:    "agent.tool.plan",
			ToolName:  call.Name,
			Risk:      decision.Level,
			Allowed:   decision.Allowed,
			Reason:    decision.Reason,
			Metadata:  map[string]any{"args": call.Args},
		})

		if !decision.Allowed {
			call.Status = "blocked"
			call.Error = decision.Reason
			calls = append(calls, call)
			continue
		}
		if decision.RequiresApproval {
			call.Status = "pending_approval"
			approval, err := s.approvals.Create(ctx, agentmodel.Approval{
				SessionID: session.ID,
				UserID:    userID,
				ToolCall:  call,
				Decision:  decision,
			})
			if err != nil {
				return agentmodel.QueryResponse{}, err
			}
			approvals = append(approvals, approval)
			calls = append(calls, call)
			continue
		}

		result, err := s.tools.Execute(ctx, call)
		if err != nil {
			call.Status = "failed"
			call.Error = err.Error()
		} else {
			call.Status = "completed"
			call.Result = result.Output
		}
		results = append(results, result)
		calls = append(calls, call)
	}

	plan.ToolCalls = calls
	answer, err := s.planner.Respond(ctx, req.Message, plan, results)
	if err != nil {
		s.logger.Warn("agent planner respond failed", zap.Error(err))
		answer = fallbackAnswer(plan, calls, approvals)
	}
	message := agentmodel.AgentMessage{Role: "assistant", Content: answer, CreatedAt: time.Now()}
	session.Messages = append(session.Messages, message)
	session.UpdatedAt = time.Now()

	s.mu.Lock()
	s.sessions[session.ID] = session
	s.mu.Unlock()

	_, _ = s.audit.Append(ctx, agentmodel.AuditEvent{
		SessionID: session.ID,
		UserID:    userID,
		Username:  username,
		Action:    "agent.query",
		Risk:      plan.Risk,
		Allowed:   true,
		Metadata:  map[string]any{"intent": plan.Intent},
	})

	return agentmodel.QueryResponse{
		SessionID: session.ID,
		Message:   message,
		Intent:    plan.Intent,
		Risk:      plan.Risk,
		ToolCalls: calls,
		Approvals: approvals,
	}, nil
}

func (s *Service) Events(ctx context.Context, sessionID string) ([]agentmodel.AuditEvent, error) {
	return s.audit.ListBySession(ctx, sessionID)
}

func (s *Service) Tools() []map[string]string {
	return s.tools.List()
}

func (s *Service) ConfirmApproval(ctx context.Context, approvalID string) (agentmodel.ToolResult, error) {
	approval, err := s.approvals.Approve(ctx, approvalID)
	if err != nil {
		return agentmodel.ToolResult{}, err
	}
	return s.tools.Execute(ctx, approval.ToolCall)
}

func (s *Service) RejectApproval(ctx context.Context, approvalID string) (agentmodel.Approval, error) {
	return s.approvals.Reject(ctx, approvalID)
}

func (s *Service) ensureSession(ctx context.Context, userID uint, username string, sessionID string) (agentmodel.AgentSession, error) {
	if sessionID != "" {
		s.mu.RLock()
		session, ok := s.sessions[sessionID]
		s.mu.RUnlock()
		if ok {
			return session, nil
		}
	}
	return s.CreateSession(ctx, userID, username)
}

func fallbackAnswer(plan agentmodel.PlanResponse, calls []agentmodel.ToolCall, approvals []agentmodel.Approval) string {
	if len(approvals) > 0 {
		return "已生成诊断计划，其中部分操作需要你确认后执行。"
	}
	for _, call := range calls {
		if call.Status == "blocked" {
			return "该请求包含高风险操作，已被安全护栏阻断。"
		}
	}
	if plan.Summary != "" {
		return plan.Summary
	}
	return "已完成诊断。"
}
