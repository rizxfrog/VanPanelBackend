package nudge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/zap"
)

func TestNewMemoryNudgeReviewer(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		logger := zap.NewNop()
		r := NewMemoryNudgeReviewer(NudgeConfig{
			MemoryInterval: 5,
			SkillInterval:  7,
		}, t.TempDir(), logger)

		if r == nil {
			t.Fatal("expected non-nil reviewer")
		}
		if r.config.MemoryInterval != 5 {
			t.Errorf("MemoryInterval = %d, want 5", r.config.MemoryInterval)
		}
		if r.config.SkillInterval != 7 {
			t.Errorf("SkillInterval = %d, want 7", r.config.SkillInterval)
		}
	})

	t.Run("default values for zero/negative config", func(t *testing.T) {
		logger := zap.NewNop()
		r := NewMemoryNudgeReviewer(NudgeConfig{}, "", logger)

		if r.config.MemoryInterval != 10 {
			t.Errorf("MemoryInterval = %d, want 10", r.config.MemoryInterval)
		}
		if r.config.SkillInterval != 10 {
			t.Errorf("SkillInterval = %d, want 10", r.config.SkillInterval)
		}
		if r.memoryDir != "data/memory" {
			t.Errorf("memoryDir = %s, want data/memory", r.memoryDir)
		}
	})
}

func TestShouldNudge(t *testing.T) {
	logger := zap.NewNop()
	r := NewMemoryNudgeReviewer(NudgeConfig{
		MemoryInterval: 3,
		SkillInterval:  5,
	}, t.TempDir(), logger)

	// Should not nudge initially
	mem, skill := r.ShouldNudge(0)
	if mem || skill {
		t.Errorf("ShouldNudge(0) = (%v, %v), want (false, false)", mem, skill)
	}

	// Record 2 turns — still not enough
	r.RecordTurn(2)
	r.RecordTurn(1)
	mem, skill = r.ShouldNudge(0)
	if mem || skill {
		t.Errorf("after 2 turns: ShouldNudge = (%v, %v), want (false, false)", mem, skill)
	}

	// Record 3rd turn — memory nudge should trigger
	r.RecordTurn(0)
	mem, skill = r.ShouldNudge(0)
	if !mem {
		t.Errorf("after 3 turns: memory nudge should be true")
	}
	if skill {
		t.Errorf("after 3 turns with 3 tool calls: skill nudge should be false")
	}

	// Record more tool calls — skill nudge should trigger at 5
	r.ResetMemory()
	r.RecordTurn(3)
	r.RecordTurn(2)
	r.RecordTurn(1)
	_, skill = r.ShouldNudge(0)
	if !skill {
		t.Errorf("after 6 tool calls: skill nudge should be true")
	}
}

func TestRecordTurn(t *testing.T) {
	logger := zap.NewNop()
	r := NewMemoryNudgeReviewer(NudgeConfig{
		MemoryInterval: 10,
		SkillInterval:  10,
	}, t.TempDir(), logger)

	r.RecordTurn(3)
	r.RecordTurn(2)

	r.state.mu.Lock()
	defer r.state.mu.Unlock()

	if r.state.TurnCount != 2 {
		t.Errorf("TurnCount = %d, want 2", r.state.TurnCount)
	}
	if r.state.ToolCallCount != 5 {
		t.Errorf("ToolCallCount = %d, want 5", r.state.ToolCallCount)
	}
}

func TestResetMemory(t *testing.T) {
	logger := zap.NewNop()
	r := NewMemoryNudgeReviewer(NudgeConfig{
		MemoryInterval: 10,
		SkillInterval:  10,
	}, t.TempDir(), logger)

	r.RecordTurn(5)
	r.RecordTurn(5)
	r.RecordTurn(5)
	r.RecordTurn(5)
	r.RecordTurn(5)
	r.RecordTurn(5)
	r.RecordTurn(5)
	r.RecordTurn(5)
	r.RecordTurn(5)
	r.RecordTurn(5)

	mem, _ := r.ShouldNudge(0)
	if !mem {
		t.Fatal("expected memory nudge before reset")
	}

	r.ResetMemory()
	mem, _ = r.ShouldNudge(0)
	if mem {
		t.Errorf("memory nudge should be false after reset")
	}

	r.state.mu.Lock()
	if r.state.TurnCount != 0 {
		t.Errorf("TurnCount = %d after reset, want 0", r.state.TurnCount)
	}
	r.state.mu.Unlock()
}

func TestResetSkill(t *testing.T) {
	logger := zap.NewNop()
	r := NewMemoryNudgeReviewer(NudgeConfig{
		MemoryInterval: 100,
		SkillInterval:  5,
	}, t.TempDir(), logger)

	r.RecordTurn(6)
	r.RecordTurn(6)

	_, skill := r.ShouldNudge(0)
	if !skill {
		t.Fatal("expected skill nudge before reset")
	}

	r.ResetSkill()

	r.state.mu.Lock()
	if r.state.ToolCallCount != 0 {
		t.Errorf("ToolCallCount = %d after reset, want 0", r.state.ToolCallCount)
	}
	r.state.mu.Unlock()
}

func TestIsDuplicate(t *testing.T) {
	tmpDir := t.TempDir()
	logger := zap.NewNop()
	r := NewMemoryNudgeReviewer(NudgeConfig{}, tmpDir, logger)

	// No file yet — should not be duplicate
	if r.isDuplicate("some new content") {
		t.Error("isDuplicate should return false when MEMORY.md doesn't exist")
	}

	// Write some content to MEMORY.md
	memPath := filepath.Join(tmpDir, "MEMORY.md")
	existingContent := "## 2025-01-01\n- user prefers short concise answers\n"
	if err := os.WriteFile(memPath, []byte(existingContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Content that matches existing — should be duplicate
	if !r.isDuplicate("user prefers short concise answers always") {
		t.Errorf("isDuplicate should return true for matching content")
	}

	// Content that doesn't match — should not be duplicate
	if r.isDuplicate("completely different unrelated topic here") {
		t.Errorf("isDuplicate should return false for non-matching content")
	}
}

func TestWriteMemory(t *testing.T) {
	tmpDir := t.TempDir()
	logger := zap.NewNop()
	r := NewMemoryNudgeReviewer(NudgeConfig{}, tmpDir, logger)

	// Write first entry
	if err := r.writeMemory("first entry"); err != nil {
		t.Fatalf("writeMemory failed: %v", err)
	}

	memPath := filepath.Join(tmpDir, "MEMORY.md")
	data, err := os.ReadFile(memPath)
	if err != nil {
		t.Fatalf("failed to read MEMORY.md: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "first entry") {
		t.Errorf("MEMORY.md should contain 'first entry', got: %s", content)
	}

	// Write second entry — should append under same date header
	if err := r.writeMemory("second entry"); err != nil {
		t.Fatalf("writeMemory failed: %v", err)
	}

	data, err = os.ReadFile(memPath)
	if err != nil {
		t.Fatalf("failed to read MEMORY.md: %v", err)
	}
	content = string(data)
	if !strings.Contains(content, "second entry") {
		t.Errorf("MEMORY.md should contain 'second entry', got: %s", content)
	}

	// Date header should appear only once
	dateCount := strings.Count(content, "## ")
	if dateCount != 1 {
		t.Errorf("expected 1 date header, got %d", dateCount)
	}
}

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "bare JSON",
			input:    `{"action": "add", "content": "test"}`,
			expected: `{"action": "add", "content": "test"}`,
		},
		{
			name:     "markdown JSON block",
			input:    "```json\n{\"action\": \"skip\"}\n```",
			expected: `{"action": "skip"}`,
		},
		{
			name:     "markdown code block without language",
			input:    "```\n{\"action\": \"add\", \"content\": \"hi\"}\n```",
			expected: `{"action": "add", "content": "hi"}`,
		},
		{
			name:     "text with embedded JSON",
			input:    "some text before {\"action\": \"skip\"} and after",
			expected: `{"action": "skip"}`,
		},
		{
			name:     "no JSON at all",
			input:    "just plain text",
			expected: "just plain text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractJSON(tt.input)
			if got != tt.expected {
				t.Errorf("extractJSON(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
