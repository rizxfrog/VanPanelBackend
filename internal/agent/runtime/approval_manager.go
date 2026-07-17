package runtime

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ApprovalResult 审批结果
type ApprovalResult struct {
	ID        string
	Status    string // "approved" / "rejected" / "pending"
	Reason    string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// ApprovalManager 审批管理器
type ApprovalManager struct {
	mu               sync.Mutex
	pendingApprovals map[string]*ApprovalResult
	autoApprove      bool // 开发模式：自动审批
	ttl              time.Duration
}

// NewApprovalManager 创建审批管理器
func NewApprovalManager(autoApprove bool) *ApprovalManager {
	m := &ApprovalManager{
		pendingApprovals: make(map[string]*ApprovalResult),
		autoApprove:      autoApprove,
		ttl:              5 * time.Minute,
	}
	// 启动过期清理
	go m.cleanupLoop()
	return m
}

// cleanupLoop 定期清理过期审批
func (m *ApprovalManager) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		m.cleanup()
	}
}

func (m *ApprovalManager) cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for id, a := range m.pendingApprovals {
		if now.After(a.ExpiresAt) {
			a.Status = "expired"
			delete(m.pendingApprovals, id)
		}
	}
}

// Request 请求审批
func (m *ApprovalManager) Request(ctx context.Context, sessionID string, call ToolCall, decision *PolicyDecision) (*ApprovalResult, error) {
	if m.autoApprove {
		return &ApprovalResult{
			ID:        uuid.New().String(),
			Status:    "approved",
			Reason:    "自动审批（开发模式）",
			CreatedAt: time.Now(),
		}, nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	approval := &ApprovalResult{
		ID:        uuid.New().String(),
		Status:    "pending",
		Reason:    decision.Reason,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(m.ttl),
	}
	m.pendingApprovals[approval.ID] = approval

	// TODO: 发送审批通知给用户（WebSocket / 推送）
	// 当前返回 pending 状态，调用方可轮询或等待

	return approval, nil
}

// Approve 批准审批
func (m *ApprovalManager) Approve(approvalID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	approval, ok := m.pendingApprovals[approvalID]
	if !ok {
		return fmt.Errorf("审批 %s 不存在", approvalID)
	}
	approval.Status = "approved"
	delete(m.pendingApprovals, approvalID)
	return nil
}

// Reject 拒绝审批
func (m *ApprovalManager) Reject(approvalID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	approval, ok := m.pendingApprovals[approvalID]
	if !ok {
		return fmt.Errorf("审批 %s 不存在", approvalID)
	}
	approval.Status = "rejected"
	delete(m.pendingApprovals, approvalID)
	return nil
}

// Get 获取审批状态
func (m *ApprovalManager) Get(approvalID string) (*ApprovalResult, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.pendingApprovals[approvalID]
	return a, ok
}
