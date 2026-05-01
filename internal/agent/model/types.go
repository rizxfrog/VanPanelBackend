package model

import "time"

type RiskLevel string

const (
	RiskSafe RiskLevel = "safe"
	RiskLow  RiskLevel = "low"
	RiskHigh RiskLevel = "high"
)

type ToolCall struct {
	ID     string         `json:"id"`
	Name   string         `json:"name"`
	Args   map[string]any `json:"args,omitempty"`
	Status string         `json:"status"`
	Result map[string]any `json:"result,omitempty"`
	Error  string         `json:"error,omitempty"`
}

type RiskDecision struct {
	Level            RiskLevel `json:"level"`
	Allowed          bool      `json:"allowed"`
	RequiresApproval bool      `json:"requiresApproval"`
	Reason           string    `json:"reason"`
}

type AuditEvent struct {
	ID        string         `json:"id"`
	SessionID string         `json:"sessionId"`
	UserID    uint           `json:"userId"`
	Username  string         `json:"username,omitempty"`
	Action    string         `json:"action"`
	ToolName  string         `json:"toolName,omitempty"`
	Risk      RiskLevel      `json:"risk"`
	Allowed   bool           `json:"allowed"`
	Reason    string         `json:"reason,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	CreatedAt time.Time      `json:"createdAt"`
}

type AgentMessage struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"createdAt"`
}

type AgentSession struct {
	ID        string         `json:"id"`
	UserID    uint           `json:"userId"`
	Username  string         `json:"username,omitempty"`
	Title     string         `json:"title"`
	Messages  []AgentMessage `json:"messages"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
}

type PlanResponse struct {
	Intent    string     `json:"intent"`
	Summary   string     `json:"summary"`
	ToolCalls []ToolCall `json:"toolCalls"`
	Risk      RiskLevel  `json:"risk"`
}

type ToolResult struct {
	ToolName string         `json:"toolName"`
	Output   map[string]any `json:"output,omitempty"`
	Error    string         `json:"error,omitempty"`
}

type Approval struct {
	ID        string       `json:"id"`
	SessionID string       `json:"sessionId"`
	UserID    uint         `json:"userId"`
	ToolCall  ToolCall     `json:"toolCall"`
	Decision  RiskDecision `json:"decision"`
	Status    string       `json:"status"`
	CreatedAt time.Time    `json:"createdAt"`
	UpdatedAt time.Time    `json:"updatedAt"`
}
