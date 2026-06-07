package guard

import (
	"context"
	"encoding/json"
	"sort"

	agentmodel "github.com/rizxfrog/VanPanelBackend/internal/agent/model"
	"github.com/rizxfrog/VanPanelBackend/internal/agent/risk"
	"github.com/rizxfrog/VanPanelBackend/internal/agent/spi"
)

// Chain 双层防线 GuardChain
// 职责：组合规则引擎（第一关）+ 审计模型（第二关）+ SPI 自定义规则
type Chain struct {
	evaluator *risk.Evaluator
	auditor   *Auditor
	rules     []spi.GuardRule
}

// NewChain 创建 GuardChain
// auditor 可以为 nil（如果未配置审计模型），此时只使用规则引擎
func NewChain(evaluator *risk.Evaluator, auditor *Auditor) *Chain {
	return &Chain{
		evaluator: evaluator,
		auditor:   auditor,
	}
}

// RegisterRule 注册 SPI 自定义规则，按优先级排序
func (c *Chain) RegisterRule(rule spi.GuardRule) {
	c.rules = append(c.rules, rule)
	sort.Slice(c.rules, func(i, j int) bool {
		return c.rules[i].Priority() < c.rules[j].Priority()
	})
}

// Evaluate 执行完整审查管道
// 返回 RiskDecision，调用方根据 decision 决定是否执行
func (c *Chain) Evaluate(ctx context.Context, toolName string, toolArgs map[string]any) *agentmodel.RiskDecision {
	// === SPI 自定义规则（最先执行） ===
	for _, rule := range c.rules {
		decision, err := rule.Evaluate(ctx, toolName, toolArgs)
		if err != nil {
			continue // 规则出错跳过，不影响后续
		}
		if !decision.Allowed {
			return &agentmodel.RiskDecision{
				Level:            agentmodel.RiskHigh,
				Allowed:          false,
				RequiresApproval: false,
				Reason:           decision.Reason,
			}
		}
	}

	// === 第一关：规则引擎 ===
	if c.evaluator != nil {
		argsStr := toolArgsToString(toolArgs)
		result := c.evaluator.Evaluate(toolName, argsStr)

		if result.Blocked {
			level := agentmodel.RiskHigh
			if result.Level == risk.RiskLevelLow {
				level = agentmodel.RiskLow
			}
			return &agentmodel.RiskDecision{
				Level:   level,
				Allowed: false,
				Reason:  result.Reason,
			}
		}
		// 未被拦截，继续到第二关（审计模型）
	}

	// === 第二关：审计模型 ===
	if c.auditor != nil {
		// TODO: 审计模型尚未实现（Task 5），实现后在此调用
		// decision := c.auditor.Audit(ctx, toolName, toolArgs)
		// if !decision.Allowed { return decision }
	}

	// 无规则引擎时默认允许
	return &agentmodel.RiskDecision{
		Level:            agentmodel.RiskSafe,
		Allowed:          true,
		RequiresApproval: false,
	}
}

// toolArgsToString 将工具参数映射转换为字符串供规则引擎检查
func toolArgsToString(args map[string]any) string {
	// 优先提取 command 字段（shell 命令）
	if cmd, ok := args["command"]; ok {
		if cmdStr, ok := cmd.(string); ok {
			return cmdStr
		}
	}
	// 降级：序列化整个 args
	b, err := json.Marshal(args)
	if err != nil {
		return ""
	}
	return string(b)
}

// Auditor 审计模型桩（将在 Task 5 中实现）
type Auditor struct{}
