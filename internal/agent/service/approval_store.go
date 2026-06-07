package service

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	agentmodel "github.com/rizxfrog/VanPanelBackend/internal/agent/model"
)

const (
	ApprovalPending  = "pending"
	ApprovalApproved = "approved"
	ApprovalRejected = "rejected"
	ApprovalExpired  = "expired"
)

var ErrApprovalNotFound = errors.New("approval not found")

type ApprovalStore struct {
	mu        sync.RWMutex
	ttl       time.Duration
	approvals map[string]agentmodel.Approval
}

func NewApprovalStore() *ApprovalStore {
	return &ApprovalStore{
		ttl:       10 * time.Minute,
		approvals: make(map[string]agentmodel.Approval),
	}
}

func (s *ApprovalStore) Create(_ context.Context, approval agentmodel.Approval) (agentmodel.Approval, error) {
	now := time.Now()
	if approval.ID == "" {
		approval.ID = uuid.NewString()
	}
	if approval.Status == "" {
		approval.Status = ApprovalPending
	}
	if approval.CreatedAt.IsZero() {
		approval.CreatedAt = now
	}
	approval.UpdatedAt = now

	s.mu.Lock()
	defer s.mu.Unlock()
	s.approvals[approval.ID] = approval
	return approval, nil
}

func (s *ApprovalStore) Get(_ context.Context, id string) (agentmodel.Approval, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	approval, ok := s.approvals[id]
	if !ok {
		return agentmodel.Approval{}, ErrApprovalNotFound
	}
	if approval.Status == ApprovalPending && s.ttl > 0 && time.Since(approval.CreatedAt) > s.ttl {
		approval.Status = ApprovalExpired
		approval.UpdatedAt = time.Now()
		s.approvals[id] = approval
	}
	return approval, nil
}

func (s *ApprovalStore) ListBySession(_ context.Context, sessionID string) ([]agentmodel.Approval, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]agentmodel.Approval, 0)
	for _, approval := range s.approvals {
		if approval.SessionID == sessionID {
			out = append(out, approval)
		}
	}
	return out, nil
}

func (s *ApprovalStore) Approve(ctx context.Context, id string) (agentmodel.Approval, error) {
	return s.setStatus(ctx, id, ApprovalApproved)
}

func (s *ApprovalStore) Reject(ctx context.Context, id string) (agentmodel.Approval, error) {
	return s.setStatus(ctx, id, ApprovalRejected)
}

func (s *ApprovalStore) setStatus(_ context.Context, id string, status string) (agentmodel.Approval, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	approval, ok := s.approvals[id]
	if !ok {
		return agentmodel.Approval{}, ErrApprovalNotFound
	}
	if approval.Status == ApprovalPending {
		approval.Status = status
		approval.UpdatedAt = time.Now()
		s.approvals[id] = approval
	}
	return approval, nil
}
