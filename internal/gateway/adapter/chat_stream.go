package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/rizxfrog/VanPanelBackend/internal/gateway"
)

// ChatStreamAdapter adapts Eino Agent SSE streaming to OpenClaw Gateway chat events.
// It implements io.Writer to receive SSE-formatted events from AgentService.QueryStream().
type ChatStreamAdapter struct {
	conn       *gateway.GatewayConnection
	runID      string
	sessionKey string
	agentID    string
	seq        atomic.Int32
	builder    strings.Builder // accumulates assistant text
	ctx        context.Context
}

// NewChatStreamAdapter creates a new streaming adapter for the given connection.
func NewChatStreamAdapter(ctx context.Context, conn *gateway.GatewayConnection, runID, sessionKey, agentID string) *ChatStreamAdapter {
	return &ChatStreamAdapter{
		conn:       conn,
		runID:      runID,
		sessionKey: sessionKey,
		agentID:    agentID,
		ctx:        ctx,
	}
}

// Write implements io.Writer — receives SSE events from AgentService.
// Format: "event: <type>\ndata: <json>\n\n"
func (a *ChatStreamAdapter) Write(p []byte) (n int, err error) {
	n = len(p)
	text := string(p)

	// Parse SSE events (may contain multiple events)
	events := strings.Split(strings.TrimSpace(text), "\n\n")
	for _, event := range events {
		if event == "" {
			continue
		}
		a.handleSSEEvent(event)
	}
	return n, nil
}

func (a *ChatStreamAdapter) handleSSEEvent(event string) {
	select {
	case <-a.ctx.Done():
		return
	default:
	}

	lines := strings.Split(strings.TrimSpace(event), "\n")
	var eventType, data string
	for _, line := range lines {
		if strings.HasPrefix(line, "event: ") {
			eventType = strings.TrimPrefix(line, "event: ")
		} else if strings.HasPrefix(line, "data: ") {
			data = strings.TrimPrefix(line, "data: ")
		}
	}

	switch eventType {
	case "delta":
		a.handleDelta(data)
	case "tool_call":
		a.handleToolCall(data)
	case "tool_result":
		a.handleToolResult(data)
	case "done":
		a.handleDone(data)
	case "error":
		a.handleError(data)
	}
}

func (a *ChatStreamAdapter) handleDelta(data string) {
	var m map[string]string
	if err := json.Unmarshal([]byte(data), &m); err != nil {
		return
	}
	delta := m["content"]
	if delta == "" {
		return
	}

	a.builder.WriteString(delta)
	full := a.builder.String()

	seq := int(a.seq.Add(1))
	replace := false
	a.conn.SendEvent("chat", gateway.ChatEvent{
		RunID:      a.runID,
		SessionKey: a.sessionKey,
		AgentID:    a.agentID,
		Seq:        seq,
		State:      gateway.ChatStateDelta,
		DeltaText:  delta,
		Replace:    &replace,
		Message: gateway.ChatMessage{
			Role: "assistant",
			Content: []gateway.ContentBlock{
				{Type: "text", Text: full},
			},
		},
	})
}

func (a *ChatStreamAdapter) handleToolCall(data string) {
	var m map[string]string
	if err := json.Unmarshal([]byte(data), &m); err != nil {
		return
	}
	toolID := m["id"]
	toolName := m["name"]
	args := m["arguments"]

	seq := int(a.seq.Add(1))
	a.conn.SendEvent("chat", gateway.ChatEvent{
		RunID:      a.runID,
		SessionKey: a.sessionKey,
		AgentID:    a.agentID,
		Seq:        seq,
		State:      gateway.ChatStateDelta,
		Message: gateway.ChatMessage{
			Role: "assistant",
			Content: []gateway.ContentBlock{
				{Type: "tool_use", Text: fmt.Sprintf(`{"id":"%s","name":"%s","input":%s}`, toolID, toolName, args)},
			},
		},
	})
}

func (a *ChatStreamAdapter) handleToolResult(data string) {
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(data), &m); err != nil {
		return
	}
	toolID, _ := m["id"].(string)
	toolName, _ := m["name"].(string)
	result, _ := json.Marshal(m["result"])

	seq := int(a.seq.Add(1))
	a.conn.SendEvent("chat", gateway.ChatEvent{
		RunID:      a.runID,
		SessionKey: a.sessionKey,
		AgentID:    a.agentID,
		Seq:        seq,
		State:      gateway.ChatStateDelta,
		Message: gateway.ChatMessage{
			Role: "tool",
			Content: []gateway.ContentBlock{
				{Type: "tool_result", Text: fmt.Sprintf(`{"id":"%s","name":"%s","result":%s}`, toolID, toolName, string(result))},
			},
		},
	})
}

func (a *ChatStreamAdapter) handleDone(data string) {
	full := a.builder.String()
	seq := int(a.seq.Add(1))
	a.conn.SendEvent("chat", gateway.ChatEvent{
		RunID:      a.runID,
		SessionKey: a.sessionKey,
		AgentID:    a.agentID,
		Seq:        seq,
		State:      gateway.ChatStateFinal,
		Message: gateway.ChatMessage{
			Role: "assistant",
			Content: []gateway.ContentBlock{
				{Type: "text", Text: full},
			},
		},
	})
}

func (a *ChatStreamAdapter) handleError(data string) {
	var m map[string]string
	if err := json.Unmarshal([]byte(data), &m); err != nil {
		return
	}
	errMsg := m["error"]
	if errMsg == "" {
		errMsg = "agent error"
	}
	seq := int(a.seq.Add(1))
	a.conn.SendEvent("chat", gateway.ChatEvent{
		RunID:      a.runID,
		SessionKey: a.sessionKey,
		AgentID:    a.agentID,
		Seq:        seq,
		State:      gateway.ChatStateError,
		ErrorMsg:   errMsg,
		ErrorKind:  "agent_error",
	})
}
