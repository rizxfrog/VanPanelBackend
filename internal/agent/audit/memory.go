package audit

import (
	"context"
	"sync"
	"time"

	agentmodel "github.com/GoSimplicity/AI-CloudOps/internal/agent/model"
	"github.com/google/uuid"
)

type Store interface {
	Append(ctx context.Context, event agentmodel.AuditEvent) (agentmodel.AuditEvent, error)
	ListBySession(ctx context.Context, sessionID string) ([]agentmodel.AuditEvent, error)
}

type MemoryStore struct {
	mu     sync.RWMutex
	events []agentmodel.AuditEvent
	limit  int
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{limit: 1000}
}

func (s *MemoryStore) Append(_ context.Context, event agentmodel.AuditEvent) (agentmodel.AuditEvent, error) {
	if event.ID == "" {
		event.ID = uuid.NewString()
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now()
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, event)
	if s.limit > 0 && len(s.events) > s.limit {
		s.events = s.events[len(s.events)-s.limit:]
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
