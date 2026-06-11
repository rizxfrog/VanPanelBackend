package gateway

import (
	"sync"
	"time"

	"go.uber.org/zap"
)

// BroadcastManager manages a set of gateway connections and provides
// broadcast/unicast event delivery with periodic tick heartbeat.
type BroadcastManager struct {
	mu              sync.RWMutex
	conns           map[string]*GatewayConnection
	logger          *zap.Logger
	presenceTracker *PresenceTracker
}

// NewBroadcastManager creates a new BroadcastManager.
func NewBroadcastManager(logger *zap.Logger) *BroadcastManager {
	return &BroadcastManager{
		conns:  make(map[string]*GatewayConnection),
		logger: logger,
	}
}

// Add registers a connection in the broadcast set.
func (b *BroadcastManager) Add(conn *GatewayConnection) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.conns[conn.ID] = conn
	b.logger.Debug("connection added to broadcast set", zap.String("connID", conn.ID))
}

// Remove unregisters a connection from the broadcast set.
func (b *BroadcastManager) Remove(connID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.conns, connID)
	b.logger.Debug("connection removed from broadcast set", zap.String("connID", connID))
}

// Broadcast sends an event to all registered connections. Errors are logged and ignored.
func (b *BroadcastManager) Broadcast(eventName string, payload interface{}) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, conn := range b.conns {
		if err := conn.SendEvent(eventName, payload); err != nil {
			b.logger.Debug("broadcast send failed", zap.String("connID", conn.ID), zap.String("event", eventName), zap.Error(err))
		}
	}
}

// BroadcastTo sends an event to a specific connection by ID. Does nothing if the
// connection is not found.
func (b *BroadcastManager) BroadcastTo(connID string, eventName string, payload interface{}) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	conn, ok := b.conns[connID]
	if !ok {
		return
	}
	if err := conn.SendEvent(eventName, payload); err != nil {
		b.logger.Debug("unicast send failed", zap.String("connID", connID), zap.String("event", eventName), zap.Error(err))
	}
}

// StartTick starts a goroutine that sends periodic tick heartbeat events to the
// connection. The goroutine exits when the connection's context is done.
func (b *BroadcastManager) StartTick(conn *GatewayConnection) {
	go func() {
		interval := time.Duration(TickIntervalMs) * time.Millisecond
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-conn.Context().Done():
				return
			case <-ticker.C:
			}
			ts := time.Now().UnixMilli()
			if err := conn.SendEvent("tick", TickPayload{TS: ts}); err != nil {
				b.logger.Debug("tick send failed", zap.String("connID", conn.ID), zap.Error(err))
			}
		}
	}()
}

// Count returns the number of registered connections.
func (b *BroadcastManager) Count() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.conns)
}

// SetPresenceTracker sets the presence tracker reference.
func (b *BroadcastManager) SetPresenceTracker(pt *PresenceTracker) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.presenceTracker = pt
}
