package gateway

import (
	"context"

	"go.uber.org/zap"
)

// ChatStreamAdapter wraps the VanPanel agent streaming into OpenClaw chat events.
// The agent service produces SSE events; we convert them to Gateway protocol chat events.
type ChatStreamAdapter struct {
	Logger *zap.Logger
}

// ChatStreamHandler defines how to stream a chat response.
type ChatStreamHandler func(ctx context.Context, conn *GatewayConnection, run *ChatRunState, message string) error

// NewChatStreamAdapter creates a new chat stream adapter.
func NewChatStreamAdapter(logger *zap.Logger) *ChatStreamAdapter {
	return &ChatStreamAdapter{Logger: logger}
}

// SendDelta sends a text delta event for a chat run.
func (a *ChatStreamAdapter) SendDelta(conn *GatewayConnection, run *ChatRunState, text string, fullContent string) {
	run.Seq++
	replace := false
	event := ChatEvent{
		RunID:      run.RunID,
		SessionKey: run.SessionKey,
		AgentID:    run.AgentID,
		Seq:        run.Seq,
		State:      ChatStateDelta,
		DeltaText:  text,
		Replace:    &replace,
		Message: ChatMessage{
			Role: "assistant",
			Content: []ContentBlock{
				{Type: "text", Text: fullContent},
			},
		},
	}
	conn.SendEvent("chat", event)
}

// SendFinal sends the final completion event.
func (a *ChatStreamAdapter) SendFinal(conn *GatewayConnection, run *ChatRunState, fullContent string) {
	run.Seq++
	event := ChatEvent{
		RunID:      run.RunID,
		SessionKey: run.SessionKey,
		AgentID:    run.AgentID,
		Seq:        run.Seq,
		State:      ChatStateFinal,
		Message: ChatMessage{
			Role: "assistant",
			Content: []ContentBlock{
				{Type: "text", Text: fullContent},
			},
		},
	}
	conn.SendEvent("chat", event)
}

// SendError sends an error event.
func (a *ChatStreamAdapter) SendError(conn *GatewayConnection, run *ChatRunState, errorMsg string) {
	run.Seq++
	event := ChatEvent{
		RunID:      run.RunID,
		SessionKey: run.SessionKey,
		AgentID:    run.AgentID,
		Seq:        run.Seq,
		State:      ChatStateError,
		ErrorMsg:   errorMsg,
		ErrorKind:  "unknown",
	}
	conn.SendEvent("chat", event)
}

// SendAbort sends an abort event.
func (a *ChatStreamAdapter) SendAbort(conn *GatewayConnection, run *ChatRunState) {
	run.Seq++
	event := ChatEvent{
		RunID:      run.RunID,
		SessionKey: run.SessionKey,
		AgentID:    run.AgentID,
		Seq:        run.Seq,
		State:      ChatStateAborted,
		StopReason: "user_aborted",
	}
	conn.SendEvent("chat", event)
}
