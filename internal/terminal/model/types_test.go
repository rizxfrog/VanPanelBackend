package model

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestMessageRoundTrip(t *testing.T) {
	raw := []byte(`{"type":"resize","sessionId":"s1","cols":120,"rows":32}`)
	var msg Message
	if err := json.Unmarshal(raw, &msg); err != nil {
		t.Fatal(err)
	}
	if msg.Type != MessageTypeResize || msg.SessionID != "s1" || msg.Cols != 120 || msg.Rows != 32 {
		t.Fatalf("message = %+v", msg)
	}
	out, err := json.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(out) {
		t.Fatalf("marshaled invalid json: %s", out)
	}
}

func TestConnectMessageValidation(t *testing.T) {
	msg := Message{Type: MessageTypeConnect, TargetType: TargetTypeLocal, TargetID: LocalTargetID}
	if err := msg.ValidateFirst(); err != nil {
		t.Fatalf("ValidateFirst error = %v", err)
	}

	msg = Message{Type: MessageTypeInput, Data: "ls"}
	if err := msg.ValidateFirst(); !errors.Is(err, ErrInvalidMessageType) {
		t.Fatalf("ValidateFirst error = %v, want ErrInvalidMessageType", err)
	}
}

func TestResizeValidation(t *testing.T) {
	msg := Message{Type: MessageTypeResize, SessionID: "s1", Cols: 120, Rows: 32}
	if err := msg.Validate(); err != nil {
		t.Fatalf("Validate error = %v", err)
	}

	msg = Message{Type: MessageTypeResize, SessionID: "s1", Cols: 0, Rows: 32}
	if err := msg.Validate(); !errors.Is(err, ErrInvalidResize) {
		t.Fatalf("Validate error = %v, want ErrInvalidResize", err)
	}
}
