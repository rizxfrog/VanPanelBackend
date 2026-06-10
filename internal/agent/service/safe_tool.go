package service

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	agentmodel "github.com/rizxfrog/VanPanelBackend/internal/agent/model"
	"github.com/rizxfrog/VanPanelBackend/internal/agent/risk"
)

// safeTool wraps an InvokableTool to intercept tool execution with risk evaluation and audit.
type safeTool struct {
	inner          tool.InvokableTool
	riskEval       *risk.Evaluator
	auditFn        func(ctx context.Context, action, toolName, reason string, riskLevel agentmodel.RiskLevel, allowed bool, args string, result string)
	info           *schema.ToolInfo
	resultCallback func(toolName, result string)
}

func wrapTool(
	t tool.BaseTool,
	riskEval *risk.Evaluator,
	auditFn func(ctx context.Context, action, toolName, reason string, riskLevel agentmodel.RiskLevel, allowed bool, args string, result string),
) (tool.BaseTool, error) {
	it, ok := t.(tool.InvokableTool)
	if !ok {
		return t, fmt.Errorf("tool is not InvokableTool")
	}
	info, err := it.Info(context.Background())
	if err != nil {
		return t, fmt.Errorf("tool Info failed: %w", err)
	}
	return &safeTool{inner: it, riskEval: riskEval, auditFn: auditFn, info: info}, nil
}

func wrapToolWithCallback(
	t tool.BaseTool,
	riskEval *risk.Evaluator,
	auditFn func(ctx context.Context, action, toolName, reason string, riskLevel agentmodel.RiskLevel, allowed bool, args string, result string),
	resultCallback func(toolName, result string),
) (tool.BaseTool, error) {
	it, ok := t.(tool.InvokableTool)
	if !ok {
		return t, fmt.Errorf("tool is not InvokableTool")
	}
	info, err := it.Info(context.Background())
	if err != nil {
		return t, fmt.Errorf("tool Info failed: %w", err)
	}
	return &safeTool{inner: it, riskEval: riskEval, auditFn: auditFn, info: info, resultCallback: resultCallback}, nil
}

func (st *safeTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return st.inner.Info(ctx)
}

func (st *safeTool) InvokableRun(ctx context.Context, argsInJSON string, opts ...tool.Option) (string, error) {
	if st.riskEval != nil {
		evalResult := st.riskEval.Evaluate(st.info.Name, argsInJSON)

		// 审计: 评估
		if st.auditFn != nil {
			st.auditFn(ctx, "tool.evaluate", st.info.Name, evalResult.Reason,
				agentmodel.RiskLevel(evalResult.Level), !evalResult.Blocked, argsInJSON, "")
		}

		if evalResult.Blocked {
			blockedMsg := fmt.Sprintf("[安全拦截] 操作被安全策略阻止\n原因: %s\n工具: %s\n建议: 请尝试更安全的替代方案",
				evalResult.Reason, st.info.Name)
			if st.auditFn != nil {
				st.auditFn(ctx, "tool.blocked", st.info.Name, evalResult.Reason,
					agentmodel.RiskLevel(evalResult.Level), false, argsInJSON, blockedMsg)
			}
			return blockedMsg, nil
		}
	}

	result, err := st.inner.InvokableRun(ctx, argsInJSON, opts...)
	if err != nil {
		return "", err
	}

	if st.auditFn != nil {
		st.auditFn(ctx, "tool.execute", st.info.Name, "",
			agentmodel.RiskSafe, true, argsInJSON, truncateString(result, 2000))
	}

	if st.resultCallback != nil {
		st.resultCallback(st.info.Name, truncateString(result, 2000))
	}

	return result, nil
}

// truncateString 截断字符串到指定长度，超出部分用 "..." 替代
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
