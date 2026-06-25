package runtime

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ApprovalResult 审批结果
type ApprovalResult struct {
	ID        string
	Status    string // "approved" / "rejected" / "pending"
	Reason    string
	CreatedAt time.Time
}

// ApprovalManager 审批管理器
type ApprovalManager struct {
	pendingApprovals map[string]*ApprovalResult
	autoApprove      bool // 开发模式：自动审批
}

// NewApprovalManager 创建审批管理器
func NewApprovalManager(autoApprove bool) *ApprovalManager {
	return &ApprovalManager{
		pendingApprovals: make(map[string]*ApprovalResult),
		autoApprove:      autoApprove,
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

	approval := &ApprovalResult{
		ID:        uuid.New().String(),
		Status:    "pending",
		Reason:    decision.Reason,
		CreatedAt: time.Now(),
	}
	m.pendingApprovals[approval.ID] = approval

	// TODO: 发送审批通知给用户（WebSocket / 推送）
	// TODO: 等待用户响应或超时

	return approval, fmt.Errorf("审批功能尚未完整实现，审批 ID: %s", approval.ID)
}

// Approve 批准审批
func (m *ApprovalManager) Approve(approvalID string) error {
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
	approval, ok := m.pendingApprovals[approvalID]
	if !ok {
		return fmt.Errorf("审批 %s 不存在", approvalID)
	}
	approval.Status = "rejected"
	delete(m.pendingApprovals, approvalID)
	return nil
}
