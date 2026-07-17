package risk

import (
	"testing"
	"time"
)

func TestBehaviorTracker_DetectsBruteForce(t *testing.T) {
	bt := NewBehaviorTracker(nil)
	sessionID := "test-session"
	// 模拟 5 分钟内 5 次高风险调用
	for i := 0; i < 5; i++ {
		call := ToolCallRecord{Name: "shell.exec", RiskLevel: RiskLevelHigh, Timestamp: time.Now()}
		alert := bt.Record(sessionID, call)
		if i < 4 && alert != nil {
			t.Fatalf("should not alert on call %d", i)
		}
	}
	// 第 5 次应该触发告警
	call := ToolCallRecord{Name: "shell.exec", RiskLevel: RiskLevelHigh, Timestamp: time.Now()}
	alert := bt.Record(sessionID, call)
	if alert == nil {
		t.Fatal("should detect brute force after 5 high-risk calls")
	}
	if alert.Pattern != "brute_force" {
		t.Fatalf("expected 'brute_force', got %q", alert.Pattern)
	}
}

func TestBehaviorTracker_DetectsLoopStuck(t *testing.T) {
	bt := NewBehaviorTracker(nil)
	sessionID := "test-session"
	// 同一工具连续调用 3 次
	for i := 0; i < 3; i++ {
		call := ToolCallRecord{Name: "shell.exec", RiskLevel: RiskLevelSafe, Timestamp: time.Now()}
		bt.Record(sessionID, call)
	}
	call := ToolCallRecord{Name: "shell.exec", RiskLevel: RiskLevelSafe, Timestamp: time.Now()}
	alert := bt.Record(sessionID, call)
	if alert == nil {
		t.Fatal("should detect loop after 3 same tool calls")
	}
}
