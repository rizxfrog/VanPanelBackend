package service

import (
	"context"
	"fmt"
	"io"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

// runAgentStream 使用 eino 标准 react.Agent.Stream() 执行流式 Agent 查询。
// 替代了旧的 textReActLoop 文本解析方式。
func runAgentStream(
	ctx context.Context,
	chatModel model.ToolCallingChatModel,
	safeTools []tool.BaseTool,
	messages []*schema.Message,
	writer io.Writer,
	writeSSE func(io.Writer, string, interface{}) error,
	sessionID string,
	personaPrompt string,
) (string, error) {
	agent, err := react.NewAgent(ctx, &react.AgentConfig{
		ToolCallingModel: chatModel,
		ToolsConfig:      compose.ToolsNodeConfig{Tools: safeTools},
		MaxStep:          10,
		MessageModifier:  react.NewPersonaModifier(personaPrompt),
	})
	if err != nil {
		return "", fmt.Errorf("创建 Agent 失败: %w", err)
	}

	sr, err := agent.Stream(ctx, messages)
	if err != nil {
		return "", fmt.Errorf("Agent Stream 失败: %w", err)
	}

	var finalContent string

	for {
		msg, err := sr.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			return finalContent, fmt.Errorf("流读取失败: %w", err)
		}

		if msg.Content != "" {
			finalContent += msg.Content
			if err := writeSSE(writer, "delta", map[string]string{
				"role":    "assistant",
				"content": msg.Content,
			}); err != nil {
				zap.L().Warn("写入 delta SSE 事件失败", zap.Error(err))
			}
		}

		for _, tc := range msg.ToolCalls {
			if err := writeSSE(writer, "tool_call", map[string]string{
				"id":        tc.ID,
				"name":      tc.Function.Name,
				"arguments": tc.Function.Arguments,
			}); err != nil {
				zap.L().Warn("写入 tool_call SSE 事件失败", zap.Error(err))
			}
		}
	}

	if err := writeSSE(writer, "done", map[string]interface{}{
		"session_id": sessionID,
		"result": map[string]string{
			"answer": finalContent,
		},
	}); err != nil {
		return finalContent, fmt.Errorf("写入 done SSE 事件失败: %w", err)
	}

	return finalContent, nil
}
