package pipeline

import (
	"context"
	"io"

	"github.com/rizxfrog/VanPanelBackend/internal/agent/spi"
	"go.uber.org/zap"
)

// Stage 6-stage pipeline orchestrator
type Stage struct {
	IntentAnalyzer spi.IntentAnalyzer
	MemoryProvider spi.MemoryProvider
	Logger         *zap.Logger
}

func NewStage(intentAnalyzer spi.IntentAnalyzer, memoryProvider spi.MemoryProvider, logger *zap.Logger) *Stage {
	return &Stage{
		IntentAnalyzer: intentAnalyzer,
		MemoryProvider: memoryProvider,
		Logger:         logger,
	}
}

// PipelineContext 在 pipeline 阶段之间传递的上下文
type PipelineContext struct {
	UserInput    string
	SessionID    string
	UserID       int
	Username     string
	IntentResult *spi.IntentResult
	Memories     []spi.MemoryEntry
	Writer       io.Writer // nil for sync mode
}

// RunIntentAnalysis 阶段①: 意图分析
func (s *Stage) RunIntentAnalysis(ctx context.Context, pc *PipelineContext) error {
	if s.IntentAnalyzer == nil {
		return nil
	}
	result, err := s.IntentAnalyzer.Analyze(ctx, pc.UserInput)
	if err != nil {
		s.Logger.Warn("intent analysis failed", zap.Error(err))
		return nil // 意图分析失败不影响主流程
	}
	pc.IntentResult = result
	return nil
}

// RunMemoryEnrichment 阶段②: 上下文增强
func (s *Stage) RunMemoryEnrichment(ctx context.Context, pc *PipelineContext) (string, error) {
	if s.MemoryProvider == nil {
		return "", nil
	}
	entries, err := s.MemoryProvider.Retrieve(ctx, pc.UserInput, pc.SessionID)
	if err != nil {
		s.Logger.Warn("memory enrichment failed", zap.Error(err))
		return "", nil
	}
	pc.Memories = entries

	// 将检索结果拼接为上下文注入文本
	if len(entries) == 0 {
		return "", nil
	}
	contextStr := "## 相关历史记忆\n"
	for _, entry := range entries {
		contextStr += "- " + entry.Content + "\n"
	}
	return contextStr, nil
}

// IsInjectionAttempt 阶段①后的快速检查：是否检测到注入攻击
func (s *Stage) IsInjectionAttempt(pc *PipelineContext) (bool, string) {
	if pc.IntentResult == nil {
		return false, ""
	}
	for _, tag := range pc.IntentResult.RiskTags {
		if tag == "prompt_injection" {
			return true, pc.IntentResult.BlockReason
		}
	}
	return false, ""
}
