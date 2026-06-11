package gateway

import (
	"runtime"
	"sync"
	"time"

	"go.uber.org/zap"
)

// HealthState caches system health information sent to clients in the
// hello-ok snapshot. It tracks a version counter so clients know when
// the health data has changed.
type HealthState struct {
	mu            sync.RWMutex
	data          interface{}
	version       int
	logger        *zap.Logger
	serverVersion string
	startTime     time.Time
}

// NewHealthState creates a new HealthState with initial healthy data.
func NewHealthState(logger *zap.Logger, serverVersion string) *HealthState {
	return &HealthState{
		data: map[string]interface{}{
			"status":  "healthy",
			"version": serverVersion,
		},
		serverVersion: serverVersion,
		logger:        logger,
		startTime:     time.Now(),
	}
}

// Update replaces the health data and increments the version counter.
func (h *HealthState) Update(data interface{}) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.data = data
	h.version++
}

// Get returns the current health data and version counter.
func (h *HealthState) Get() (data interface{}, version int) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.data, h.version
}

// StateVersion returns a StateVersion with presence set to 0 and the
// current health version.
func (h *HealthState) StateVersion() StateVersion {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return StateVersion{
		Presence: 0,
		Health:   h.version,
	}
}

// Start launches the periodic health refresh goroutine.
func (h *HealthState) Start() {
	go h.refreshLoop()
}

func (h *HealthState) refreshLoop() {
	ticker := time.NewTicker(time.Duration(HealthRefreshMs) * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		h.Refresh()
	}
}

// Refresh collects current runtime statistics and updates the health data.
func (h *HealthState) Refresh() {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	h.mu.Lock()
	defer h.mu.Unlock()

	h.data = map[string]interface{}{
		"status":      "healthy",
		"version":     h.serverVersion,
		"uptimeMs":    time.Since(h.startTime).Milliseconds(),
		"heapAlloc":   memStats.HeapAlloc,
		"heapSys":     memStats.HeapSys,
		"memoryAlloc": memStats.Alloc,
		"goroutines":  runtime.NumGoroutine(),
	}
	h.version++
	h.logger.Debug("health state refreshed", zap.Int("version", h.version))
}
