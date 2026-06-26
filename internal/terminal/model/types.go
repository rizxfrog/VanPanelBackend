package model

import (
	"errors"
	"time"
)

const (
	TargetTypeLocal = "local"
	TargetTypeSSH   = "ssh"
	LocalTargetID   = "local"
)

const (
	MessageTypeConnect   = "connect"
	MessageTypeConnected = "connected"
	MessageTypeInput     = "input"
	MessageTypeOutput    = "output"
	MessageTypeResize    = "resize"
	MessageTypePing      = "ping"
	MessageTypePong      = "pong"
	MessageTypeError     = "error"
	MessageTypeClose     = "close"
	MessageTypeClosed    = "closed"
)

var (
	ErrInvalidMessageType = errors.New("invalid terminal message type")
	ErrInvalidTarget      = errors.New("invalid terminal target")
	ErrInvalidSession     = errors.New("invalid terminal session")
	ErrInvalidResize      = errors.New("invalid terminal resize")
)

type Target struct {
	Type string `json:"type"`
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Message struct {
	Type       string `json:"type"`
	SessionID  string `json:"sessionId,omitempty"`
	TargetType string `json:"targetType,omitempty"`
	TargetID   string `json:"targetId,omitempty"`
	TargetName string `json:"targetName,omitempty"`
	Data       string `json:"data,omitempty"`
	Cols       int    `json:"cols,omitempty"`
	Rows       int    `json:"rows,omitempty"`
	Message    string `json:"message,omitempty"`
	Reason     string `json:"reason,omitempty"`
}

type SessionMetadata struct {
	ID         string    `json:"id"`
	UserID     int       `json:"userId"`
	Username   string    `json:"username"`
	TargetType string    `json:"targetType"`
	TargetID   string    `json:"targetId"`
	TargetName string    `json:"targetName"`
	SourceIP   string    `json:"sourceIp"`
	StartedAt  time.Time `json:"startedAt"`
	EndedAt    time.Time `json:"endedAt,omitempty"`
	ExitReason string    `json:"exitReason,omitempty"`
	Error      string    `json:"error,omitempty"`
}

func (m Message) ValidateFirst() error {
	if m.Type != MessageTypeConnect {
		return ErrInvalidMessageType
	}
	if m.TargetType != TargetTypeLocal && m.TargetType != TargetTypeSSH {
		return ErrInvalidTarget
	}
	if m.TargetID == "" {
		return ErrInvalidTarget
	}
	return nil
}

func (m Message) Validate() error {
	switch m.Type {
	case MessageTypeConnect:
		return m.ValidateFirst()
	case MessageTypeInput, MessageTypePing, MessageTypeClose:
		if m.SessionID == "" {
			return ErrInvalidSession
		}
	case MessageTypeResize:
		if m.SessionID == "" {
			return ErrInvalidSession
		}
		if m.Cols <= 0 || m.Rows <= 0 {
			return ErrInvalidResize
		}
	}
	return nil
}
