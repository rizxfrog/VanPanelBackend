package audit

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	agentDao "github.com/rizxfrog/VanPanelBackend/internal/agent/dao"
	agentmodel "github.com/rizxfrog/VanPanelBackend/internal/agent/model"
	"github.com/rizxfrog/VanPanelBackend/internal/model"
	"go.uber.org/zap"
)

// 审计事件类型常量
const (
	ActionReceive  = "agent.receive"
	ActionEvaluate = "tool.evaluate"
	ActionExecute  = "tool.execute"
	ActionBlocked  = "tool.blocked"
	ActionComplete = "agent.complete"
)

type Store interface {
	Append(ctx context.Context, event agentmodel.AuditEvent) (agentmodel.AuditEvent, error)
	ListBySession(ctx context.Context, sessionID string) ([]agentmodel.AuditEvent, error)
}

type MemoryStore struct {
	mu     sync.RWMutex
	events []agentmodel.AuditEvent
	limit  int
	dao    agentDao.AgentDAO
	logger *zap.Logger
}

func NewMemoryStore(dao agentDao.AgentDAO, logger *zap.Logger) *MemoryStore {
	return &MemoryStore{limit: 2000, dao: dao, logger: logger}
}

func (s *MemoryStore) Append(ctx context.Context, event agentmodel.AuditEvent) (agentmodel.AuditEvent, error) {
	if event.ID == "" {
		event.ID = uuid.NewString()
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now()
	}

	s.mu.Lock()
	s.events = append(s.events, event)
	if s.limit > 0 && len(s.events) > s.limit {
		s.events = s.events[len(s.events)-s.limit:]
	}
	s.mu.Unlock()

	// DB write-through (non-fatal)
	if s.dao != nil {
		if err := s.dao.CreateAuditEvent(ctx, toDBAuditEvent(event)); err != nil && s.logger != nil {
			s.logger.Error("audit write-through failed",
				zap.String("event_id", event.ID),
				zap.String("session_id", event.SessionID),
				zap.Error(err),
			)
		}
	}

	return event, nil
}

func (s *MemoryStore) ListBySession(_ context.Context, sessionID string) ([]agentmodel.AuditEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]agentmodel.AuditEvent, 0)
	for _, event := range s.events {
		if event.SessionID == sessionID {
			out = append(out, event)
		}
	}
	return out, nil
}

func toDBAuditEvent(e agentmodel.AuditEvent) *model.AgentAuditEvent {
	return &model.AgentAuditEvent{
		SessionID: e.SessionID,
		UserID:    int(e.UserID),
		ToolName:  e.ToolName,
		RiskLevel: string(e.Risk),
		Action:    e.Action,
		Result:    fmt.Sprintf("allowed=%v reason=%s", e.Allowed, e.Reason),
		CreatedAt: e.CreatedAt,
	}
}
