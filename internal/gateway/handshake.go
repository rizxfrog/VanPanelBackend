package gateway

import (
	"context"
	"fmt"
)

// HandleConnect validates the connect handshake, authenticates the client,
// and constructs a HelloOk response.
func HandleConnect(
	ctx context.Context,
	conn *GatewayConnection,
	params *ConnectParams,
	authHandler *AuthHandler,
	presenceTracker *PresenceTracker,
	healthState *HealthState,
	config *GatewayConfig,
) (*HelloOk, error) {
	// Protocol version check.
	if params.MinProtocol > ProtocolVersion {
		return nil, fmt.Errorf("client requires protocol >= %d, server supports %d",
			params.MinProtocol, ProtocolVersion)
	}
	if params.MaxProtocol < ProtocolVersion {
		return nil, fmt.Errorf("client requires protocol <= %d, server supports %d",
			params.MaxProtocol, ProtocolVersion)
	}

	// Authenticate the client.
	role := "guest"
	var scopes []string

	if authHandler != nil {
		var err error
		role, scopes, err = authHandler.Validate(params)
		if err != nil {
			return nil, fmt.Errorf("authentication failed: %w", err)
		}
	}

	// Set auth and client info on the connection.
	conn.SetAuth(role, scopes)
	conn.SetClient(params.Client)

	// Build the server state snapshot.
	snapshot := BuildSnapshot(presenceTracker, healthState)

	// Build the hello-ok response.
	helloOk := BuildHelloOk(
		conn.ID,
		config.ServerVersion,
		config.Methods,
		config.Events,
		snapshot,
		role,
		scopes,
	)

	return helloOk, nil
}

// BuildSnapshot creates a Snapshot of the current server state from
// the presence tracker and health state.
func BuildSnapshot(presenceTracker *PresenceTracker, healthState *HealthState) *Snapshot {
	snapshot := &Snapshot{
		Presence: []PresenceEntry{},
		Health:   nil,
		StateVersion: StateVersion{
			Presence: 0,
			Health:   0,
		},
		SessionDefaults: SessionDefaults{
			DefaultAgentID: "main",
			MainKey:        "agent:main:global",
			MainSessionKey: "agent:main:global",
			Scope:          "global",
		},
	}

	if presenceTracker != nil {
		snapshot.Presence = presenceTracker.List()
		if snapshot.Presence == nil {
			snapshot.Presence = []PresenceEntry{}
		}
		snapshot.StateVersion.Presence = presenceTracker.Version()
	}

	if healthState != nil {
		healthData, healthVersion := healthState.Get()
		snapshot.Health = healthData
		snapshot.StateVersion.Health = healthVersion
	}

	return snapshot
}
