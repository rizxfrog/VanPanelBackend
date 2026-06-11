package gateway

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// GatewayConnection represents a single WebSocket connection to a client.
type GatewayConnection struct {
	ID          string
	Conn        *websocket.Conn
	Role        string
	Scopes      []string
	Client      ClientInfo
	ConnectedAt time.Time
	LastTick    time.Time

	mu     sync.Mutex
	closed bool
	logger *zap.Logger
	ctx    context.Context
	cancel context.CancelFunc

	// Presence info
	PresenceKey string
	Presence    PresenceEntry
}

// NewGatewayConnection creates a new connection wrapper.
func NewGatewayConnection(conn *websocket.Conn, logger *zap.Logger) *GatewayConnection {
	ctx, cancel := context.WithCancel(context.Background())
	return &GatewayConnection{
		ID:          uuid.New().String(),
		Conn:        conn,
		ConnectedAt: time.Now(),
		LastTick:    time.Now(),
		logger:      logger,
		ctx:         ctx,
		cancel:      cancel,
	}
}

// SendJSON sends a JSON message to the client (thread-safe).
func (c *GatewayConnection) SendJSON(v interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	data, err := json.Marshal(v)
	if err != nil {
		return err
	}

	if err := c.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
		c.logger.Warn("failed to write message", zap.String("connID", c.ID), zap.Error(err))
		return err
	}
	return nil
}

// SendResponse sends a response frame to the client.
func (c *GatewayConnection) SendResponse(reqID string, payload interface{}) error {
	return c.SendJSON(NewResponse(reqID, payload))
}

// SendError sends an error response to the client.
func (c *GatewayConnection) SendError(reqID, code, message string) error {
	return c.SendJSON(NewErrorResponse(reqID, code, message))
}

// SendEvent pushes a server event to the client.
func (c *GatewayConnection) SendEvent(eventName string, payload interface{}) error {
	return c.SendJSON(NewEvent(eventName, payload))
}

// SendRawFrame sends a pre-built frame (for chat events etc.).
func (c *GatewayConnection) SendRawFrame(data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	if err := c.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
		c.logger.Warn("failed to write frame", zap.String("connID", c.ID), zap.Error(err))
		return err
	}
	return nil
}

// HasScope checks if the connection has a specific scope.
func (c *GatewayConnection) HasScope(scope string) bool {
	for _, s := range c.Scopes {
		if s == scope || s == string(ScopeAdmin) {
			return true
		}
	}
	// Admin scope implies all lower scopes
	if c.Role == "admin" {
		return true
	}
	return false
}

// ReadMessage reads a raw WebSocket message.
func (c *GatewayConnection) ReadMessage() (int, []byte, error) {
	return c.Conn.ReadMessage()
}

// Close closes the connection.
func (c *GatewayConnection) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	c.closed = true
	c.cancel()
	c.Conn.Close()
}

// Context returns the connection's context.
func (c *GatewayConnection) Context() context.Context {
	return c.ctx
}

// SetAuth sets authentication info after successful handshake.
func (c *GatewayConnection) SetAuth(role string, scopes []string) {
	c.Role = role
	c.Scopes = scopes
}

// SetClient sets client info after successful handshake.
func (c *GatewayConnection) SetClient(client ClientInfo) {
	c.Client = client
}
