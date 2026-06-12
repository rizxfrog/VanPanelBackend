package pipeline

import (
	"context"

	"github.com/rizxfrog/VanPanelBackend/internal/agent/spi"
	"go.uber.org/zap"
)

// HybridIntentAnalyzer 混合意图分析器
// 第一层：正则规则（DB 配置）快速过滤
// 第二层：LLM 语义检测（DB 配置的 system prompt）深度审查
// 第三层：DefaultIntentAnalyzer（硬编码规则 + 关键词意图分类）
type HybridIntentAnalyzer struct {
	injectRulesGetter func(context.Context) ([]InjectRule, error)
	auditPromptGetter func(context.Context) (*LLMAuditPrompt, error)
	llmAuditor        *LLMAuditor
	fallback          *DefaultIntentAnalyzer
	logger            *zap.Logger
}

// NewHybridIntentAnalyzer creates a HybridIntentAnalyzer with callback functions
// to avoid circular imports between pipeline and service packages.
func NewHybridIntentAnalyzer(
	injectRulesGetter func(context.Context) ([]InjectRule, error),
	auditor *LLMAuditor,
	auditPromptGetter func(context.Context) (*LLMAuditPrompt, error),
	logger *zap.Logger,
) *HybridIntentAnalyzer {
	return &HybridIntentAnalyzer{
		injectRulesGetter: injectRulesGetter,
		auditPromptGetter: auditPromptGetter,
		llmAuditor:        auditor,
		fallback:          NewDefaultIntentAnalyzer(),
		logger:            logger,
	}
}

// Analyze implements spi.IntentAnalyzer
func (a *HybridIntentAnalyzer) Analyze(ctx context.Context, userInput string) (*spi.IntentResult, error) {
	// 第一层：正则规则
	rules, err := a.injectRulesGetter(ctx)
	if err != nil {
		a.logger.Warn("加载注入规则失败，使用 fallback", zap.Error(err))
		return a.fallback.Analyze(ctx, userInput)
	}
	for _, rule := range rules {
		if rule.Re != nil && rule.Re.MatchString(userInput) {
			return &spi.IntentResult{
				Intent:      "dangerous",
				RiskTags:    []string{"prompt_injection"},
				Confidence:  1.0,
				BlockReason: "检测到注入攻击: " + rule.Desc,
			}, nil
		}
	}

	// 第二层：LLM 审计
	if a.llmAuditor != nil {
		promptCfg, err := a.auditPromptGetter(ctx)
		if err != nil {
			a.logger.Warn("加载 LLM 审计配置失败", zap.Error(err))
		} else if promptCfg.Enabled {
			result, err := a.llmAuditor.Audit(ctx, userInput,
				promptCfg.Model,
				promptCfg.SystemPrompt,
				promptCfg.Temperature,
				promptCfg.MaxTokens,
				promptCfg.MaxRetries,
			)
			if err != nil {
				a.logger.Warn("LLM 审计调用失败", zap.Error(err))
			} else if !result.Safe {
				return &spi.IntentResult{
					Intent:      "dangerous",
					RiskTags:    []string{"prompt_injection", "llm_detected"},
					Confidence:  0.9,
					BlockReason: result.Reason,
				}, nil
			}
		}
	}

	// 第三层：fallback 关键词意图分类
	return a.fallback.Analyze(ctx, userInput)
}
