package risk

import (
	"sync"
	"time"

	"go.uber.org/zap"
)

// ToolCallRecord 工具调用记录
type ToolCallRecord struct {
	Name      string
	Args      map[string]any
	Timestamp time.Time
	RiskLevel RiskLevel
}

// BehaviorAlert 行为异常告警
type BehaviorAlert struct {
	Pattern  string // "brute_force" / "loop_stuck" / "privilege_escalation"
	Severity string // "warning" / "critical"
	Details  string
}

// BehaviorTracker 行为序列分析器
type BehaviorTracker struct {
	mu           sync.Mutex
	sessionCalls map[string][]ToolCallRecord
	maxRecords   int
	logger       *zap.Logger
}

// NewBehaviorTracker 创建行为追踪器
func NewBehaviorTracker(logger *zap.Logger) *BehaviorTracker {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &BehaviorTracker{
		sessionCalls: make(map[string][]ToolCallRecord),
		maxRecords:   100,
		logger:       logger,
	}
}

// Record 记录工具调用并检测异常模式
func (bt *BehaviorTracker) Record(sessionID string, call ToolCallRecord) *BehaviorAlert {
	bt.mu.Lock()
	defer bt.mu.Unlock()

	records := bt.sessionCalls[sessionID]
	records = append(records, call)
	if len(records) > bt.maxRecords {
		records = records[len(records)-bt.maxRecords:]
	}
	bt.sessionCalls[sessionID] = records

	// 检测模式
	if alert := bt.detectBruteForce(records); alert != nil {
		return alert
	}
	if alert := bt.detectLoopStuck(records); alert != nil {
		return alert
	}
	return nil
}

func (bt *BehaviorTracker) detectBruteForce(records []ToolCallRecord) *BehaviorAlert {
	if len(records) < 5 {
		return nil
	}
	// 最近 5 分钟内的高风险调用
	cutoff := time.Now().Add(-5 * time.Minute)
	highRiskCount := 0
	for _, r := range records {
		if r.Timestamp.After(cutoff) && r.RiskLevel == RiskLevelHigh {
			highRiskCount++
		}
	}
	if highRiskCount >= 5 {
		return &BehaviorAlert{
			Pattern:  "brute_force",
			Severity: "critical",
			Details:  "5 分钟内检测到 5 次以上高风险调用",
		}
	}
	return nil
}

func (bt *BehaviorTracker) detectLoopStuck(records []ToolCallRecord) *BehaviorAlert {
	if len(records) < 4 {
		return nil
	}
	// 最后 4 次调用使用同一工具
	last := records[len(records)-1]
	if last.RiskLevel == RiskLevelHigh {
		return nil
	}
	sameCount := 1
	for i := len(records) - 2; i >= len(records)-4 && i >= 0; i-- {
		if records[i].Name == last.Name {
			sameCount++
		}
	}
	if sameCount >= 4 {
		return &BehaviorAlert{
			Pattern:  "loop_stuck",
			Severity: "warning",
			Details:  "同一工具连续调用 4 次以上",
		}
	}
	return nil
}

// ClearSession 清除会话记录
func (bt *BehaviorTracker) ClearSession(sessionID string) {
	bt.mu.Lock()
	defer bt.mu.Unlock()
	delete(bt.sessionCalls, sessionID)
}
