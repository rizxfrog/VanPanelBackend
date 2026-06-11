package rpc

import (
	"context"
	"encoding/json"
	"time"

	"github.com/rizxfrog/VanPanelBackend/internal/gateway"
)

func init() {
	gateway.RegisterMethod("chat.send", string(gateway.ScopeWrite), handleChatSend)
	gateway.RegisterMethod("chat.abort", string(gateway.ScopeWrite), handleChatAbort)
	gateway.RegisterMethod("chat.history", string(gateway.ScopeRead), handleChatHistory)
	gateway.RegisterMethod("chat.startup", string(gateway.ScopeRead), handleChatStartup)
	gateway.RegisterMethod("chat.metadata", string(gateway.ScopeRead), handleChatMetadata)
	gateway.RegisterMethod("chat.message.get", string(gateway.ScopeRead), handleChatMessageGet)
}

func handleChatSend(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	var req struct {
		SessionKey string `json:"sessionKey"`
		Message    string `json:"message"`
		AgentID    string `json:"agentId,omitempty"`
	}
	json.Unmarshal(params, &req)

	runID := "run-" + time.Now().Format("20060102150405")

	// Start agent query in background, stream results as chat events
	go streamAgentReply(ctx, conn, req.SessionKey, req.AgentID, runID, req.Message)

	return map[string]interface{}{
		"runId":  runID,
		"status": "started",
	}, nil
}

// streamAgentReply runs the agent query and streams results as chat events.
// For now it sends a simulated response since the agent isn't wired yet.
func streamAgentReply(ctx context.Context, conn *gateway.GatewayConnection, sessionKey, agentID, runID, message string) {
	// Simulate AI response with streaming text
	reply := "I received your message: \"" + message + "\"  \nThe AI agent is being wired. For now, this is a simulated response to verify the streaming protocol works correctly. Once the Eino agent integration is complete, real tool calling and system queries will be available."

	seq := 0
	for i := 0; i < len(reply); i += 3 {
		select {
		case <-ctx.Done():
			return
		default:
		}

		seq++
		end := i + 3
		if end > len(reply) {
			end = len(reply)
		}
		delta := reply[i:end]

		fullContent := reply[:end]
		replace := false
		event := gateway.ChatEvent{
			RunID:      runID,
			SessionKey: sessionKey,
			AgentID:    agentID,
			Seq:        seq,
			State:      gateway.ChatStateDelta,
			DeltaText:  delta,
			Replace:    &replace,
			Message: gateway.ChatMessage{
				Role: "assistant",
				Content: []gateway.ContentBlock{
					{Type: "text", Text: fullContent},
				},
			},
		}
		conn.SendEvent("chat", event)
		time.Sleep(30 * time.Millisecond)
	}

	// Send final event
	seq++
	finalEvent := gateway.ChatEvent{
		RunID:      runID,
		SessionKey: sessionKey,
		AgentID:    agentID,
		Seq:        seq,
		State:      gateway.ChatStateFinal,
		Message: gateway.ChatMessage{
			Role: "assistant",
			Content: []gateway.ContentBlock{
				{Type: "text", Text: reply},
			},
		},
	}
	conn.SendEvent("chat", finalEvent)
}

func handleChatAbort(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]bool{"ok": true}, nil
}

func handleChatHistory(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	now := time.Now().UnixMilli()
	return map[string]interface{}{
		"messages": []map[string]interface{}{
			{
				"role":      "assistant",
				"content":   []map[string]string{{"type": "text", "text": "Welcome to VanPanel! I'm your AI operations assistant. How can I help you today?"}},
				"timestamp": now,
			},
		},
		"hasMore": false,
	}, nil
}

func handleChatStartup(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{
		"messages":   []interface{}{},
		"sessionKey": "agent:main:global",
	}, nil
}

func handleChatMetadata(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{
		"availableModels": []string{"gpt-4o"},
		"defaultModel":    "gpt-4o",
	}, nil
}

func handleChatMessageGet(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{
		"message": map[string]interface{}{
			"role":    "assistant",
			"content": []map[string]string{{"type": "text", "text": "Message content"}},
		},
	}, nil
}
