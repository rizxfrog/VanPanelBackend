// Package builtin provides built-in tools that execute locally on the server
// without requiring external MCP server connections.
package builtin

import (
	"github.com/cloudwego/eino/components/tool"
)

// NewTools returns all built-in tools for the AI Agent.
// Each tool implements the eino tool.InvokableTool interface.
func NewTools() []tool.BaseTool {
	return []tool.BaseTool{}
}
