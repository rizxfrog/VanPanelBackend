package gateway

import (
	"encoding/json"
	"fmt"
)

// EncodeFrame serializes an arbitrary value as JSON.
func EncodeFrame(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

// DecodeFrame does a lightweight frame-type check and returns the raw type field.
// Full decoding is done in the dispatcher based on the method.
func DecodeFrame(data []byte) (frameType string, frameID string, method string, err error) {
	var raw struct {
		Type   string `json:"type"`
		ID     string `json:"id"`
		Method string `json:"method,omitempty"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return "", "", "", fmt.Errorf("invalid frame: %w", err)
	}
	if raw.Type == "" {
		return "", "", "", fmt.Errorf("frame missing 'type' field")
	}
	return raw.Type, raw.ID, raw.Method, nil
}

// BuildHelloOk constructs a hello-ok response from components.
func BuildHelloOk(connID, serverVersion string, methods, events []string, snapshot *Snapshot, role string, scopes []string) *HelloOk {
	return &HelloOk{
		Type:     "hello-ok",
		Protocol: ProtocolVersion,
		Server: ServerInfo{
			Version: serverVersion,
			ConnID:  connID,
		},
		Features: FeatureList{
			Methods: methods,
			Events:  events,
		},
		Snapshot: snapshot,
		Auth: HelloAuth{
			Role:   role,
			Scopes: scopes,
		},
		Policy: GatewayPolicy{
			MaxPayload:       MaxPayloadBytes,
			MaxBufferedBytes: MaxBufferedBytes,
			TickIntervalMs:   TickIntervalMs,
		},
	}
}

// NewResponse builds a success response frame.
func NewResponse(reqID string, payload interface{}) *ResponseFrame {
	return &ResponseFrame{
		Type:    "res",
		ID:      reqID,
		OK:      true,
		Payload: payload,
	}
}

// NewErrorResponse builds an error response frame.
func NewErrorResponse(reqID, code, message string) *ResponseFrame {
	return &ResponseFrame{
		Type: "res",
		ID:   reqID,
		OK:   false,
		Error: &ErrorShape{
			Code:    code,
			Message: message,
		},
	}
}

// NewEvent builds a server-pushed event frame.
func NewEvent(eventName string, payload interface{}) *EventFrame {
	return &EventFrame{
		Type:    "event",
		Event:   eventName,
		Payload: payload,
	}
}

// NewTickEvent builds a tick heartbeat event.
func NewTickEvent(ts int64) *EventFrame {
	return NewEvent("tick", TickPayload{TS: ts})
}

// ToJSON marshals any value to JSON bytes.
func ToJSON(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}
