package adapter

import (
	"context"
	"encoding/json"
	"strings"
	"sync/atomic"
	"time"

	"github.com/rizxfrog/VanPanelBackend/internal/gateway"
)

// ChatStreamAdapter adapts Eino Agent SSE streaming to OpenClaw Gateway chat/agent events.
// It implements io.Writer to receive SSE-formatted events from AgentService.QueryStream().
type ChatStreamAdapter struct {
	conn       *gateway.GatewayConnection
	runID      string
	sessionKey string
	agentID    string
	seq        atomic.Int32
	builder    strings.Builder // accumulates assistant text for streaming display
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
				{Type: "text", Text: a.builder.String()},
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

	// Parse args from JSON string to interface{} for structured output
	var argsObj interface{}
	if args != "" {
		json.Unmarshal([]byte(args), &argsObj)
	}

	seq := int(a.seq.Add(1))
	a.conn.SendEvent("agent", gateway.AgentToolPayload{
		RunID:      a.runID,
		Seq:        seq,
		Stream:     "tool",
		TS:         time.Now().UnixMilli(),
		SessionKey: a.sessionKey,
		AgentID:    a.agentID,
		Data: gateway.ToolStreamData{
			ToolCallID: toolID,
			Name:       toolName,
			Phase:      "start",
			Args:       argsObj,
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

	// Extract result text - may be string or complex object
	var resultText string
	if s, ok := m["result"].(string); ok {
		resultText = s
	} else {
		b, _ := json.Marshal(m["result"])
		resultText = string(b)
	}

	// Check error status
	isError := false
	if status, ok := m["status"].(string); ok && status == "error" {
		isError = true
	}

	seq := int(a.seq.Add(1))
	a.conn.SendEvent("agent", gateway.AgentToolPayload{
		RunID:      a.runID,
		Seq:        seq,
		Stream:     "tool",
		TS:         time.Now().UnixMilli(),
		SessionKey: a.sessionKey,
		AgentID:    a.agentID,
		Data: gateway.ToolStreamData{
			ToolCallID: toolID,
			Name:       toolName,
			Phase:      "result",
			Result:     resultText,
			IsError:    isError,
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
