package planner

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	agentmodel "github.com/GoSimplicity/AI-CloudOps/internal/agent/model"
)

func TestHTTPPlannerPlanUsesAIFirstAssistantQuery(t *testing.T) {
	var requestedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		if r.URL.Path != "/api/v1/assistant/query" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if payload["question"] != "hello" {
			t.Fatalf("unexpected question payload: %#v", payload)
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"code":    0,
			"message": "智能助手查询成功",
			"data": map[string]any{
				"answer":     "Hello, how can I help you today?",
				"session_id": "s1",
			},
		})
	}))
	defer server.Close()

	planner := NewHTTPPlanner(server.URL, 0)
	plan, err := planner.Plan(context.Background(), "hello", nil)
	if err != nil {
		t.Fatalf("plan failed: %v", err)
	}

	if requestedPath != "/api/v1/assistant/query" {
		t.Fatalf("planner used wrong path: %s", requestedPath)
	}
	if plan.Intent != "ai_first" {
		t.Fatalf("unexpected intent: %s", plan.Intent)
	}
	if plan.Summary != "Hello, how can I help you today?" {
		t.Fatalf("unexpected summary: %s", plan.Summary)
	}
	if len(plan.ToolCalls) != 0 {
		t.Fatalf("expected no Go-side tool calls, got %d", len(plan.ToolCalls))
	}
}

func TestHTTPPlannerRespondReturnsAIFirstPlanSummary(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("Respond must not call old agent/respond endpoint for ai_first plan")
	}))
	defer server.Close()

	planner := NewHTTPPlanner(server.URL, 0)
	answer, err := planner.Respond(context.Background(), "hello", agentmodel.PlanResponse{
		Intent:  "ai_first",
		Summary: "Hello, how can I help you today?",
		Risk:    agentmodel.RiskSafe,
	}, nil)
	if err != nil {
		t.Fatalf("respond failed: %v", err)
	}
	if answer != "Hello, how can I help you today?" {
		t.Fatalf("unexpected answer: %s", answer)
	}
}

func TestHTTPPlannerStreamUsesAssistantStreamEndpoint(t *testing.T) {
	var requestedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		if r.URL.Path != "/api/v1/assistant/query/stream" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Accept") != "text/event-stream" {
			t.Fatalf("missing stream accept header: %s", r.Header.Get("Accept"))
		}

		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("event: delta\ndata: {\"content\":\"hello\"}\n\n"))
	}))
	defer server.Close()

	planner := NewHTTPPlanner(server.URL, 0)
	var chunks []byte
	err := planner.Stream(context.Background(), "hello", nil, func(chunk []byte) error {
		chunks = append(chunks, chunk...)
		return nil
	})
	if err != nil {
		t.Fatalf("stream failed: %v", err)
	}

	if requestedPath != "/api/v1/assistant/query/stream" {
		t.Fatalf("planner used wrong path: %s", requestedPath)
	}
	if string(chunks) != "event: delta\ndata: {\"content\":\"hello\"}\n\n" {
		t.Fatalf("unexpected stream chunks: %q", string(chunks))
	}
}
