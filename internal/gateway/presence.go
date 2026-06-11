package gateway

import (
	"sync"

	"go.uber.org/zap"
)

// PresenceTracker tracks which clients are currently connected. It maintains
// a version counter that increments on every mutation, enabling clients to
// efficiently detect changes by comparing version numbers.
type PresenceTracker struct {
	mu      sync.Mutex
	entries map[string]*PresenceEntry
	version int
	logger  *zap.Logger
}

// NewPresenceTracker creates a new PresenceTracker.
func NewPresenceTracker(logger *zap.Logger) *PresenceTracker {
	return &PresenceTracker{
		entries: make(map[string]*PresenceEntry),
		logger:  logger,
	}
}

// Upsert adds or updates a presence entry and increments the version counter.
func (p *PresenceTracker) Upsert(key string, entry *PresenceEntry) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.entries[key] = entry
	p.version++
}

// Remove deletes a presence entry and increments the version counter.
func (p *PresenceTracker) Remove(key string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.entries, key)
	p.version++
}

// List returns a copy of all presence entries as a slice.
func (p *PresenceTracker) List() []PresenceEntry {
	p.mu.Lock()
	defer p.mu.Unlock()
	result := make([]PresenceEntry, 0, len(p.entries))
	for _, entry := range p.entries {
		result = append(result, *entry)
	}
	return result
}

// Version returns the current version counter.
func (p *PresenceTracker) Version() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.version
}

// StateVersion returns a StateVersion with the current presence version and
// health set to 0.
func (p *PresenceTracker) StateVersion() StateVersion {
	p.mu.Lock()
	defer p.mu.Unlock()
	return StateVersion{
		Presence: p.version,
		Health:   0,
	}
}
