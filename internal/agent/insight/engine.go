package insight

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// InsightsReport Token/费用/工具使用分析报告
type InsightsReport struct {
	Period         string            `json:"period"`
	TotalSessions  int               `json:"total_sessions"`
	TotalMessages  int               `json:"total_messages"`
	TotalTokens    TokenBreakdown    `json:"total_tokens"`
	EstimatedCost  CostBreakdown     `json:"estimated_cost"`
	ToolUsage      []*ToolUsageStat  `json:"tool_usage"`
	DailyActivity  []*DailyActivity  `json:"daily_activity"`
	ModelBreakdown []*ModelBreakdown `json:"model_breakdown"`
	AvgSessionLen  float64           `json:"avg_session_len"`
}

// TokenBreakdown Token 用量明细
type TokenBreakdown struct {
	Input  int64 `json:"input"`
	Output int64 `json:"output"`
	Total  int64 `json:"total"`
}

// CostBreakdown 费用估算明细
type CostBreakdown struct {
	TotalUSD float64            `json:"total_usd"`
	ByModel  map[string]float64 `json:"by_model"`
}

// ToolUsageStat 工具使用统计
type ToolUsageStat struct {
	ToolName    string  `json:"tool_name"`
	Count       int     `json:"count"`
	SuccessRate float64 `json:"success_rate"`
}

// DailyActivity 每日活动统计
type DailyActivity struct {
	Date     string `json:"date"`
	Sessions int    `json:"sessions"`
	Messages int    `json:"messages"`
	TokensIn int64  `json:"tokens_in"`
	TokensOut int64  `json:"tokens_out"`
}

// ModelBreakdown 模型用量明细
type ModelBreakdown struct {
	Model     string `json:"model"`
	Sessions  int    `json:"sessions"`
	TokensIn  int64  `json:"tokens_in"`
	TokensOut int64  `json:"tokens_out"`
}

// PricePer1K 模型每千 Token 定价
type PricePer1K struct {
	Input  float64
	Output float64
}

// modelPricing 模型定价表（USD / 1K tokens）
var modelPricing = map[string]PricePer1K{
	"gpt-4o":            {Input: 0.00250, Output: 0.01000},
	"gpt-4o-mini":       {Input: 0.00015, Output: 0.00060},
	"gpt-4.1":           {Input: 0.00200, Output: 0.00800},
	"gpt-4.1-mini":      {Input: 0.00040, Output: 0.00160},
	"claude-3.5-sonnet": {Input: 0.00300, Output: 0.01500},
	"claude-3-haiku":    {Input: 0.00080, Output: 0.00400},
	"deepseek-chat":     {Input: 0.00014, Output: 0.00028},
	"deepseek-reasoner": {Input: 0.00055, Output: 0.00219},
	"gemini-2.0-flash":  {Input: 0.00010, Output: 0.00040},
}

// InsightsEngine Token/费用/工具使用分析引擎
type InsightsEngine struct {
	db     *gorm.DB
	logger *zap.Logger
}

// NewInsightsEngine 创建分析引擎实例
func NewInsightsEngine(db *gorm.DB, logger *zap.Logger) *InsightsEngine {
	return &InsightsEngine{db: db, logger: logger}
}

// Generate 生成最近 N 天的分析报告
func (e *InsightsEngine) Generate(ctx context.Context, days int) (*InsightsReport, error) {
	if days <= 0 {
		days = 30
	}
	since := time.Now().AddDate(0, 0, -days)

	report := &InsightsReport{
		Period:        fmt.Sprintf("最近 %d 天", days),
		EstimatedCost: CostBreakdown{ByModel: make(map[string]float64)},
	}

	// 总会话数
	if err := e.db.WithContext(ctx).Raw(`
		SELECT COUNT(*) FROM cl_agent_sessions 
		WHERE updated_at >= ?
	`, since).Scan(&report.TotalSessions).Error; err != nil {
		e.logger.Error("查询总会话数失败", zap.Error(err))
		return nil, err
	}

	// Token 总量 + 总消息数
	var tokenResult struct {
		TokensInput  int64
		TokensOutput int64
		TotalMsg     int
	}
	if err := e.db.WithContext(ctx).Raw(`
		SELECT 
			COALESCE(SUM((metadata->>'tokens_input')::bigint), 0) as tokens_input,
			COALESCE(SUM((metadata->>'tokens_output')::bigint), 0) as tokens_output,
			COUNT(*) as total_msg
		FROM cl_agent_messages 
		WHERE role = 'assistant' AND created_at >= ?
	`, since).Scan(&tokenResult).Error; err != nil {
		e.logger.Error("查询 Token 总量失败", zap.Error(err))
		return nil, err
	}
	report.TotalMessages = tokenResult.TotalMsg
	report.TotalTokens = TokenBreakdown{
		Input:  tokenResult.TokensInput,
		Output: tokenResult.TokensOutput,
		Total:  tokenResult.TokensInput + tokenResult.TokensOutput,
	}

	// 平均会话长度
	if report.TotalSessions > 0 {
		report.AvgSessionLen = float64(report.TotalMessages) / float64(report.TotalSessions)
	}

	// 工具使用统计
	if err := e.queryToolUsage(ctx, since, report); err != nil {
		return nil, err
	}

	// 每日活动
	if err := e.queryDailyActivity(ctx, since, report); err != nil {
		return nil, err
	}

	// 模型用量明细
	if err := e.queryModelBreakdown(ctx, since, report); err != nil {
		return nil, err
	}

	// 费用估算
	e.calculateCosts(report)

	return report, nil
}

// queryToolUsage 查询工具使用统计
func (e *InsightsEngine) queryToolUsage(ctx context.Context, since time.Time, report *InsightsReport) error {
	type toolRow struct {
		ToolName string `gorm:"column:tool_name"`
		Count    int    `gorm:"column:count"`
	}
	var rows []toolRow
	if err := e.db.WithContext(ctx).Raw(`
		SELECT 
			tool_item->>'name' as tool_name,
			COUNT(*) as count
		FROM cl_agent_messages,
			 jsonb_array_elements(metadata->'tool_calls') as tool_item
		WHERE role = 'assistant' AND created_at >= ?
		GROUP BY tool_item->>'name'
		ORDER BY count DESC
	`, since).Scan(&rows).Error; err != nil {
		e.logger.Error("查询工具使用统计失败", zap.Error(err))
		return err
	}
	for _, r := range rows {
		report.ToolUsage = append(report.ToolUsage, &ToolUsageStat{
			ToolName:    r.ToolName,
			Count:       r.Count,
			SuccessRate: 1.0, // 默认成功率，后续可从 metadata 补充
		})
	}
	return nil
}

// queryDailyActivity 查询每日活动统计
func (e *InsightsEngine) queryDailyActivity(ctx context.Context, since time.Time, report *InsightsReport) error {
	type dayRow struct {
		Date      string `gorm:"column:date"`
		Sessions  int    `gorm:"column:sessions"`
		Messages  int    `gorm:"column:messages"`
		TokensIn  int64  `gorm:"column:tokens_in"`
		TokensOut int64  `gorm:"column:tokens_out"`
	}
	var rows []dayRow
	if err := e.db.WithContext(ctx).Raw(`
		SELECT 
			date(created_at) as date,
			COUNT(DISTINCT session_id) as sessions,
			COUNT(*) as messages,
			COALESCE(SUM((metadata->>'tokens_input')::bigint), 0) as tokens_in,
			COALESCE(SUM((metadata->>'tokens_output')::bigint), 0) as tokens_out
		FROM cl_agent_messages
		WHERE role = 'assistant' AND created_at >= ?
		GROUP BY date(created_at)
		ORDER BY date DESC
		LIMIT 30
	`, since).Scan(&rows).Error; err != nil {
		e.logger.Error("查询每日活动失败", zap.Error(err))
		return err
	}
	for _, r := range rows {
		report.DailyActivity = append(report.DailyActivity, &DailyActivity{
			Date:      r.Date,
			Sessions:  r.Sessions,
			Messages:  r.Messages,
			TokensIn:  r.TokensIn,
			TokensOut: r.TokensOut,
		})
	}
	return nil
}

// queryModelBreakdown 查询模型用量明细
func (e *InsightsEngine) queryModelBreakdown(ctx context.Context, since time.Time, report *InsightsReport) error {
	type modelRow struct {
		Model     string `gorm:"column:model"`
		Sessions  int    `gorm:"column:sessions"`
		TokensIn  int64  `gorm:"column:tokens_in"`
		TokensOut int64  `gorm:"column:tokens_out"`
	}
	var rows []modelRow
	if err := e.db.WithContext(ctx).Raw(`
		SELECT 
			COALESCE(metadata->>'model', 'unknown') as model,
			COUNT(DISTINCT session_id) as sessions,
			COALESCE(SUM((metadata->>'tokens_input')::bigint), 0) as tokens_in,
			COALESCE(SUM((metadata->>'tokens_output')::bigint), 0) as tokens_out
		FROM cl_agent_messages
		WHERE role = 'assistant' AND created_at >= ?
		GROUP BY metadata->>'model'
	`, since).Scan(&rows).Error; err != nil {
		e.logger.Error("查询模型用量明细失败", zap.Error(err))
		return err
	}
	for _, r := range rows {
		report.ModelBreakdown = append(report.ModelBreakdown, &ModelBreakdown{
			Model:     r.Model,
			Sessions:  r.Sessions,
			TokensIn:  r.TokensIn,
			TokensOut: r.TokensOut,
		})
	}
	return nil
}

// calculateCosts 计算费用估算
func (e *InsightsEngine) calculateCosts(report *InsightsReport) {
	var total float64
	for _, m := range report.ModelBreakdown {
		cost := estimateCost(m.Model, m.TokensIn, m.TokensOut)
		report.EstimatedCost.ByModel[m.Model] = cost
		total += cost
	}
	report.EstimatedCost.TotalUSD = total
}

// estimateCost 根据模型和 Token 用量估算费用
func estimateCost(model string, tokensIn, tokensOut int64) float64 {
	price, ok := modelPricing[model]
	if !ok {
		return 0
	}
	return float64(tokensIn)/1000*price.Input + float64(tokensOut)/1000*price.Output
}

// FormatJSON 将报告格式化为 JSON 字符串
func (e *InsightsEngine) FormatJSON(report *InsightsReport) (string, error) {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// FormatTerminal 将报告格式化为终端友好的文本
func (e *InsightsEngine) FormatTerminal(report *InsightsReport) string {
	lines := []string{
		fmt.Sprintf("───── VanPanel Insights (%s) ─────", report.Period),
		"",
		"📊 总览",
		fmt.Sprintf("  会话数: %-6d  消息数: %d", report.TotalSessions, report.TotalMessages),
		fmt.Sprintf("  输入 Token: %s  输出 Token: %s  合计: %s",
			formatNum(report.TotalTokens.Input),
			formatNum(report.TotalTokens.Output),
			formatNum(report.TotalTokens.Total)),
		fmt.Sprintf("  估算费用: $%.2f", report.EstimatedCost.TotalUSD),
	}

	// 平均会话长度
	if report.AvgSessionLen > 0 {
		lines = append(lines, fmt.Sprintf("  平均会话消息数: %.1f", report.AvgSessionLen))
	}

	// 工具使用
	if len(report.ToolUsage) > 0 {
		lines = append(lines, "", "🔧 工具使用")
		for _, t := range report.ToolUsage {
			lines = append(lines, fmt.Sprintf("  %-30s %4d 次", t.ToolName, t.Count))
		}
	}

	// 每日活动
	if len(report.DailyActivity) > 0 {
		lines = append(lines, "", "📅 每日活动")
		for _, d := range report.DailyActivity {
			lines = append(lines, fmt.Sprintf("  %s  会话:%-3d  消息:%-4d  Token入:%-8s  Token出:%-8s",
				d.Date, d.Sessions, d.Messages, formatNum(d.TokensIn), formatNum(d.TokensOut)))
		}
	}

	// 模型用量
	if len(report.ModelBreakdown) > 0 {
		lines = append(lines, "", "🤖 模型用量")
		for _, m := range report.ModelBreakdown {
			cost := report.EstimatedCost.ByModel[m.Model]
			lines = append(lines, fmt.Sprintf("  %-25s  会话:%-3d  Token入:%-8s  Token出:%-8s  费用:$%.2f",
				m.Model, m.Sessions, formatNum(m.TokensIn), formatNum(m.TokensOut), cost))
		}
	}

	return strings.Join(lines, "\n")
}

// formatNum 格式化数字为人类可读形式
func formatNum(n int64) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}
