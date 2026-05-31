package audit

import (
	"context"
	"testing"

	agentmodel "github.com/rizxfrog/VanPanelBackend/internal/agent/model"
)

func TestMemoryStoreAppendAndListBySession(t *testing.T) {
	store := NewMemoryStore()

	event, err := store.Append(context.Background(), agentmodel.AuditEvent{
		SessionID: "s1",
		Action:    "agent.query",
	})
	if err != nil {
		t.Fatalf("append failed: %v", err)
	}
	if event.ID == "" || event.CreatedAt.IsZero() {
		t.Fatalf("expected generated id and timestamp, got %+v", event)
	}

	_, _ = store.Append(context.Background(), agentmodel.AuditEvent{SessionID: "s2", Action: "agent.query"})
	events, err := store.ListBySession(context.Background(), "s1")
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(events) != 1 || events[0].SessionID != "s1" {
		t.Fatalf("unexpected events: %+v", events)
	}
}
