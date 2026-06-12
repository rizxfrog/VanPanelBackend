package server

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/server"
	"go.uber.org/zap"

	agentRisk "github.com/rizxfrog/VanPanelBackend/internal/agent/risk"
	agentToolManager "github.com/rizxfrog/VanPanelBackend/internal/agent/tool/mcp/manager"
)

// ServeOptions configures how the MCP server runs.
type ServeOptions struct {
	Transport string // "stdio" or "http"
	Port      int    // for "http" transport, default 8890
}

// Serve starts the VanPanel MCP server exposing all agent tools.
func Serve(ctx context.Context, opts ServeOptions, toolMgr *agentToolManager.ToolManager, riskEval *agentRisk.Evaluator, logger *zap.Logger) error {
	mcpServer := server.NewMCPServer(
		"VanPanel",
		"1.0.0",
		server.WithToolCapabilities(false),
	)

	bridge := NewAgentMcpBridge(toolMgr, riskEval, logger)
	bridge.RegisterTools(mcpServer)

	switch opts.Transport {
	case "http":
		addr := fmt.Sprintf(":%d", opts.Port)
		if opts.Port == 0 {
			addr = ":8890"
		}
		logger.Info("MCP HTTP 服务端启动", zap.String("addr", addr))
		sseServer := server.NewSSEServer(mcpServer)
		return sseServer.Start(addr)
	default:
		logger.Info("MCP stdio 服务端启动")
		return server.ServeStdio(mcpServer)
	}
}
