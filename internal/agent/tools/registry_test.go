package tools

import (
	"context"
	"testing"

	agentmodel "github.com/rizxfrog/VanPanelBackend/internal/agent/model"
)

func TestRegistryListsKnownTools(t *testing.T) {
	registry := NewRegistry()

	tools := registry.List()
	if len(tools) != 8 {
		t.Fatalf("expected 8 tools, got %d: %+v", len(tools), tools)
	}
	if tools[0]["name"] != "container.inspect" {
		t.Fatalf("expected sorted tool list, got %+v", tools)
	}
}

func TestRegistryExecutesSystemInspect(t *testing.T) {
	registry := NewRegistry()

	result, err := registry.Execute(context.Background(), agentmodel.ToolCall{Name: "system.inspect"})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if result.Output["os"] == "" || result.Output["hostname"] == "" {
		t.Fatalf("expected system output, got %+v", result)
	}
}

func TestRegistryReturnsMissingToolError(t *testing.T) {
	registry := NewRegistry()

	_, err := registry.Execute(context.Background(), agentmodel.ToolCall{Name: "missing.tool"})
	if err != ErrToolNotFound {
		t.Fatalf("expected ErrToolNotFound, got %v", err)
	}
}
