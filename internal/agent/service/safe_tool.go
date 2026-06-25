package service

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	agentmodel "github.com/rizxfrog/VanPanelBackend/internal/agent/model"
	"github.com/rizxfrog/VanPanelBackend/internal/agent/risk"
	"go.uber.org/zap"
)

// resultCallbackType is the callback type, now includes toolCallID and status.
type resultCallbackType func(toolCallID, toolName, result, status string)

// preCallbackType is called BEFORE the tool executes, so the client knows
// which tool is about to be invoked even if execution fails afterwards.
type preCallbackType func(toolCallID, toolName, args string)

// safeTool wraps an InvokableTool to intercept tool execution with risk evaluation and audit.
type safeTool struct {
	inner          tool.InvokableTool
	riskEval       *risk.Evaluator
	auditFn        func(ctx context.Context, action, toolName, reason string, riskLevel agentmodel.RiskLevel, allowed bool, args string, result string)
	info           *schema.ToolInfo
	preCallback    preCallbackType
	resultCallback resultCallbackType
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
	preCallback preCallbackType,
	resultCallback resultCallbackType,
) (tool.BaseTool, error) {
	it, ok := t.(tool.InvokableTool)
	if !ok {
		return t, fmt.Errorf("tool is not InvokableTool")
	}
	info, err := it.Info(context.Background())
	if err != nil {
		return t, fmt.Errorf("tool Info failed: %w", err)
	}
	return &safeTool{inner: it, riskEval: riskEval, auditFn: auditFn, info: info, preCallback: preCallback, resultCallback: resultCallback}, nil
}

func (st *safeTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return st.inner.Info(ctx)
}

func (st *safeTool) InvokableRun(ctx context.Context, argsInJSON string, opts ...tool.Option) (string, error) {
	// 从 Eino 框架上下文中提取 tool call ID（LLM 生成的唯一标识）
	toolCallID := compose.GetToolCallID(ctx)

	zap.L().Info("[ToolCall] 开始执行工具",
		zap.String("tool", st.info.Name),
		zap.String("callID", toolCallID),
		zap.String("args", argsInJSON),
	)

	// 预执行回调：通知客户端即将调用哪个工具（即使后续执行失败，前端也能看到工具调用信息）
	if st.preCallback != nil {
		st.preCallback(toolCallID, st.info.Name, argsInJSON)
	}

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
			if st.resultCallback != nil {
				st.resultCallback(toolCallID, st.info.Name, blockedMsg, "error")
			}
			return blockedMsg, nil
		}
	}

	result, err := st.inner.InvokableRun(ctx, argsInJSON, opts...)
	if err != nil {
		zap.L().Warn("[ToolCall] 工具执行失败（将返回文本结果供 LLM 重试）",
			zap.String("tool", st.info.Name),
			zap.String("callID", toolCallID),
			zap.String("args", argsInJSON),
			zap.Error(err),
		)

		// 关键改动：工具执行失败时，将错误信息作为文本结果返回给 Eino 框架，
		// 而不是返回 Go error。这样 LLM 能看到失败原因并自动重试修正后的命令，
		// 符合 ReAct 模式的设计理念（MaxStep=10 限制了最大重试次数）。
		errorMsg := fmt.Sprintf("❌ 执行失败: %s\n\n%s 调用参数: %s\n错误详情: %s",
			st.info.Name, st.info.Name, argsInJSON, err.Error())

		if st.auditFn != nil {
			st.auditFn(ctx, "tool.execute", st.info.Name, "execution failed",
				agentmodel.RiskSafe, true, argsInJSON, truncateString(errorMsg, 2000))
		}
		if st.resultCallback != nil {
			st.resultCallback(toolCallID, st.info.Name, errorMsg, "error")
		}
		// 返回 nil error 是关键：让 Eino 将此作为正常工具结果传递给 LLM
		return errorMsg, nil
	}

	if st.auditFn != nil {
		st.auditFn(ctx, "tool.execute", st.info.Name, "",
			agentmodel.RiskSafe, true, argsInJSON, truncateString(result, 2000))
	}

	if st.resultCallback != nil {
		st.resultCallback(toolCallID, st.info.Name, result, "success")
	}

	zap.L().Info("[ToolCall] 工具执行成功",
		zap.String("tool", st.info.Name),
		zap.String("callID", toolCallID),
		zap.String("args", argsInJSON),
		zap.String("result", truncateString(result, 200)),
	)

	return result, nil
}

// truncateString 截断字符串到指定长度，超出部分用 "..." 替代
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
