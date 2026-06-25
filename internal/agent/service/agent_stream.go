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
		zap.L().Error("[AgentStream] agent.Stream() 创建失败", zap.Error(err))
		// 发送错误事件到前端，包含完整错误信息
		_ = writeSSE(writer, "error", map[string]interface{}{
			"error": fmt.Sprintf("Agent Stream 创建失败: %s", err.Error()),
		})
		return "", fmt.Errorf("创建 Agent 流失败: %w", err)
	}

	zap.L().Info("[AgentStream] 流式 Agent 已启动", zap.String("sessionID", sessionID))

	var finalContent string

	for {
		msg, err := sr.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			zap.L().Error("[AgentStream] 流读取失败", zap.Error(err))
			_ = writeSSE(writer, "error", map[string]interface{}{
				"error": fmt.Sprintf("Agent Stream 读取错误: %s", err.Error()),
			})
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

		// tool_call 事件已移至 safe_tool.preCallback 中发送，
		// 确保即使工具执行失败，前端也能看到工具调用信息。
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
