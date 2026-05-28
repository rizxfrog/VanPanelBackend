// Package remote implements the MCP client for remotely-running MCP servers
// that communicate via SSE or streamable HTTP transport.
package remote

import (
	"context"

	mcpclient "github.com/mark3labs/mcp-go/client"
)

// Client wraps an MCP SSE/HTTP client connection to a remotely-running MCP server.
type Client struct {
	client *mcpclient.Client
	name   string
}

// NewSSEClient creates a new remote MCP client using SSE transport.
func NewSSEClient(ctx context.Context, name string, baseURL string, headers map[string]string) (*Client, error) {
	c, err := mcpclient.NewSSEMCPClient(baseURL, mcpclient.WithHeaders(headers))
	if err != nil {
		return nil, err
	}
	return &Client{client: c, name: name}, nil
}

// Name returns the MCP server name.
func (c *Client) Name() string {
	return c.name
}

// Close shuts down the MCP client connection.
func (c *Client) Close() error {
	return c.client.Close()
}
