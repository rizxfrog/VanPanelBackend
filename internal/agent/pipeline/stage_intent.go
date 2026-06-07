package pipeline

import (
	"context"
	"regexp"
	"strings"

	"github.com/rizxfrog/VanPanelBackend/internal/agent/spi"
)

// DefaultIntentAnalyzer 默认意图分析器（规则优先，可选 LLM 增强）
type DefaultIntentAnalyzer struct {
	InjectionPatterns []*regexp.Regexp
}

func NewDefaultIntentAnalyzer() *DefaultIntentAnalyzer {
	return &DefaultIntentAnalyzer{
		InjectionPatterns: []*regexp.Regexp{
			// 角色劫持
			regexp.MustCompile(`(?i)(ignore|forget|disregard)\s+(previous|all|above)\s+(instructions|prompts|rules)`),
			// 越狱
			regexp.MustCompile(`(?i)(you\s+are\s+now|act\s+as|pretend\s+to\s+be|you\s+must\s+obey)`),
			// 指令覆盖
			regexp.MustCompile(`(?i)(new\s+system\s+prompt|override\s+system|system:\s*you)`),
			// 代码注入通过自然语言
			regexp.MustCompile(`(?i)(execute\s+this\s+code|run\s+this\s+command\s+without\s+question)`),
		},
	}
}

func (a *DefaultIntentAnalyzer) Analyze(ctx context.Context, userInput string) (*spi.IntentResult, error) {
	result := &spi.IntentResult{
		Confidence: 1.0,
	}

	// Step 1: 注入检测
	for _, pattern := range a.InjectionPatterns {
		if pattern.MatchString(userInput) {
			result.RiskTags = append(result.RiskTags, "prompt_injection")
			result.BlockReason = "检测到提示词注入攻击特征"
			return result, nil
		}
	}

	// Step 2: 意图分类（关键词规则）
	lower := strings.ToLower(userInput)
	switch {
	case containsAny(lower, "清理", "删除", "删除", "修改", "重启", "停止", "kill", "rm ", "chmod", "chown"):
		result.Intent = "dangerous"
		result.RiskTags = append(result.RiskTags, "destructive_operation")
	case containsAny(lower, "查看", "检查", "巡检", "状态", "top", "free", "df ", "ps ", "ss ", "show", "list", "get"):
		result.Intent = "inspect"
	case containsAny(lower, "分析", "排查", "诊断", "原因", "为什么", "问题"):
		result.Intent = "diagnose"
	default:
		result.Intent = "query"
	}
	return result, nil
}

func containsAny(s string, keywords ...string) bool {
	for _, kw := range keywords {
		if strings.Contains(s, kw) {
			return true
		}
	}
	return false
}
