/*
 * MIT License
 *
 * Copyright (c) 2024 Bamboo
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in
 * all copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
 * THE SOFTWARE.
 *
 */

package model

import "time"

// AgentSessionStatus 会话状态
type AgentSessionStatus int8

const (
	AgentSessionStatusActive   AgentSessionStatus = iota + 1 // 活跃
	AgentSessionStatusArchived                               // 已归档
)

// AgentMessageRole 消息角色
type AgentMessageRole = string

const (
	AgentMessageRoleUser      AgentMessageRole = "user"      // 用户
	AgentMessageRoleAssistant AgentMessageRole = "assistant" // 助手
	AgentMessageRoleSystem    AgentMessageRole = "system"    // 系统
	AgentMessageRoleTool      AgentMessageRole = "tool"      // 工具
)

// MCPPluginStatus MCP插件状态
type MCPPluginStatus = string

const (
	MCPPluginStatusActive   MCPPluginStatus = "active"   // 正常
	MCPPluginStatusDisabled MCPPluginStatus = "disabled" // 已禁用
	MCPPluginStatusError    MCPPluginStatus = "error"    // 异常
)

// RemoteMCPTransport 远程MCP传输方式
type RemoteMCPTransport = string

const (
	RemoteMCPTransportSSE            RemoteMCPTransport = "sse"             // SSE
	RemoteMCPTransportStreamableHTTP RemoteMCPTransport = "streamable-http" // Streamable HTTP
)

// RemoteMCPAuthType 远程MCP认证方式
type RemoteMCPAuthType = string

const (
	RemoteMCPAuthTypeNone   RemoteMCPAuthType = "none"   // 无认证
	RemoteMCPAuthTypeBearer RemoteMCPAuthType = "bearer" // Bearer Token
	RemoteMCPAuthTypeBasic  RemoteMCPAuthType = "basic"  // Basic Auth
)

// AgentSession 智能体对话会话
type AgentSession struct {
	Model
	UserID       int                `json:"user_id" gorm:"index;not null;comment:用户ID"`
	Title        string             `json:"title" gorm:"type:varchar(200);not null;default:'';comment:会话标题"`
	ModelName    string             `json:"model" gorm:"column:model;type:varchar(100);not null;default:'';comment:使用的模型"`
	ToolCount    int                `json:"tool_count" gorm:"not null;default:0;comment:可用工具数"`
	MessageCount int                `json:"message_count" gorm:"not null;default:0;comment:消息数"`
	Status       AgentSessionStatus `json:"status" gorm:"not null;default:1;index;comment:状态 1活跃 2归档"`
}

func (AgentSession) TableName() string {
	return "cl_agent_sessions"
}

// AgentMessage 智能体对话消息
type AgentMessage struct {
	ID         int64            `json:"id" gorm:"primaryKey;autoIncrement;type:bigint;comment:主键ID"`
	SessionID  string           `json:"session_id" gorm:"type:varchar(36);not null;index;comment:会话UUID"`
	Role       AgentMessageRole `json:"role" gorm:"type:varchar(20);not null;comment:角色 user/assistant/system/tool"`
	Content    string           `json:"content" gorm:"type:text;not null;comment:消息内容"`
	ToolCalls  JSONMap          `json:"tool_calls" gorm:"type:json;comment:工具调用信息"`
	ToolCallID string           `json:"tool_call_id" gorm:"type:varchar(100);comment:工具调用ID"`
	Metadata     JSONMap          `json:"metadata" gorm:"type:json;comment:元数据"`
	SearchVector string           `json:"-" gorm:"->;-:migration;type:tsvector;comment:全文搜索向量(PostgreSQL GENERATED列)"`
	CreatedAt    time.Time        `json:"created_at" gorm:"autoCreateTime;comment:创建时间"`
}

func (AgentMessage) TableName() string {
	return "cl_agent_messages"
}

// BuiltinTool 内置工具
type BuiltinTool struct {
	Name        string    `json:"name" gorm:"primaryKey;type:varchar(100);comment:工具唯一标识"`
	DisplayName string    `json:"display_name" gorm:"type:varchar(200);not null;comment:显示名称"`
	Description string    `json:"description" gorm:"type:text;comment:工具描述"`
	Category    string    `json:"category" gorm:"type:varchar(50);index;comment:工具分类"`
	Enabled     bool      `json:"enabled" gorm:"not null;default:true;comment:是否启用"`
	Config      JSONMap   `json:"config" gorm:"type:json;comment:工具配置"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime;comment:创建时间"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime;comment:更新时间"`
}

func (BuiltinTool) TableName() string {
	return "cl_agent_builtin_tools"
}

// MCPPlugin MCP插件
type MCPPlugin struct {
	Model
	Name        string          `json:"name" gorm:"type:varchar(100);not null;uniqueIndex;comment:插件标识"`
	DisplayName string          `json:"display_name" gorm:"type:varchar(200);not null;comment:显示名称"`
	Description string          `json:"description" gorm:"type:text;comment:插件描述"`
	Version     string          `json:"version" gorm:"type:varchar(50);comment:版本号"`
	Author      string          `json:"author" gorm:"type:varchar(100);comment:作者"`
	Category    string          `json:"category" gorm:"type:varchar(50);index;comment:插件分类"`
	Tags        StringList      `json:"tags" gorm:"type:json;comment:标签"`
	IconURL     string          `json:"icon_url" gorm:"type:varchar(500);comment:图标URL"`
	Homepage    string          `json:"homepage" gorm:"type:varchar(500);comment:主页地址"`
	Manifest    JSONMap         `json:"manifest" gorm:"type:json;not null;comment:插件清单"`
	BinaryPath  string          `json:"binary_path" gorm:"type:varchar(500);comment:二进制路径"`
	BinaryHash  string          `json:"binary_hash" gorm:"type:varchar(64);comment:二进制哈希"`
	Downloads   int             `json:"downloads" gorm:"not null;default:0;comment:下载次数"`
	Status      MCPPluginStatus `json:"status" gorm:"type:varchar(20);not null;default:'active';index;comment:状态 active/disabled/error"`
}

func (MCPPlugin) TableName() string {
	return "cl_agent_mcp_plugins"
}

// MCPPluginInstall MCP插件安装记录
type MCPPluginInstall struct {
	Model
	PluginID int     `json:"plugin_id" gorm:"not null;index;comment:插件ID"`
	UserID   int     `json:"user_id" gorm:"not null;index;comment:用户ID"`
	Config   JSONMap `json:"config" gorm:"type:json;comment:安装配置"`
	Enabled  bool    `json:"enabled" gorm:"not null;default:true;comment:是否启用"`
}

func (MCPPluginInstall) TableName() string {
	return "cl_agent_mcp_plugin_installs"
}

// RemoteMCPConfig 远程MCP配置
type RemoteMCPConfig struct {
	Model
	UserID      int                `json:"user_id" gorm:"not null;index;comment:用户ID"`
	Name        string             `json:"name" gorm:"type:varchar(100);not null;comment:配置名称"`
	Description string             `json:"description" gorm:"type:text;comment:配置描述"`
	Transport   RemoteMCPTransport `json:"transport" gorm:"type:varchar(30);not null;comment:传输方式 sse/streamable-http"`
	URL         string             `json:"url" gorm:"type:varchar(500);not null;comment:服务地址"`
	AuthType    RemoteMCPAuthType  `json:"auth_type" gorm:"type:varchar(20);not null;default:'none';comment:认证方式 none/bearer/basic"`
	AuthConfig  JSONMap            `json:"auth_config" gorm:"type:json;comment:认证配置"`
	Enabled     bool               `json:"enabled" gorm:"not null;default:true;comment:是否启用"`
}

func (RemoteMCPConfig) TableName() string {
	return "cl_agent_remote_mcp_configs"
}

// AgentAuditEvent 智能体审计事件
type AgentAuditEvent struct {
	ID        int64     `json:"id" gorm:"primaryKey;autoIncrement;type:bigint;comment:主键ID"`
	SessionID string    `json:"session_id" gorm:"type:varchar(36);not null;index;comment:会话UUID"`
	UserID    int       `json:"user_id" gorm:"not null;index;comment:用户ID"`
	ToolName  string    `json:"tool_name" gorm:"type:varchar(100);comment:工具名称"`
	ToolArgs  string    `json:"tool_args" gorm:"type:text;comment:工具参数"`
	RiskLevel string    `json:"risk_level" gorm:"type:varchar(20);comment:风险等级"`
	Action    string    `json:"action" gorm:"type:varchar(20);comment:动作"`
	Result    string    `json:"result" gorm:"type:text;comment:执行结果"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime;comment:创建时间"`
}

func (AgentAuditEvent) TableName() string {
	return "cl_agent_audit_events"
}

// ==================== 请求/响应结构体 ====================

// CreateAgentSessionReq 创建会话请求
type CreateAgentSessionReq struct {
	Title string `json:"title" binding:"omitempty,max=200"`
	Model string `json:"model" binding:"omitempty,max=100"`
}

// ListAgentSessionsReq 会话列表请求
type ListAgentSessionsReq struct {
	ListReq
}

// GetAgentSessionReq 获取会话详情请求
type GetAgentSessionReq struct {
	ID int `json:"id" form:"id" uri:"id" binding:"required"`
}

// ListAgentMessagesReq 消息列表请求
type ListAgentMessagesReq struct {
	ListReq
	SessionID string `json:"session_id" form:"session_id" binding:"required"`
}

// AgentQueryReq 智能体查询请求
type AgentQueryReq struct {
	SessionID string `json:"session_id"` // 可选，为空时自动创建会话
	Question  string `json:"question" binding:"required"`
}

// AgentQueryResponse 智能体查询响应
type AgentQueryResponse struct {
	SessionID string `json:"session_id"`
	Answer    string `json:"answer"`
}

// AgentStreamEvent 智能体流式事件
type AgentStreamEvent struct {
	Event string      `json:"event"`
	Data  interface{} `json:"data"`
}

// ListBuiltinToolsReq 内置工具列表请求
type ListBuiltinToolsReq struct {
	ListReq
}

// ToggleBuiltinToolReq 切换内置工具状态请求
type ToggleBuiltinToolReq struct {
	Name string `json:"name" form:"name" binding:"required"`
}

// ListMCPPluginsReq MCP插件列表请求
type ListMCPPluginsReq struct {
	ListReq
	Category string `json:"category" form:"category" binding:"omitempty"`
}

// GetMCPPluginReq 获取MCP插件详情请求
type GetMCPPluginReq struct {
	ID int `json:"id" form:"id" uri:"id" binding:"required"`
}

// UploadMCPPluginReq 上传MCP插件请求
type UploadMCPPluginReq struct {
	Manifest string `json:"manifest" binding:"required"`
}

// InstallMCPPluginReq 安装MCP插件请求
type InstallMCPPluginReq struct {
	ID     int     `json:"id" form:"id" uri:"id" binding:"required"`
	Config JSONMap `json:"config" binding:"omitempty"`
}

// UninstallMCPPluginReq 卸载MCP插件请求
type UninstallMCPPluginReq struct {
	ID int `json:"id" form:"id" uri:"id" binding:"required"`
}

// ToggleMCPPluginReq 切换MCP插件状态请求
type ToggleMCPPluginReq struct {
	ID int `json:"id" form:"id" uri:"id" binding:"required"`
}

// ListRemoteMCPConfigsReq 远程MCP配置列表请求
type ListRemoteMCPConfigsReq struct {
	ListReq
}

// CreateRemoteMCPConfigReq 创建远程MCP配置请求
type CreateRemoteMCPConfigReq struct {
	Name        string             `json:"name" binding:"required,max=100"`
	Description string             `json:"description" binding:"omitempty"`
	Transport   RemoteMCPTransport `json:"transport" binding:"required,oneof=sse streamable-http"`
	URL         string             `json:"url" binding:"required,url"`
	AuthType    RemoteMCPAuthType  `json:"auth_type" binding:"omitempty,oneof=none bearer basic"`
	AuthConfig  JSONMap            `json:"auth_config" binding:"omitempty"`
}

// UpdateRemoteMCPConfigReq 更新远程MCP配置请求
type UpdateRemoteMCPConfigReq struct {
	ID          int                `json:"id" form:"id" uri:"id" binding:"required"`
	Name        string             `json:"name" binding:"omitempty,max=100"`
	Description string             `json:"description" binding:"omitempty"`
	Transport   RemoteMCPTransport `json:"transport" binding:"omitempty,oneof=sse streamable-http"`
	URL         string             `json:"url" binding:"omitempty,url"`
	AuthType    RemoteMCPAuthType  `json:"auth_type" binding:"omitempty,oneof=none bearer basic"`
	AuthConfig  JSONMap            `json:"auth_config" binding:"omitempty"`
}

// DeleteRemoteMCPConfigReq 删除远程MCP配置请求
type DeleteRemoteMCPConfigReq struct {
	ID int `json:"id" form:"id" uri:"id" binding:"required"`
}

// ToggleRemoteMCPConfigReq 切换远程MCP配置状态请求
type ToggleRemoteMCPConfigReq struct {
	ID int `json:"id" form:"id" uri:"id" binding:"required"`
}

// TestRemoteMCPConfigReq 测试远程MCP配置请求
type TestRemoteMCPConfigReq struct {
	ID int `json:"id" form:"id" uri:"id" binding:"required"`
}

// TestRemoteMCPResult 远程MCP测试结果
type TestRemoteMCPResult struct {
	Reachable bool     `json:"reachable"`
	Tools     []string `json:"tools"`
	Error     string   `json:"error"`
}

// AgentConfig agent 配置项（key-value，JSON value）
type AgentConfig struct {
	ID          int       `json:"id" gorm:"primaryKey;autoIncrement"`
	ConfigKey   string    `json:"config_key" gorm:"type:varchar(100);uniqueIndex;not null"`
	ConfigValue string    `json:"config_value" gorm:"type:text;not null"`
	Description string    `json:"description" gorm:"type:varchar(500)"`
	UpdatedAt   time.Time `json:"updated_at"`
	CreatedAt   time.Time `json:"created_at"`
}

func (AgentConfig) TableName() string {
	return "cl_agent_config"
}
