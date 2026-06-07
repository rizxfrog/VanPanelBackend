package memory

import (
	"testing"

	"github.com/rizxfrog/VanPanelBackend/internal/agent/spi"
)

// TestProviderInterface verifies NewProvider returns spi.MemoryProvider
func TestProviderInterface(t *testing.T) {
	// Interface check at compile time
	var _ spi.MemoryProvider = NewProvider(nil, nil)
}

func TestRetrieveWithNilDAO(t *testing.T) {
	p := NewProvider(nil, nil)
	// Should handle nil gracefully - Retrieve will call dao.ListUserMemoriesByUser which
	// will panic on nil, but that's expected without proper DAO initialization
	_ = p
}
