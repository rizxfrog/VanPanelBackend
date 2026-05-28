// Package local implements the MCP client for locally-running MCP servers
// that communicate via stdio transport.
package local

import (
	"context"
	"fmt"

	mcpclient "github.com/mark3labs/mcp-go/client"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"go.uber.org/zap"
)

const maxRestartAttempts = 3

// LocalMCPClient wraps an MCP stdio client connection to a locally-running MCP server.
// It manages the subprocess lifecycle and provides tool discovery and invocation.
type LocalMCPClient struct {
	client  *mcpclient.Client
	name    string
	command string
	env     []string
	args    []string
	logger  *zap.Logger
}

// NewClient creates a new local MCP client for the given server command.
// The subprocess is spawned immediately via the mcp-go library.
// Call Start() to send the MCP initialize handshake.
func NewClient(ctx context.Context, name string, command string, args []string, env []string, logger *zap.Logger) (*LocalMCPClient, error) {
	c, err := mcpclient.NewStdioMCPClient(command, env, args...)
	if err != nil {
		return nil, fmt.Errorf("创建本地MCP客户端失败: %w", err)
	}
	return &LocalMCPClient{
		client:  c,
		name:    name,
		command: command,
		env:     env,
		args:    args,
		logger:  logger,
	}, nil
}

// Name returns the MCP server name.
func (c *LocalMCPClient) Name() string {
	return c.name
}

// Start sends the MCP initialize handshake to the subprocess.
func (c *LocalMCPClient) Start(ctx context.Context) error {
	_, err := c.client.Initialize(ctx, mcpgo.InitializeRequest{
		Params: mcpgo.InitializeParams{
			ProtocolVersion: mcpgo.LATEST_PROTOCOL_VERSION,
			ClientInfo: mcpgo.Implementation{
				Name:    "ai-cloudops-agent",
				Version: "1.0.0",
			},
		},
	})
	if err != nil {
		return fmt.Errorf("初始化本地MCP客户端失败 [%s]: %w", c.name, err)
	}
	c.logger.Info("本地MCP客户端已启动", zap.String("name", c.name))
	return nil
}

// IsRunning checks whether the MCP subprocess is alive by sending a ping.
func (c *LocalMCPClient) IsRunning(ctx context.Context) bool {
	if c.client == nil {
		return false
	}
	return c.client.Ping(ctx) == nil
}

// ListTools discovers tools offered by the MCP server.
func (c *LocalMCPClient) ListTools(ctx context.Context) ([]mcpgo.Tool, error) {
	result, err := c.client.ListTools(ctx, mcpgo.ListToolsRequest{})
	if err != nil {
		return nil, fmt.Errorf("列出本地MCP工具失败 [%s]: %w", c.name, err)
	}
	return result.Tools, nil
}

// CallTool invokes a tool on the MCP server with the given arguments.
func (c *LocalMCPClient) CallTool(ctx context.Context, toolName string, args map[string]any) (*mcpgo.CallToolResult, error) {
	result, err := c.client.CallTool(ctx, mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      toolName,
			Arguments: args,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("调用本地MCP工具失败 [%s/%s]: %w", c.name, toolName, err)
	}
	return result, nil
}

// Close shuts down the MCP client connection and subprocess.
func (c *LocalMCPClient) Close() error {
	if c.client == nil {
		return nil
	}
	return c.client.Close()
}

// Restart attempts to restart the subprocess up to maxRestartAttempts times.
// On each attempt it closes the old client, spawns a new subprocess, and re-initializes.
func (c *LocalMCPClient) Restart(ctx context.Context) error {
	var lastErr error
	for attempt := 1; attempt <= maxRestartAttempts; attempt++ {
		c.logger.Warn("尝试重启本地MCP客户端",
			zap.String("name", c.name),
			zap.Int("attempt", attempt),
		)

		// 关闭旧连接
		if c.client != nil {
			_ = c.client.Close()
		}

		// 创建新连接
		newClient, err := mcpclient.NewStdioMCPClient(c.command, c.env, c.args...)
		if err != nil {
			lastErr = fmt.Errorf("重新创建本地MCP客户端失败: %w", err)
			c.logger.Error("重启失败", zap.String("name", c.name), zap.Int("attempt", attempt), zap.Error(err))
			continue
		}
		c.client = newClient

		// 重新初始化
		if err := c.Start(ctx); err != nil {
			lastErr = err
			c.logger.Error("重启初始化失败", zap.String("name", c.name), zap.Int("attempt", attempt), zap.Error(err))
			continue
		}

		c.logger.Info("本地MCP客户端重启成功", zap.String("name", c.name))
		return nil
	}
	return fmt.Errorf("重启本地MCP客户端失败，已尝试%d次: %w", maxRestartAttempts, lastErr)
}
