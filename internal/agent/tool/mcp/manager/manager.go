// Package manager manages the lifecycle of MCP client connections and
// aggregates tools from multiple MCP servers into a unified registry.
package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	jsonschema "github.com/eino-contrib/jsonschema"
	"github.com/mark3labs/mcp-go/mcp"
	"go.uber.org/zap"

	"github.com/GoSimplicity/AI-CloudOps/internal/agent/dao"
	"github.com/GoSimplicity/AI-CloudOps/internal/model"

	mcplocal "github.com/GoSimplicity/AI-CloudOps/internal/agent/tool/mcp/local"
	mcpremote "github.com/GoSimplicity/AI-CloudOps/internal/agent/tool/mcp/remote"
)

// ToolManager aggregates all tool sources (builtin, local MCP, remote MCP)
// and provides a unified interface for tool discovery and invocation.
type ToolManager struct {
	dao           dao.AgentDAO
	builtinTools  map[string]tool.BaseTool
	localClients  map[int]*mcplocal.LocalMCPClient   // pluginID -> client
	remoteClients map[int]*mcpremote.RemoteMCPClient  // configID -> client
	mu            sync.RWMutex
	logger        *zap.Logger
}

// NewToolManager creates a new ToolManager.
// Call SetBuiltinTools() to register builtin tool implementations before use.
func NewToolManager(dao dao.AgentDAO, logger *zap.Logger) *ToolManager {
	return &ToolManager{
		dao:           dao,
		builtinTools:  make(map[string]tool.BaseTool),
		localClients:  make(map[int]*mcplocal.LocalMCPClient),
		remoteClients: make(map[int]*mcpremote.RemoteMCPClient),
		logger:        logger,
	}
}

// SetBuiltinTools registers the builtin tool implementations by name.
func (m *ToolManager) SetBuiltinTools(tools []tool.BaseTool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, t := range tools {
		info, err := t.Info(context.Background())
		if err != nil {
			m.logger.Warn("获取内置工具信息失败", zap.Error(err))
			continue
		}
		m.builtinTools[info.Name] = t
	}
}

// GetAllTools returns all available tools from all sources:
// 1. Enabled builtin tools from the database
// 2. Tools from enabled local MCP plugin installations
// 3. Tools from enabled remote MCP configurations
func (m *ToolManager) GetAllTools(ctx context.Context) []tool.BaseTool {
	m.mu.Lock()
	defer m.mu.Unlock()

	var tools []tool.BaseTool

	// 1. 内置工具
	tools = append(tools, m.getBuiltinToolsLocked(ctx)...)

	// 2. 本地 MCP 插件工具
	tools = append(tools, m.getLocalMCPToolsLocked(ctx)...)

	// 3. 远程 MCP 工具
	tools = append(tools, m.getRemoteMCPToolsLocked(ctx)...)

	return tools
}

// Reload refreshes all tool sources. Closes existing clients and re-discovers tools on next call.
func (m *ToolManager) Reload(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, c := range m.localClients {
		if err := c.Close(); err != nil {
			m.logger.Warn("关闭本地MCP客户端失败", zap.Int("pluginID", id), zap.Error(err))
		}
		delete(m.localClients, id)
	}
	for id, c := range m.remoteClients {
		if err := c.Close(); err != nil {
			m.logger.Warn("关闭远程MCP客户端失败", zap.Int("configID", id), zap.Error(err))
		}
		delete(m.remoteClients, id)
	}

	m.logger.Info("工具管理器已重载")
}

// Close closes all MCP client connections. Should be called on application shutdown.
func (m *ToolManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, c := range m.localClients {
		if err := c.Close(); err != nil {
			m.logger.Warn("关闭本地MCP客户端失败", zap.Int("pluginID", id), zap.Error(err))
		}
	}
	for id, c := range m.remoteClients {
		if err := c.Close(); err != nil {
			m.logger.Warn("关闭远程MCP客户端失败", zap.Int("configID", id), zap.Error(err))
		}
	}
}

// ==================== 内置工具 ====================

// getBuiltinToolsLocked returns enabled builtin tools. Caller must hold m.mu.
func (m *ToolManager) getBuiltinToolsLocked(ctx context.Context) []tool.BaseTool {
	dbTools, err := m.dao.ListBuiltinTools(ctx)
	if err != nil {
		m.logger.Error("查询内置工具列表失败", zap.Error(err))
		return nil
	}

	var tools []tool.BaseTool
	for _, dbt := range dbTools {
		if !dbt.Enabled {
			continue
		}
		if bt, ok := m.builtinTools[dbt.Name]; ok {
			tools = append(tools, bt)
		}
	}
	return tools
}

// ==================== 本地 MCP 插件 ====================

// getLocalMCPToolsLocked returns tools from enabled local MCP plugin installations.
// Caller must hold m.mu.
func (m *ToolManager) getLocalMCPToolsLocked(ctx context.Context) []tool.BaseTool {
	installs, err := m.dao.ListEnabledPluginInstalls(ctx)
	if err != nil {
		m.logger.Error("查询已启用插件安装失败", zap.Error(err))
		return nil
	}

	var tools []tool.BaseTool
	for _, install := range installs {
		client, err := m.getOrCreateLocalClientLocked(ctx, install)
		if err != nil {
			m.logger.Warn("获取本地MCP客户端失败",
				zap.Int("pluginID", install.PluginID), zap.Error(err))
			continue
		}
		mcpTools, err := client.ListTools(ctx)
		if err != nil {
			m.logger.Warn("列出本地MCP工具失败",
				zap.Int("pluginID", install.PluginID), zap.Error(err))
			continue
		}
		for _, mt := range mcpTools {
			tools = append(tools, NewMCPToolAdapter(client.Name(), mt, nil, localToolCaller{client}))
		}
	}
	return tools
}

// getOrCreateLocalClientLocked returns an existing or newly created local MCP client.
// Caller must hold m.mu.
func (m *ToolManager) getOrCreateLocalClientLocked(ctx context.Context, install *model.MCPPluginInstall) (*mcplocal.LocalMCPClient, error) {
	if c, ok := m.localClients[install.PluginID]; ok {
		if c.IsRunning(ctx) {
			return c, nil
		}
		// 客户端已断开，尝试重启
		if err := c.Restart(ctx); err != nil {
			m.logger.Error("重启本地MCP客户端失败", zap.Int("pluginID", install.PluginID), zap.Error(err))
			return nil, err
		}
		return c, nil
	}

	// 获取插件信息以取得二进制路径
	plugin, err := m.dao.GetPlugin(ctx, install.PluginID)
	if err != nil {
		return nil, fmt.Errorf("获取插件信息失败: %w", err)
	}
	if plugin.BinaryPath == "" {
		return nil, fmt.Errorf("插件 [%s] 未配置二进制路径", plugin.Name)
	}

	c, err := mcplocal.NewClient(ctx, plugin.Name, plugin.BinaryPath, nil, nil, m.logger)
	if err != nil {
		return nil, err
	}
	if err := c.Start(ctx); err != nil {
		_ = c.Close()
		return nil, err
	}

	m.localClients[install.PluginID] = c
	return c, nil
}

// ==================== 远程 MCP 配置 ====================

// getRemoteMCPToolsLocked returns tools from enabled remote MCP configurations.
// Caller must hold m.mu.
func (m *ToolManager) getRemoteMCPToolsLocked(ctx context.Context) []tool.BaseTool {
	configs, err := m.dao.ListEnabledRemoteMCPConfigs(ctx)
	if err != nil {
		m.logger.Error("查询已启用远程MCP配置失败", zap.Error(err))
		return nil
	}

	var tools []tool.BaseTool
	for _, cfg := range configs {
		client, err := m.getOrCreateRemoteClientLocked(ctx, cfg)
		if err != nil {
			m.logger.Warn("获取远程MCP客户端失败",
				zap.Int("configID", cfg.ID), zap.String("name", cfg.Name), zap.Error(err))
			continue
		}
		mcpTools, err := client.ListTools(ctx)
		if err != nil {
			m.logger.Warn("列出远程MCP工具失败",
				zap.Int("configID", cfg.ID), zap.String("name", cfg.Name), zap.Error(err))
			continue
		}
		for _, mt := range mcpTools {
			tools = append(tools, NewMCPToolAdapter(client.Name(), mt, nil, remoteToolCaller{client}))
		}
	}
	return tools
}

// getOrCreateRemoteClientLocked returns an existing or newly created remote MCP client.
// Caller must hold m.mu.
func (m *ToolManager) getOrCreateRemoteClientLocked(ctx context.Context, cfg *model.RemoteMCPConfig) (*mcpremote.RemoteMCPClient, error) {
	if c, ok := m.remoteClients[cfg.ID]; ok {
		return c, nil
	}

	auth := mcpremote.AuthConfig{Type: cfg.AuthType}
	if cfg.AuthConfig != nil {
		if v, ok := cfg.AuthConfig["token"].(string); ok {
			auth.Token = v
		}
		if v, ok := cfg.AuthConfig["username"].(string); ok {
			auth.Username = v
		}
		if v, ok := cfg.AuthConfig["password"].(string); ok {
			auth.Password = v
		}
	}

	c := mcpremote.NewClient(cfg.Name, cfg.URL, cfg.Transport, auth, m.logger)
	if err := c.Connect(ctx); err != nil {
		return nil, err
	}

	m.remoteClients[cfg.ID] = c
	return c, nil
}

// ==================== MCP 工具调用器 ====================

// mcpToolCaller abstracts MCP tool invocation for both local and remote clients.
type mcpToolCaller interface {
	CallTool(ctx context.Context, toolName string, args map[string]any) (*mcp.CallToolResult, error)
}

// localToolCaller adapts LocalMCPClient to mcpToolCaller.
type localToolCaller struct{ c *mcplocal.LocalMCPClient }

func (a localToolCaller) CallTool(ctx context.Context, name string, args map[string]any) (*mcp.CallToolResult, error) {
	return a.c.CallTool(ctx, name, args)
}

// remoteToolCaller adapts RemoteMCPClient to mcpToolCaller.
type remoteToolCaller struct{ c *mcpremote.RemoteMCPClient }

func (a remoteToolCaller) CallTool(ctx context.Context, name string, args map[string]any) (*mcp.CallToolResult, error) {
	return a.c.CallTool(ctx, name, args)
}

// ==================== MCP 工具 → Eino 适配器 ====================

// MCPToolAdapter wraps an MCP tool as an eino InvokableTool.
type MCPToolAdapter struct {
	serverName  string
	mcpTool     mcp.Tool
	caller      mcpToolCaller
	paramsOneOf *schema.ParamsOneOf
}

// NewMCPToolAdapter creates a new MCP tool adapter.
// schemaOverride is optional; if nil, the MCP tool's InputSchema is converted automatically.
func NewMCPToolAdapter(serverName string, mcpTool mcp.Tool, schemaOverride *schema.ParamsOneOf, caller mcpToolCaller) *MCPToolAdapter {
	a := &MCPToolAdapter{
		serverName: serverName,
		mcpTool:    mcpTool,
		caller:     caller,
	}
	if schemaOverride != nil {
		a.paramsOneOf = schemaOverride
	} else {
		a.paramsOneOf = convertMCPSchema(mcpTool.InputSchema)
	}
	return a
}

// Info returns the tool metadata for the eino framework.
func (a *MCPToolAdapter) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name:        a.serverName + "." + a.mcpTool.Name,
		Desc:        a.mcpTool.Description,
		ParamsOneOf: a.paramsOneOf,
	}, nil
}

// InvokableRun executes the tool by calling the MCP server.
func (a *MCPToolAdapter) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	var args map[string]any
	if argumentsInJSON != "" {
		if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
			return "", fmt.Errorf("解析工具参数失败: %w", err)
		}
	}

	result, err := a.caller.CallTool(ctx, a.mcpTool.Name, args)
	if err != nil {
		return "", err
	}

	return extractTextContent(result), nil
}

// ==================== 辅助函数 ====================

// extractTextContent extracts text from an MCP tool call result.
func extractTextContent(result *mcp.CallToolResult) string {
	if result == nil || len(result.Content) == 0 {
		return ""
	}
	var text string
	for _, content := range result.Content {
		if tc, ok := mcp.AsTextContent(content); ok {
			if text != "" {
				text += "\n"
			}
			text += tc.Text
		}
	}
	if text == "" {
		data, _ := json.Marshal(result.Content)
		text = string(data)
	}
	return text
}

// convertMCPSchema converts an MCP ToolInputSchema to an eino ParamsOneOf.
// Tries JSON round-trip first; falls back to basic parameter mapping.
func convertMCPSchema(inputSchema mcp.ToolInputSchema) *schema.ParamsOneOf {
	if converted, ok := tryJSONRoundTrip(inputSchema); ok {
		return converted
	}
	return basicParamMapping(inputSchema)
}

// tryJSONRoundTrip marshals the MCP schema to JSON and unmarshals into eino's jsonschema.Schema.
func tryJSONRoundTrip(inputSchema mcp.ToolInputSchema) (*schema.ParamsOneOf, bool) {
	schemaBytes, err := json.Marshal(inputSchema)
	if err != nil {
		return nil, false
	}

	var s jsonschema.Schema
	if err := json.Unmarshal(schemaBytes, &s); err != nil {
		return nil, false
	}

	// eino 的 jsonschema.Schema 的 Type 字段标记为 json:"-"，
	// 但其自定义 UnmarshalJSON 会正确处理 type 关键字。
	// 如果未设置则手动补充。
	if s.Type == "" {
		s.Type = inputSchema.Type
	}

	return schema.NewParamsOneOfByJSONSchema(&s), true
}

// basicParamMapping creates a ParamsOneOf by mapping MCP properties to ParameterInfo.
// Used as a fallback when JSON round-trip conversion fails.
func basicParamMapping(inputSchema mcp.ToolInputSchema) *schema.ParamsOneOf {
	if len(inputSchema.Properties) == 0 {
		return nil
	}

	requiredSet := make(map[string]bool, len(inputSchema.Required))
	for _, r := range inputSchema.Required {
		requiredSet[r] = true
	}

	params := make(map[string]*schema.ParameterInfo, len(inputSchema.Properties))
	for name, raw := range inputSchema.Properties {
		propMap, ok := raw.(map[string]any)
		if !ok {
			params[name] = &schema.ParameterInfo{Type: schema.String}
			continue
		}
		pi := &schema.ParameterInfo{
			Type:     mapJSONSchemaType(propMap),
			Required: requiredSet[name],
		}
		if desc, ok := propMap["description"].(string); ok {
			pi.Desc = desc
		}
		if enumVals, ok := propMap["enum"].([]any); ok {
			for _, ev := range enumVals {
				if s, ok := ev.(string); ok {
					pi.Enum = append(pi.Enum, s)
				}
			}
		}
		params[name] = pi
	}

	return schema.NewParamsOneOfByParams(params)
}

// mapJSONSchemaType maps a JSON Schema type string to eino's DataType.
func mapJSONSchemaType(propMap map[string]any) schema.DataType {
	t, _ := propMap["type"].(string)
	switch t {
	case "string":
		return schema.String
	case "integer":
		return schema.Integer
	case "number":
		return schema.Number
	case "boolean":
		return schema.Boolean
	case "array":
		return schema.Array
	default:
		return schema.String
	}
}
