package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// GatewayConfig holds configuration for the gateway server.
type GatewayConfig struct {
	ServerVersion string
	Methods       []string
	Events        []string
}

// DefaultGatewayConfig returns the default gateway configuration.
func DefaultGatewayConfig() *GatewayConfig {
	return &GatewayConfig{
		ServerVersion: "vanpanel-0.1.0",
		Methods: []string{
			"chat.send", "chat.abort", "chat.history",
			"health", "presence.list", "sessions.list",
			"agent.list", "agent.get", "config.get",
			"connect",
		},
		Events: []string{
			"tick", "chat", "presence", "health",
			"shutdown", "config.changed", "approval.requested",
		},
	}
}

// GatewayServer manages WebSocket connections and the connection lifecycle.
type GatewayServer struct {
	logger          *zap.Logger
	upgrader        websocket.Upgrader
	broadcastMgr    *BroadcastManager
	presenceTracker *PresenceTracker
	healthState     *HealthState
	authHandler     *AuthHandler
	config          *GatewayConfig
}

// NewGatewayServer creates a new GatewayServer.
func NewGatewayServer(
	logger *zap.Logger,
	broadcastMgr *BroadcastManager,
	presenceTracker *PresenceTracker,
	healthState *HealthState,
	authHandler *AuthHandler,
	config *GatewayConfig,
) *GatewayServer {
	if config == nil {
		config = DefaultGatewayConfig()
	}
	if config.ServerVersion == "" {
		config.ServerVersion = "vanpanel-0.1.0"
	}
	if config.Methods == nil {
		config.Methods = []string{"chat.send", "chat.abort", "health"}
	}
	if config.Events == nil {
		config.Events = []string{"tick", "chat", "presence", "health", "shutdown"}
	}

	return &GatewayServer{
		logger: logger,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
		broadcastMgr:    broadcastMgr,
		presenceTracker: presenceTracker,
		healthState:     healthState,
		authHandler:     authHandler,
		config:          config,
	}
}

// ServeWS handles a WebSocket upgrade request and starts the connection
// lifecycle in a background goroutine.
func (s *GatewayServer) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("failed to upgrade websocket", zap.Error(err))
		return
	}

	// Generate a random challenge nonce and send it immediately after connect.
	challengeNonce := uuid.New().String()
	challengeEvent := NewEvent("challenge", map[string]string{"nonce": challengeNonce})
	if err := conn.WriteJSON(challengeEvent); err != nil {
		s.logger.Warn("failed to send challenge nonce", zap.Error(err))
		conn.Close()
		return
	}

	go s.handleConnection(conn)
}

// handleConnection manages the full lifecycle of a single WebSocket connection.
func (s *GatewayServer) handleConnection(conn *websocket.Conn) {
	gwConn := NewGatewayConnection(conn, s.logger)
	defer gwConn.Close()

	connID := gwConn.ID
	logger := s.logger.With(zap.String("connID", connID))

	// Register the connection on the broadcaster.
	s.broadcastMgr.Add(gwConn)
	defer s.broadcastMgr.Remove(connID)

	// Clean up from presence tracker on disconnect.
	defer s.presenceTracker.Remove(connID)

	// Start tick goroutine via the broadcast manager.
	s.broadcastMgr.StartTick(gwConn)

	// Read loop.
	handshakeComplete := false
	for {
		_, message, err := gwConn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				logger.Info("connection closed normally")
			} else if websocket.IsUnexpectedCloseError(err) {
				logger.Warn("connection closed unexpectedly", zap.Error(err))
			} else {
				if !handshakeComplete {
					logger.Warn("connection closed before handshake", zap.Error(err))
				} else {
					logger.Info("connection read error", zap.Error(err))
				}
			}
			break
		}

		// All messages are expected to be request frames.
		var req RequestFrame
		if err := json.Unmarshal(message, &req); err != nil {
			logger.Warn("invalid frame", zap.Error(err))
			gwConn.SendError("", ErrCodeInvalidRequest, "invalid frame")
			continue
		}
		if req.Type != "req" {
			logger.Warn("unexpected frame type", zap.String("type", req.Type))
			gwConn.SendError(req.ID, ErrCodeInvalidRequest, "only request frames are accepted")
			continue
		}

		if !handshakeComplete {
			// The first message must be a connect handshake request.
			if req.Method != "connect" {
				logger.Warn("expected connect handshake", zap.String("method", req.Method))
				gwConn.SendError(req.ID, ErrCodeInvalidRequest, "first message must be a connect handshake")
				break
			}

			// Parse the ConnectParams from the request.
			params, err := parseConnectParams(message)
			if err != nil {
				logger.Warn("invalid connect params", zap.Error(err))
				gwConn.SendError(req.ID, ErrCodeInvalidRequest, fmt.Sprintf("invalid connect params: %v", err))
				break
			}

			// Perform handshake.
			helloOk, err := HandleConnect(
				gwConn.Context(),
				gwConn,
				params,
				s.authHandler,
				s.presenceTracker,
				s.healthState,
				s.config,
			)
			if err != nil {
				logger.Warn("handshake failed", zap.Error(err))
				gwConn.SendError(req.ID, ErrCodeInvalidRequest, err.Error())
				break
			}

			// Send hello-ok response wrapped in a response frame.
			if err := gwConn.SendJSON(NewResponse(req.ID, helloOk)); err != nil {
				logger.Warn("failed to send hello-ok", zap.Error(err))
				break
			}

			// Register in presence tracker after successful handshake.
			presenceEntry := &PresenceEntry{
				Platform:        gwConn.Client.Platform,
				Version:         gwConn.Client.Version,
				Mode:            string(gwConn.Client.Mode),
				DeviceFamily:    gwConn.Client.DeviceFamily,
				ModelIdentifier: gwConn.Client.ModelIdentifier,
				InstanceID:      gwConn.Client.InstanceID,
				Roles:           []string{gwConn.Role},
				Scopes:          gwConn.Scopes,
				TS:              time.Now().UnixMilli(),
			}
			gwConn.PresenceKey = connID
			gwConn.Presence = *presenceEntry
			s.presenceTracker.Upsert(connID, presenceEntry)

			handshakeComplete = true
			logger.Info("handshake complete",
				zap.String("clientID", string(gwConn.Client.ID)),
				zap.String("role", gwConn.Role),
			)
		} else {
			// After handshake, dispatch all messages to the dispatcher.
			Dispatch(gwConn, &req)
		}
	}
}

// parseConnectParams extracts ConnectParams from a raw JSON request frame.
func parseConnectParams(data []byte) (*ConnectParams, error) {
	var frame struct {
		Params json.RawMessage `json:"params"`
	}
	if err := json.Unmarshal(data, &frame); err != nil {
		return nil, fmt.Errorf("invalid request frame: %w", err)
	}

	var params ConnectParams
	if err := json.Unmarshal(frame.Params, &params); err != nil {
		return nil, fmt.Errorf("invalid connect params: %w", err)
	}

	return &params, nil
}
