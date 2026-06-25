package gateway

import "sync"

// SubscriptionHub tracks which connections are subscribed to session
// list changes and per-session message events. Thread-safe.
type SubscriptionHub struct {
	mu          sync.RWMutex
	sessionSubs map[string]bool            // connID → subscribed to all session changes
	msgSubs     map[string]map[string]bool // connID → {sessionKey: true}
}

// NewSubscriptionHub creates a new SubscriptionHub.
func NewSubscriptionHub() *SubscriptionHub {
	return &SubscriptionHub{
		sessionSubs: make(map[string]bool),
		msgSubs:     make(map[string]map[string]bool),
	}
}

// SubscribeSession subscribes a connection to global session list changes.
func (h *SubscriptionHub) SubscribeSession(connID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.sessionSubs[connID] = true
}

// UnsubscribeSession removes a connection from global session list subscription.
func (h *SubscriptionHub) UnsubscribeSession(connID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.sessionSubs, connID)
}

// IsSubscribedToSession checks if a connection is subscribed to session list changes.
func (h *SubscriptionHub) IsSubscribedToSession(connID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.sessionSubs[connID]
}

// SubscribeMessages subscribes a connection to message events for a specific session.
func (h *SubscriptionHub) SubscribeMessages(connID, sessionKey string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.msgSubs[connID] == nil {
		h.msgSubs[connID] = make(map[string]bool)
	}
	h.msgSubs[connID][sessionKey] = true
}

// UnsubscribeMessages removes a connection from message events for a specific session.
func (h *SubscriptionHub) UnsubscribeMessages(connID, sessionKey string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if subs, ok := h.msgSubs[connID]; ok {
		delete(subs, sessionKey)
		if len(subs) == 0 {
			delete(h.msgSubs, connID)
		}
	}
}

// IsSubscribedToMessages checks if a connection is subscribed to message events for a session.
func (h *SubscriptionHub) IsSubscribedToMessages(connID, sessionKey string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	subs, ok := h.msgSubs[connID]
	if !ok {
		return false
	}
	return subs[sessionKey]
}

// GetMessageSubscribers returns all connection IDs subscribed to messages for a session.
func (h *SubscriptionHub) GetMessageSubscribers(sessionKey string) []string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var subscribers []string
	for connID, subs := range h.msgSubs {
		if subs[sessionKey] {
			subscribers = append(subscribers, connID)
		}
	}
	return subscribers
}

// GetSessionSubscribers returns all connection IDs subscribed to session list changes.
func (h *SubscriptionHub) GetSessionSubscribers() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	subscribers := make([]string, 0, len(h.sessionSubs))
	for connID := range h.sessionSubs {
		subscribers = append(subscribers, connID)
	}
	return subscribers
}

// CleanupConn removes all subscriptions for a connection. Call when the
// connection closes.
func (h *SubscriptionHub) CleanupConn(connID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.sessionSubs, connID)
	delete(h.msgSubs, connID)
}
