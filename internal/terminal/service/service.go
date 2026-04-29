package service

import (
	"context"
	"io"
	"sync"
	"time"

	terminalmodel "github.com/GoSimplicity/AI-CloudOps/internal/terminal/model"
	"github.com/GoSimplicity/AI-CloudOps/internal/terminal/pty"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type Stream interface {
	io.ReadWriteCloser
	Resize(cols, rows int) error
	Wait() error
}

type LocalAdapter interface {
	Start(cols, rows int) (pty.Session, error)
}

type SSHAdapter interface {
	ListTargets(ctx context.Context) ([]terminalmodel.Target, error)
	Start(ctx context.Context, targetID string, cols, rows int) (Stream, terminalmodel.Target, error)
}

type Config struct {
	IdleTimeout time.Duration
}

type StartRequest struct {
	UserID     int
	Username   string
	SourceIP   string
	TargetType string
	TargetID   string
	Cols       int
	Rows       int
}

type Session struct {
	terminalmodel.SessionMetadata
	Stream Stream
}

type TerminalService interface {
	ListTargets(ctx context.Context) ([]terminalmodel.Target, error)
	Start(ctx context.Context, req StartRequest) (*Session, error)
	Get(sessionID string) (*Session, bool)
	Close(sessionID string, reason string) error
	ListSessions(userID int) []terminalmodel.SessionMetadata
}

type terminalService struct {
	logger       *zap.Logger
	localAdapter LocalAdapter
	sshAdapter   SSHAdapter
	config       Config
	mu           sync.RWMutex
	sessions     map[string]*Session
}

func NewTerminalService(logger *zap.Logger, localAdapter LocalAdapter, sshAdapter SSHAdapter, config Config) TerminalService {
	if logger == nil {
		logger = zap.NewNop()
	}
	if config.IdleTimeout == 0 {
		config.IdleTimeout = 30 * time.Minute
	}
	return &terminalService{
		logger:       logger,
		localAdapter: localAdapter,
		sshAdapter:   sshAdapter,
		config:       config,
		sessions:     make(map[string]*Session),
	}
}

func (s *terminalService) ListTargets(ctx context.Context) ([]terminalmodel.Target, error) {
	targets := []terminalmodel.Target{{Type: terminalmodel.TargetTypeLocal, ID: terminalmodel.LocalTargetID, Name: "Local Shell"}}
	if s.sshAdapter == nil {
		return targets, nil
	}
	sshTargets, err := s.sshAdapter.ListTargets(ctx)
	if err != nil {
		return targets, err
	}
	return append(targets, sshTargets...), nil
}

func (s *terminalService) Start(ctx context.Context, req StartRequest) (*Session, error) {
	if req.Cols <= 0 {
		req.Cols = 80
	}
	if req.Rows <= 0 {
		req.Rows = 24
	}

	var stream Stream
	target := terminalmodel.Target{Type: req.TargetType, ID: req.TargetID}
	var err error

	switch req.TargetType {
	case terminalmodel.TargetTypeLocal:
		if req.TargetID != terminalmodel.LocalTargetID || s.localAdapter == nil {
			return nil, terminalmodel.ErrInvalidTarget
		}
		stream, err = s.localAdapter.Start(req.Cols, req.Rows)
		target.Name = "Local Shell"
	case terminalmodel.TargetTypeSSH:
		if s.sshAdapter == nil {
			return nil, terminalmodel.ErrInvalidTarget
		}
		stream, target, err = s.sshAdapter.Start(ctx, req.TargetID, req.Cols, req.Rows)
	default:
		return nil, terminalmodel.ErrInvalidTarget
	}
	if err != nil {
		return nil, err
	}

	session := &Session{
		SessionMetadata: terminalmodel.SessionMetadata{
			ID:         uuid.NewString(),
			UserID:     req.UserID,
			Username:   req.Username,
			TargetType: target.Type,
			TargetID:   target.ID,
			TargetName: target.Name,
			SourceIP:   req.SourceIP,
			StartedAt:  time.Now(),
		},
		Stream: stream,
	}

	s.mu.Lock()
	s.sessions[session.ID] = session
	s.mu.Unlock()
	return session, nil
}

func (s *terminalService) Get(sessionID string) (*Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.sessions[sessionID]
	return session, ok
}

func (s *terminalService) Close(sessionID string, reason string) error {
	s.mu.Lock()
	session, ok := s.sessions[sessionID]
	if ok {
		delete(s.sessions, sessionID)
		session.EndedAt = time.Now()
		session.ExitReason = reason
	}
	s.mu.Unlock()
	if !ok {
		return terminalmodel.ErrInvalidSession
	}
	return session.Stream.Close()
}

func (s *terminalService) ListSessions(userID int) []terminalmodel.SessionMetadata {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]terminalmodel.SessionMetadata, 0)
	for _, session := range s.sessions {
		if session.UserID == userID {
			out = append(out, session.SessionMetadata)
		}
	}
	return out
}
