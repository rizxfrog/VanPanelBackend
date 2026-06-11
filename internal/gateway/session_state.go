package gateway

import (
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ChatRunState tracks an active chat run for a session.
type ChatRunState struct {
	RunID      string
	SessionKey string
	AgentID    string
	Seq        int
	CreatedAt  time.Time
	Active     bool
}

// SessionState tracks active runs and subscribers per session.
type SessionState struct {
	ActiveRuns map[string]*ChatRunState // keyed by sessionKey
	Logger     *zap.Logger
}

// NewSessionState creates a new session state tracker.
func NewSessionState(logger *zap.Logger) *SessionState {
	return &SessionState{
		ActiveRuns: make(map[string]*ChatRunState),
		Logger:     logger,
	}
}

// StartRun begins a new chat run for a session.
func (s *SessionState) StartRun(sessionKey, agentID string) *ChatRunState {
	run := &ChatRunState{
		RunID:      uuid.New().String(),
		SessionKey: sessionKey,
		AgentID:    agentID,
		Seq:        0,
		CreatedAt:  time.Now(),
		Active:     true,
	}
	s.ActiveRuns[sessionKey] = run
	return run
}

// GetRun returns the active run for a session, if any.
func (s *SessionState) GetRun(sessionKey string) *ChatRunState {
	return s.ActiveRuns[sessionKey]
}

// AbortRun marks a run as stopped.
func (s *SessionState) AbortRun(sessionKey string) {
	if run := s.ActiveRuns[sessionKey]; run != nil {
		run.Active = false
	}
	delete(s.ActiveRuns, sessionKey)
}

// FinishRun marks a run as complete and returns the run.
func (s *SessionState) FinishRun(sessionKey string) *ChatRunState {
	run := s.ActiveRuns[sessionKey]
	if run != nil {
		run.Active = false
		delete(s.ActiveRuns, sessionKey)
	}
	return run
}
