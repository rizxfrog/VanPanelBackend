package gateway

import (
	"context"
	"sync"
	"time"
)

// RunEntry tracks a single active chat run for abort support.
type RunEntry struct {
	RunID      string
	SessionKey string
	ConnID     string
	Cancel     context.CancelFunc
	StartedAt  time.Time
}

// RunTracker tracks active chat runs so chat.abort can cancel them.
// Thread-safe for concurrent access from multiple goroutines.
type RunTracker struct {
	mu   sync.RWMutex
	runs map[string]*RunEntry // runID → entry
}

// NewRunTracker creates a new RunTracker.
func NewRunTracker() *RunTracker {
	return &RunTracker{
		runs: make(map[string]*RunEntry),
	}
}

// Register adds an active run to the tracker.
func (rt *RunTracker) Register(runID, sessionKey, connID string, cancel context.CancelFunc) *RunEntry {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	entry := &RunEntry{
		RunID:      runID,
		SessionKey: sessionKey,
		ConnID:     connID,
		Cancel:     cancel,
		StartedAt:  time.Now(),
	}
	rt.runs[runID] = entry
	return entry
}

// Get retrieves an active run by ID.
func (rt *RunTracker) Get(runID string) (*RunEntry, bool) {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	entry, ok := rt.runs[runID]
	return entry, ok
}

// GetBySession returns all active runs for a given session key.
func (rt *RunTracker) GetBySession(sessionKey string) []*RunEntry {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	var entries []*RunEntry
	for _, entry := range rt.runs {
		if entry.SessionKey == sessionKey {
			entries = append(entries, entry)
		}
	}
	return entries
}

// Unregister removes a run from the tracker. Idempotent — safe to call
// multiple times for the same runID.
func (rt *RunTracker) Unregister(runID string) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	delete(rt.runs, runID)
}

// Count returns the number of active tracked runs.
func (rt *RunTracker) Count() int {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	return len(rt.runs)
}
