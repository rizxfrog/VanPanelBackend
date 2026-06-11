package insight

import (
	"math"
	"testing"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

func TestNewInsightsEngine(t *testing.T) {
	engine := NewInsightsEngine(&gorm.DB{}, zap.NewNop())
	if engine == nil {
		t.Fatal("NewInsightsEngine 返回 nil")
	}
	if engine.db == nil {
		t.Fatal("engine.db 为 nil")
	}
	if engine.logger == nil {
		t.Fatal("engine.logger 为 nil")
	}
}

func TestFormatJSON(t *testing.T) {
	engine := NewInsightsEngine(nil, nil)
	report := &InsightsReport{
		Period:        "最近 7 天",
		TotalSessions: 10,
		TotalMessages: 50,
		TotalTokens: TokenBreakdown{
			Input:  10000,
			Output: 5000,
			Total:  15000,
		},
		EstimatedCost: CostBreakdown{
			TotalUSD: 0.05,
			ByModel: map[string]float64{
				"gpt-4o": 0.05,
			},
		},
		AvgSessionLen: 5.0,
		ToolUsage: []*ToolUsageStat{
			{ToolName: "read_file", Count: 10, SuccessRate: 1.0},
		},
		DailyActivity: []*DailyActivity{
			{Date: "2026-06-12", Sessions: 3, Messages: 15, TokensIn: 3000, TokensOut: 1500},
		},
		ModelBreakdown: []*ModelBreakdown{
			{Model: "gpt-4o", Sessions: 10, TokensIn: 10000, TokensOut: 5000},
		},
	}

	jsonStr, err := engine.FormatJSON(report)
	if err != nil {
		t.Fatalf("FormatJSON 失败: %v", err)
	}
	if jsonStr == "" {
		t.Fatal("FormatJSON 返回空字符串")
	}

	// 验证 JSON 包含关键字段
	checks := []string{
		`"period"`,
		`"total_sessions"`,
		`"total_tokens"`,
		`"estimated_cost"`,
		`"tool_usage"`,
		`"daily_activity"`,
		`"model_breakdown"`,
		`"avg_session_len"`,
	}
	for _, c := range checks {
		if !contains(jsonStr, c) {
			t.Errorf("FormatJSON 输出缺少字段: %s", c)
		}
	}
}

func TestFormatTerminal(t *testing.T) {
	engine := NewInsightsEngine(nil, nil)
	report := &InsightsReport{
		Period:        "最近 7 天",
		TotalSessions: 5,
		TotalMessages: 25,
		TotalTokens: TokenBreakdown{
			Input:  5000,
			Output: 2500,
			Total:  7500,
		},
		EstimatedCost: CostBreakdown{
			TotalUSD: 0.03,
			ByModel: map[string]float64{
				"deepseek-chat": 0.03,
			},
		},
		AvgSessionLen: 5.0,
		ToolUsage: []*ToolUsageStat{
			{ToolName: "search_docs", Count: 8, SuccessRate: 1.0},
		},
		DailyActivity: []*DailyActivity{
			{Date: "2026-06-12", Sessions: 2, Messages: 10, TokensIn: 2000, TokensOut: 1000},
		},
		ModelBreakdown: []*ModelBreakdown{
			{Model: "deepseek-chat", Sessions: 5, TokensIn: 5000, TokensOut: 2500},
		},
	}

	output := engine.FormatTerminal(report)

	// 验证包含关键部分
	sections := []string{
		"VanPanel Insights",
		"最近 7 天",
		"总览",
		"会话数",
		"消息数",
		"Token",
		"估算费用",
		"工具使用",
		"每日活动",
		"模型用量",
	}
	for _, s := range sections {
		if !contains(output, s) {
			t.Errorf("FormatTerminal 输出缺少部分: %s", s)
		}
	}
}

func TestEstimateCost(t *testing.T) {
	tests := []struct {
		model           string
		tokensIn        int64
		tokensOut       int64
		expectedMinimum float64
		expectedExact   float64
	}{
		{"gpt-4o", 1000, 1000, 0.01, 0.0125},
		{"gpt-4o-mini", 1000, 1000, 0.0005, 0.00075},
		{"deepseek-chat", 1000, 1000, 0.0003, 0.00042},
		{"unknown-model", 1000, 1000, 0, 0},
		{"gpt-4o", 0, 0, 0, 0},
	}

	const epsilon = 1e-9
	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			got := estimateCost(tt.model, tt.tokensIn, tt.tokensOut)
			if tt.expectedExact > 0 && math.Abs(got-tt.expectedExact) > epsilon {
				t.Errorf("estimateCost(%q, %d, %d) = %f, 期望 %f",
					tt.model, tt.tokensIn, tt.tokensOut, got, tt.expectedExact)
			}
			if tt.expectedExact == 0 && got != 0 {
				t.Errorf("estimateCost(%q, %d, %d) = %f, 期望 0",
					tt.model, tt.tokensIn, tt.tokensOut, got)
			}
		})
	}
}

func TestFormatNum(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{0, "0"},
		{500, "500"},
		{999, "999"},
		{1000, "1.0K"},
		{1500, "1.5K"},
		{9999, "10.0K"},
		{1_000_000, "1.0M"},
		{2_500_000, "2.5M"},
		{999_999_999, "1000.0M"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := formatNum(tt.input)
			if got != tt.expected {
				t.Errorf("formatNum(%d) = %q, 期望 %q", tt.input, got, tt.expected)
			}
		})
	}
}

// contains 检查字符串是否包含子串
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
