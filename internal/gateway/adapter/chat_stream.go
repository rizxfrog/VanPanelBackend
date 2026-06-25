package adapter

import (
	"context"
	"encoding/json"
	"fmt"
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
	toolBlocks []gateway.ContentBlock // tool_use + tool_result blocks in current turn
	bm         *gateway.BroadcastManager
	subHub     *gateway.SubscriptionHub
}

// NewChatStreamAdapter creates a new streaming adapter for the given connection.
func NewChatStreamAdapter(ctx context.Context, conn *gateway.GatewayConnection, runID, sessionKey, agentID string, bm *gateway.BroadcastManager, subHub *gateway.SubscriptionHub) *ChatStreamAdapter {
	return &ChatStreamAdapter{
		conn:       conn,
		runID:      runID,
		sessionKey: sessionKey,
		agentID:    agentID,
		ctx:        ctx,
		bm:         bm,
		subHub:     subHub,
	}
}

// sendChatEvent pushes a chat event to the originating connection AND
// to all other connections subscribed to message events for this session.
func (a *ChatStreamAdapter) sendChatEvent(event gateway.ChatEvent) {
	a.conn.SendEvent("chat", event)
	if a.bm != nil && a.subHub != nil {
		for _, connID := range a.subHub.GetMessageSubscribers(a.sessionKey) {
			if connID != a.conn.ID {
				a.bm.BroadcastTo(connID, "chat", event)
			}
		}
	}
}

// sendAgentEvent pushes an agent tool event to the originating connection AND
// to all other connections subscribed to message events for this session.
func (a *ChatStreamAdapter) sendAgentEvent(event gateway.AgentToolPayload) {
	a.conn.SendEvent("agent", event)
	if a.bm != nil && a.subHub != nil {
		for _, connID := range a.subHub.GetMessageSubscribers(a.sessionKey) {
			if connID != a.conn.ID {
				a.bm.BroadcastTo(connID, "agent", event)
			}
		}
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

	// Build message content: text + any pending tool blocks
	contentBlocks := []gateway.ContentBlock{
		{Type: "text", Text: a.builder.String()},
	}
	contentBlocks = append(contentBlocks, a.toolBlocks...)

	seq := int(a.seq.Add(1))
	replace := false
	a.sendChatEvent(gateway.ChatEvent{
		RunID:      a.runID,
		SessionKey: a.sessionKey,
		AgentID:    a.agentID,
		Seq:        seq,
		State:      gateway.ChatStateDelta,
		DeltaText:  delta,
		Replace:    &replace,
		Message: gateway.ChatMessage{
			Role:    "assistant",
			Content: contentBlocks,
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

	// Add tool_use block to the tool blocks list (for done/final message)
	toolBlock := gateway.ContentBlock{
		Type:  "tool_use",
		ID:    toolID,
		Name:  toolName,
		Input: json.RawMessage(args),
	}
	a.toolBlocks = append(a.toolBlocks, toolBlock)

	// Send agent event for dedicated tool stream handler
	seq := int(a.seq.Add(1))
	a.sendAgentEvent(gateway.AgentToolPayload{
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

	// Also send a chat delta so tool call text appears inline in the chat stream
	deltaText := fmt.Sprintf("\n> 调用工具: **%s**\n> 参数: %s\n\n", toolName, args)
	a.builder.WriteString(deltaText)
	seq2 := int(a.seq.Add(1))
	replace := false
	a.sendChatEvent(gateway.ChatEvent{
		RunID:      a.runID,
		SessionKey: a.sessionKey,
		AgentID:    a.agentID,
		Seq:        seq2,
		State:      gateway.ChatStateDelta,
		DeltaText:  deltaText,
		Replace:    &replace,
		Message: gateway.ChatMessage{
			Role: "assistant",
			Content: []gateway.ContentBlock{
				{Type: "text", Text: a.builder.String()},
				toolBlock,
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

	// Add tool_result block to the tool blocks list
	toolResultBlock := gateway.ContentBlock{
		Type: "tool_result",
		ID:   toolID,
		Name: toolName,
		Text: resultText,
	}
	a.toolBlocks = append(a.toolBlocks, toolResultBlock)

	seq := int(a.seq.Add(1))
	a.sendAgentEvent(gateway.AgentToolPayload{
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

	// Also send inline chat delta for tool result
	// Truncate long results for display
	displayResult := resultText
	if len(displayResult) > 500 {
		displayResult = displayResult[:500] + "\n... (结果已截断)"
	}
	statusIcon := "[✓]"
	if isError {
		statusIcon = "[✗]"
	}
	deltaText := fmt.Sprintf("> %s 结果 (%s):\n```\n%s\n```\n\n", statusIcon, toolName, displayResult)
	a.builder.WriteString(deltaText)
	seq2 := int(a.seq.Add(1))
	replace := false
	a.sendChatEvent(gateway.ChatEvent{
		RunID:      a.runID,
		SessionKey: a.sessionKey,
		AgentID:    a.agentID,
		Seq:        seq2,
		State:      gateway.ChatStateDelta,
		DeltaText:  deltaText,
		Replace:    &replace,
		Message: gateway.ChatMessage{
			Role: "assistant",
			Content: []gateway.ContentBlock{
				{Type: "text", Text: a.builder.String()},
				toolResultBlock,
			},
		},
	})
}

func (a *ChatStreamAdapter) handleDone(data string) {
	full := a.builder.String()

	// Build final message content: text + all tool blocks
	contentBlocks := []gateway.ContentBlock{
		{Type: "text", Text: full},
	}
	contentBlocks = append(contentBlocks, a.toolBlocks...)

	seq := int(a.seq.Add(1))
	a.sendChatEvent(gateway.ChatEvent{
		RunID:      a.runID,
		SessionKey: a.sessionKey,
		AgentID:    a.agentID,
		Seq:        seq,
		State:      gateway.ChatStateFinal,
		Message: gateway.ChatMessage{
			Role:    "assistant",
			Content: contentBlocks,
		},
	})

	// Reset per-turn state
	a.toolBlocks = nil
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
	a.sendChatEvent(gateway.ChatEvent{
		RunID:      a.runID,
		SessionKey: a.sessionKey,
		AgentID:    a.agentID,
		Seq:        seq,
		State:      gateway.ChatStateError,
		ErrorMsg:   errMsg,
		ErrorKind:  "agent_error",
	})
}
