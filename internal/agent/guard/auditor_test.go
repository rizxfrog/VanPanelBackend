package guard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuditorApprove(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{
					"content": `{"decision":"approve","reason":"操作安全，参数无异常"}`,
				}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	a := NewAuditor(AuditorConfig{
		BaseURL:    server.URL,
		Model:      "test-model",
		MaxRetries: 0,
	})

	result, err := a.Evaluate(context.Background(), "disk.df", map[string]any{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Allowed {
		t.Errorf("expected approve, got reject: %s", result.Reason)
	}
}

func TestAuditorReject(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{
					"content": `{"decision":"reject","reason":"参数包含路径遍历: ../../etc/shadow"}`,
				}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	a := NewAuditor(AuditorConfig{
		BaseURL:    server.URL,
		Model:      "test-model",
		MaxRetries: 0,
	})

	result, err := a.Evaluate(context.Background(), "shell.exec", map[string]any{
		"command": "cat ../../etc/shadow",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Allowed {
		t.Error("expected reject, got approve")
	}
}

func TestAuditorServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	a := NewAuditor(AuditorConfig{
		BaseURL:    server.URL,
		Model:      "test-model",
		MaxRetries: 1,
	})

	_, err := a.Evaluate(context.Background(), "disk.df", map[string]any{})
	if err == nil {
		t.Error("expected error for server error")
	}
}
