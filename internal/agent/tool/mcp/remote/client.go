// Package remote implements the MCP client for remotely-running MCP servers
// that communicate via SSE or streamable HTTP transport.
package remote

import (
	"context"
	"encoding/base64"
	"fmt"

	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"go.uber.org/zap"
)

// AuthConfig holds authentication configuration for a remote MCP connection.
type AuthConfig struct {
	Type     string // "none", "bearer", "basic"
	Token    string // Bearer token value
	Username string // Basic auth username
	Password string // Basic auth password
}

// RemoteMCPClient wraps an MCP client connection to a remotely-running MCP server
// using SSE or streamable HTTP transport.
type RemoteMCPClient struct {
	client    *mcpclient.Client
	name      string
	url       string
	transport string // "sse" or "streamable-http"
	auth      AuthConfig
	logger    *zap.Logger
}

// NewClient creates a new remote MCP client. Call Connect() to establish the connection.
func NewClient(name string, url string, transportType string, auth AuthConfig, logger *zap.Logger) *RemoteMCPClient {
	return &RemoteMCPClient{
		name:      name,
		url:       url,
		transport: transportType,
		auth:      auth,
		logger:    logger,
	}
}

// Name returns the MCP server name.
func (c *RemoteMCPClient) Name() string {
	return c.name
}

// Connect establishes the MCP connection and completes the initialize handshake.
func (c *RemoteMCPClient) Connect(ctx context.Context) error {
	headers := c.buildHeaders()

	var mcpClient *mcpclient.Client
	var err error

	switch c.transport {
	case "streamable-http":
		opts := make([]transport.StreamableHTTPCOption, 0, 1)
		if len(headers) > 0 {
			opts = append(opts, transport.WithHTTPHeaders(headers))
		}
		mcpClient, err = mcpclient.NewStreamableHttpClient(c.url, opts...)
	default: // "sse"
		opts := make([]transport.ClientOption, 0, 1)
		if len(headers) > 0 {
			opts = append(opts, transport.WithHeaders(headers))
		}
		mcpClient, err = mcpclient.NewSSEMCPClient(c.url, opts...)
	}
	if err != nil {
		return fmt.Errorf("创建远程MCP客户端失败 [%s]: %w", c.name, err)
	}
	c.client = mcpClient

	// 启动连接（SSE 需要显式 Start，StreamableHTTP 的 Start 是幂等的）
	if err := c.client.Start(ctx); err != nil {
		return fmt.Errorf("启动远程MCP客户端失败 [%s]: %w", c.name, err)
	}

	// MCP 初始化握手
	_, err = c.client.Initialize(ctx, mcpgo.InitializeRequest{
		Params: mcpgo.InitializeParams{
			ProtocolVersion: mcpgo.LATEST_PROTOCOL_VERSION,
			ClientInfo: mcpgo.Implementation{
				Name:    "ai-cloudops-agent",
				Version: "1.0.0",
			},
		},
	})
	if err != nil {
		return fmt.Errorf("初始化远程MCP客户端失败 [%s]: %w", c.name, err)
	}

	c.logger.Info("远程MCP客户端已连接", zap.String("name", c.name), zap.String("url", c.url))
	return nil
}

// ListTools discovers tools offered by the remote MCP server.
func (c *RemoteMCPClient) ListTools(ctx context.Context) ([]mcpgo.Tool, error) {
	if c.client == nil {
		return nil, fmt.Errorf("远程MCP客户端未连接 [%s]", c.name)
	}
	result, err := c.client.ListTools(ctx, mcpgo.ListToolsRequest{})
	if err != nil {
		return nil, fmt.Errorf("列出远程MCP工具失败 [%s]: %w", c.name, err)
	}
	return result.Tools, nil
}

// CallTool invokes a tool on the remote MCP server with the given arguments.
func (c *RemoteMCPClient) CallTool(ctx context.Context, toolName string, args map[string]any) (*mcpgo.CallToolResult, error) {
	if c.client == nil {
		return nil, fmt.Errorf("远程MCP客户端未连接 [%s]", c.name)
	}
	result, err := c.client.CallTool(ctx, mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      toolName,
			Arguments: args,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("调用远程MCP工具失败 [%s/%s]: %w", c.name, toolName, err)
	}
	return result, nil
}

// Close shuts down the MCP client connection.
func (c *RemoteMCPClient) Close() error {
	if c.client == nil {
		return nil
	}
	return c.client.Close()
}

// buildHeaders constructs HTTP headers based on the configured authentication type.
func (c *RemoteMCPClient) buildHeaders() map[string]string {
	headers := make(map[string]string)
	switch c.auth.Type {
	case "bearer":
		if c.auth.Token != "" {
			headers["Authorization"] = "Bearer " + c.auth.Token
		}
	case "basic":
		if c.auth.Username != "" {
			cred := base64.StdEncoding.EncodeToString(
				[]byte(c.auth.Username + ":" + c.auth.Password),
			)
			headers["Authorization"] = "Basic " + cred
		}
	}
	return headers
}
