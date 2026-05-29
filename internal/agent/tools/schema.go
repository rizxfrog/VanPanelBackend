package tools

// ToolSchema 描述工具参数的 JSON Schema
type ToolSchema struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties"`
	Required   []string            `json:"required,omitempty"`
}

// Property 描述单个参数的类型和约束
type Property struct {
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Enum        []string `json:"enum,omitempty"`
	Default     any      `json:"default,omitempty"`
}

// SchemaProvider 工具参数描述接口，LLM 用于构造合法参数
type SchemaProvider interface {
	Parameters() ToolSchema
}
