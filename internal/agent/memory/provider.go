package memory

import (
	"context"
	"strings"

	"github.com/rizxfrog/VanPanelBackend/internal/agent/dao"
	"github.com/rizxfrog/VanPanelBackend/internal/agent/spi"
	"go.uber.org/zap"
)

type provider struct {
	dao    dao.AgentDAO
	logger *zap.Logger
}

// NewProvider creates a MemoryProvider backed by AgentDAO
func NewProvider(d dao.AgentDAO, logger *zap.Logger) spi.MemoryProvider {
	return &provider{dao: d, logger: logger}
}

func (p *provider) Retrieve(ctx context.Context, query string, sessionID string) ([]spi.MemoryEntry, error) {
	// For now: fetch recent memories, filter by keyword match
	// SessionID can be used to exclude current session's temporary messages
	// We use userID 0 as placeholder — in real usage, userID comes from context
	memories, err := p.dao.ListUserMemoriesByUser(ctx, 0, 20)
	if err != nil {
		return nil, err
	}

	var entries []spi.MemoryEntry
	keywords := strings.Fields(strings.ToLower(query))
	for _, m := range memories {
		lowerContent := strings.ToLower(m.Content)
		score := 0.0
		for _, kw := range keywords {
			if strings.Contains(lowerContent, kw) {
				score += 0.3
			}
		}
		if score > 0 {
			entries = append(entries, spi.MemoryEntry{
				Source:  "long_term",
				Content: m.Content,
				Score:   score,
				Metadata: map[string]any{
					"type":       m.MemoryType,
					"importance": m.Importance,
				},
			})
		}
	}
	return entries, nil
}
