package service

import (
	"context"
	"testing"
	"time"

	agentmodel "github.com/rizxfrog/VanPanelBackend/internal/agent/model"
)

func TestApprovalStoreCreateApproveReject(t *testing.T) {
	store := NewApprovalStore()

	approval, err := store.Create(context.Background(), agentmodel.Approval{SessionID: "s1"})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if approval.ID == "" || approval.Status != ApprovalPending {
		t.Fatalf("unexpected approval: %+v", approval)
	}

	approved, err := store.Approve(context.Background(), approval.ID)
	if err != nil {
		t.Fatalf("approve failed: %v", err)
	}
	if approved.Status != ApprovalApproved {
		t.Fatalf("expected approved, got %+v", approved)
	}

	rejected, err := store.Reject(context.Background(), approval.ID)
	if err != nil {
		t.Fatalf("reject failed: %v", err)
	}
	if rejected.Status != ApprovalApproved {
		t.Fatalf("approved approval should not be overwritten, got %+v", rejected)
	}
}

func TestApprovalStoreExpiresPendingApproval(t *testing.T) {
	store := NewApprovalStore()
	store.ttl = time.Millisecond

	approval, err := store.Create(context.Background(), agentmodel.Approval{
		SessionID: "s1",
		CreatedAt: time.Now().Add(-time.Second),
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	expired, err := store.Get(context.Background(), approval.ID)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if expired.Status != ApprovalExpired {
		t.Fatalf("expected expired, got %+v", expired)
	}
}
