package server

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"go.uber.org/zap"

	agentRisk "github.com/rizxfrog/VanPanelBackend/internal/agent/risk"
	agentToolManager "github.com/rizxfrog/VanPanelBackend/internal/agent/tool/mcp/manager"
)

// AgentMcpBridge bridges Eino Agent tools to MCP server tools.
type AgentMcpBridge struct {
	toolMgr  *agentToolManager.ToolManager
	riskEval *agentRisk.Evaluator
	logger   *zap.Logger
}

// NewAgentMcpBridge creates a new bridge.
func NewAgentMcpBridge(toolMgr *agentToolManager.ToolManager, riskEval *agentRisk.Evaluator, logger *zap.Logger) *AgentMcpBridge {
	return &AgentMcpBridge{toolMgr: toolMgr, riskEval: riskEval, logger: logger}
}

// RegisterTools registers all agent tools as MCP tools on the server.
func (b *AgentMcpBridge) RegisterTools(mcpServer *server.MCPServer) {
	tools := b.toolMgr.GetAllTools(context.Background())
	for _, t := range tools {
		info, err := t.Info(context.Background())
		if err != nil {
			b.logger.Warn("获取工具信息失败", zap.Error(err))
			continue
		}

		mcpTool := mcp.NewTool(
			info.Name,
			mcp.WithDescription(info.Desc),
		)
		handler := b.createHandler(t)
		mcpServer.AddTool(mcpTool, handler)
	}
	b.logger.Info("MCP 服务端工具注册完成", zap.Int("count", len(tools)))
}

// createHandler creates a tool handler with risk evaluation.
func (b *AgentMcpBridge) createHandler(einoTool tool.BaseTool) server.ToolHandlerFunc {
	info, _ := einoTool.Info(context.Background())
	toolName := info.Name

	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// 1. Risk audit
		if b.riskEval != nil {
			argsJSON, _ := json.Marshal(req.Params.Arguments)
			result := b.riskEval.Evaluate(toolName, string(argsJSON))
			if result.Blocked {
				return mcp.NewToolResultError(
					fmt.Sprintf("风险审计拒绝: %s", result.Reason),
				), fmt.Errorf("风险审计拒绝: %s", result.Reason)
			}
		}

		// 2. Execute tool
		argsJSON, err := json.Marshal(req.Params.Arguments)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("参数序列化失败: %v", err)), err
		}

		invokableTool, ok := einoTool.(tool.InvokableTool)
		if !ok {
			return mcp.NewToolResultError(
				fmt.Sprintf("工具 %s 不支持 InvokableTool 接口", toolName),
			), fmt.Errorf("工具 %s 不支持 InvokableTool 接口", toolName)
		}

		output, err := invokableTool.InvokableRun(ctx, string(argsJSON))
		if err != nil {
			b.logger.Error("MCP 工具调用失败", zap.String("tool", toolName), zap.Error(err))
			return mcp.NewToolResultError(fmt.Sprintf("工具调用失败: %v", err)), err
		}

		return mcp.NewToolResultText(output), nil
	}
}
