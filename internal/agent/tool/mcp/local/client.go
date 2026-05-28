// Package local implements the MCP client for locally-running MCP servers
// that communicate via stdio transport.
package local

import (
	"context"

	mcpclient "github.com/mark3labs/mcp-go/client"
)

// Client wraps an MCP stdio client connection to a locally-running MCP server.
type Client struct {
	client *mcpclient.Client
	name   string
}

// NewClient creates a new local MCP client for the given server command.
func NewClient(ctx context.Context, name string, command string, args []string, env []string) (*Client, error) {
	c, err := mcpclient.NewStdioMCPClient(command, env, args...)
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
