package runtime

import "context"

// CapsuleExecutor 执行隔离接口
type CapsuleExecutor interface {
	// Execute 在隔离环境中执行工具调用
	Execute(ctx context.Context, call ToolCall) (*CapsuleOutput, error)
	// Name 返回执行器名称
	Name() string
}
