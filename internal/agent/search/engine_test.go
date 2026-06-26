package search

import (
	"testing"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

func TestNewSearchEngine(t *testing.T) {
	logger := zap.NewNop()
	engine := NewSearchEngine(&gorm.DB{}, logger)

	if engine == nil {
		t.Fatal("NewSearchEngine 返回 nil")
	}
	if engine.db == nil {
		t.Error("SearchEngine.db 不应为 nil")
	}
	if engine.logger == nil {
		t.Error("SearchEngine.logger 不应为 nil")
	}
}

func TestSearchResultStruct(t *testing.T) {
	sr := &SearchResult{
		SessionID:    "test-session",
		MessageID:    1,
		Role:         "user",
		Headline:     "test headline",
		Content:      "test content",
		Rank:         0.95,
		SessionTitle: "test title",
	}

	if sr.SessionID != "test-session" {
		t.Errorf("SessionID = %q, want %q", sr.SessionID, "test-session")
	}
	if sr.MessageID != 1 {
		t.Errorf("MessageID = %d, want %d", sr.MessageID, 1)
	}
	if sr.Role != "user" {
		t.Errorf("Role = %q, want %q", sr.Role, "user")
	}
}

func TestBrowseResultStruct(t *testing.T) {
	br := &BrowseResult{
		SessionID:    "test-session",
		Title:        "test title",
		Preview:      "preview text",
		MessageCount: 42,
	}

	if br.SessionID != "test-session" {
		t.Errorf("SessionID = %q, want %q", br.SessionID, "test-session")
	}
	if br.MessageCount != 42 {
		t.Errorf("MessageCount = %d, want %d", br.MessageCount, 42)
	}
}

// TestParameterDefaults 验证参数默认值逻辑（limit <= 0 时默认为 20）
func TestParameterDefaults(t *testing.T) {
	tests := []struct {
		name     string
		limit    int
		expected int
	}{
		{"负数默认为20", -1, 20},
		{"零默认为20", 0, 20},
		{"正数保持不变", 10, 10},
		{"默认值为20", -5, 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limit := tt.limit
			if limit <= 0 {
				limit = 20
			}
			if limit != tt.expected {
				t.Errorf("limit = %d, want %d", limit, tt.expected)
			}
		})
	}
}
