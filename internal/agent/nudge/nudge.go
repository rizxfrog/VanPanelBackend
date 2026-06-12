package nudge

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// NudgeConfig 记忆审查配置
type NudgeConfig struct {
	MemoryInterval int // 记忆审查间隔 (轮数), default 10
	SkillInterval  int // skill 审查间隔 (工具调用数), default 10
}

// NudgeState 记忆审查状态
type NudgeState struct {
	mu                sync.Mutex
	TurnCount         int
	ToolCallCount     int
	LastMemoryNudgeAt time.Time
	LastSkillNudgeAt  time.Time
}

// MemoryNudgeReviewer 记忆审查器
type MemoryNudgeReviewer struct {
	state     *NudgeState
	config    NudgeConfig
	memoryDir string // data/memory/
	logger    *zap.Logger

	// LLM call function — injected to avoid circular deps
	// Signature: func(ctx, systemPrompt, userPrompt string) (string, error)
	llmCall func(ctx context.Context, systemPrompt, userPrompt string) (string, error)
}

const reviewMemoryPrompt = `你是一个记忆审查助手。回顾以下对话，考虑是否要保存到记忆中。

关注:
1. 用户是否透露了个人信息 — 角色、偏好、个人细节?
2. 用户是否表达了对你行为方式的期望、工作风格偏好?
3. 用户是否提到了重要事实、决策或约束?

如果没有值得记录的，只回复 {"action": "skip"}
如果有值得记录的，回复:
{"action": "add", "content": "需要记录的内容", "importance": "high|medium|low"}`

// MemoryReviewResult LLM memory review response
type MemoryReviewResult struct {
	Action     string `json:"action"`
	Content    string `json:"content,omitempty"`
	Importance string `json:"importance,omitempty"`
}

// NewMemoryNudgeReviewer 创建记忆审查器
func NewMemoryNudgeReviewer(cfg NudgeConfig, memoryDir string, logger *zap.Logger) *MemoryNudgeReviewer {
	if cfg.MemoryInterval <= 0 {
		cfg.MemoryInterval = 10
	}
	if cfg.SkillInterval <= 0 {
		cfg.SkillInterval = 10
	}
	if memoryDir == "" {
		memoryDir = "data/memory"
	}
	return &MemoryNudgeReviewer{
		state:     &NudgeState{},
		config:    cfg,
		memoryDir: memoryDir,
		logger:    logger,
	}
}

// SetLLMCall sets the LLM function for review (set after construction to avoid circular deps)
func (r *MemoryNudgeReviewer) SetLLMCall(fn func(ctx context.Context, systemPrompt, userPrompt string) (string, error)) {
	r.llmCall = fn
}

// ShouldNudge checks if nudge should be triggered
func (r *MemoryNudgeReviewer) ShouldNudge(toolCallsThisTurn int) (memoryNudge, skillNudge bool) {
	r.state.mu.Lock()
	defer r.state.mu.Unlock()

	if r.state.TurnCount >= r.config.MemoryInterval {
		memoryNudge = true
	}
	if r.state.ToolCallCount >= r.config.SkillInterval {
		skillNudge = true
	}
	return
}

// RecordTurn increments counters after a turn completes
func (r *MemoryNudgeReviewer) RecordTurn(toolCalls int) {
	r.state.mu.Lock()
	defer r.state.mu.Unlock()
	r.state.TurnCount++
	r.state.ToolCallCount += toolCalls
}

// ResetMemory resets memory nudge counter
func (r *MemoryNudgeReviewer) ResetMemory() {
	r.state.mu.Lock()
	defer r.state.mu.Unlock()
	r.state.TurnCount = 0
	r.state.LastMemoryNudgeAt = time.Now()
}

// ResetSkill resets skill nudge counter
func (r *MemoryNudgeReviewer) ResetSkill() {
	r.state.mu.Lock()
	defer r.state.mu.Unlock()
	r.state.ToolCallCount = 0
	r.state.LastSkillNudgeAt = time.Now()
}

// Review performs memory review using LLM and writes to MEMORY.md
// Call in a goroutine — it does NOT return errors to avoid blocking
func (r *MemoryNudgeReviewer) Review(ctx context.Context, conversationText string) {
	if r.llmCall == nil {
		r.logger.Warn("记忆审查跳过：LLM 调用函数未设置")
		return
	}

	// Build prompt
	userPrompt := fmt.Sprintf("对话内容:\n\n%s", conversationText)

	// Call LLM
	resp, err := r.llmCall(ctx, reviewMemoryPrompt, userPrompt)
	if err != nil {
		r.logger.Error("记忆审查 LLM 调用失败", zap.Error(err))
		return
	}

	// Parse JSON response
	var result MemoryReviewResult
	// Try to extract JSON from response (may be wrapped in markdown)
	resp = extractJSON(resp)
	if err := json.Unmarshal([]byte(resp), &result); err != nil {
		r.logger.Warn("记忆审查响应解析失败", zap.Error(err), zap.String("response", resp))
		return
	}

	if result.Action != "add" || result.Content == "" {
		return
	}

	// Dedup check — check if similar content already exists
	if r.isDuplicate(result.Content) {
		r.logger.Debug("记忆审查跳过：内容已存在")
		return
	}

	// Write to MEMORY.md
	if err := r.writeMemory(result.Content); err != nil {
		r.logger.Error("写入 MEMORY.md 失败", zap.Error(err))
	}
}

// isDuplicate checks if content already exists in MEMORY.md
func (r *MemoryNudgeReviewer) isDuplicate(content string) bool {
	data, err := os.ReadFile(filepath.Join(r.memoryDir, "MEMORY.md"))
	if err != nil {
		return false // file doesn't exist yet, no duplicate possible
	}
	// Simple keyword overlap check — if any 3-word phrase matches
	words := strings.Fields(content)
	existingContent := string(data)
	matches := 0
	for i := 0; i < len(words)-2; i++ {
		phrase := strings.Join(words[i:i+3], " ")
		if strings.Contains(existingContent, phrase) {
			matches++
		}
	}
	return matches >= 2 // at least 2 phrases match = likely duplicate
}

// writeMemory appends to MEMORY.md with date header
func (r *MemoryNudgeReviewer) writeMemory(content string) error {
	if err := os.MkdirAll(r.memoryDir, 0755); err != nil {
		return fmt.Errorf("创建记忆目录失败: %w", err)
	}

	memPath := filepath.Join(r.memoryDir, "MEMORY.md")
	today := time.Now().Format("2006-01-02")

	// Read existing to check if today's header exists
	existing, _ := os.ReadFile(memPath)
	hasTodayHeader := strings.Contains(string(existing), "## "+today)

	f, err := os.OpenFile(memPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("打开 MEMORY.md 失败: %w", err)
	}
	defer f.Close()

	var entry strings.Builder
	if !hasTodayHeader {
		if len(existing) > 0 {
			entry.WriteString("\n")
		}
		entry.WriteString(fmt.Sprintf("## %s\n", today))
	}
	entry.WriteString(fmt.Sprintf("- %s\n", content))

	_, err = f.WriteString(entry.String())
	return err
}

// extractJSON extracts JSON from markdown code blocks
func extractJSON(text string) string {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "```json") {
		text = strings.TrimPrefix(text, "```json")
		if idx := strings.Index(text, "```"); idx > 0 {
			text = text[:idx]
		}
	} else if strings.HasPrefix(text, "```") {
		text = strings.TrimPrefix(text, "```")
		if idx := strings.Index(text, "```"); idx > 0 {
			text = text[:idx]
		}
	}
	text = strings.TrimSpace(text)
	// Find outermost braces
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start >= 0 && end > start {
		return text[start : end+1]
	}
	return text
}
