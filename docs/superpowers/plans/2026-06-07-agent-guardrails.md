# Agent 安全护栏与审计链路 — 实现计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 打通 Agent 模块的两条关键链路：安全校验器在工具执行前介入（P0），全链路审计记录（P1）。

**Architecture:** 在 `textReActLoop.executeTool()` 中插入 risk eval → audit → execute 流程。sync 路径通过 tool wrapper 代理实现同样的拦截。

**Tech Stack:** Go 1.25+, Eino (ReAct Agent), gin, GORM

---

### Task 1: 增强 AuditStore

**Files:**
- Modify: `internal/agent/audit/memory.go`

**Step 1: 增加审计事件类型常量和 DB 写穿透**

将 `MemoryStore` 增加 DAO 依赖用于 DB 持久化，增加事件类型常量。

`internal/agent/audit/memory.go`:

```go
package audit

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	agentDao "github.com/rizxfrog/VanPanelBackend/internal/agent/dao"
	agentmodel "github.com/rizxfrog/VanPanelBackend/internal/agent/model"
	"go.uber.org/zap"
)

// 审计事件类型
const (
	ActionReceive  = "agent.receive"
	ActionEvaluate = "tool.evaluate"
	ActionExecute  = "tool.execute"
	ActionBlocked  = "tool.blocked"
	ActionComplete = "agent.complete"
)

type Store interface {
	Append(ctx context.Context, event agentmodel.AuditEvent) (agentmodel.AuditEvent, error)
	ListBySession(ctx context.Context, sessionID string) ([]agentmodel.AuditEvent, error)
}

type MemoryStore struct {
	mu     sync.RWMutex
	events []agentmodel.AuditEvent
	limit  int
	dao    agentDao.AgentDAO
	logger *zap.Logger
}

func NewMemoryStore(dao agentDao.AgentDAO, logger *zap.Logger) *MemoryStore {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &MemoryStore{limit: 2000, dao: dao, logger: logger}
}

func (s *MemoryStore) Append(ctx context.Context, event agentmodel.AuditEvent) (agentmodel.AuditEvent, error) {
	if event.ID == "" {
		event.ID = uuid.NewString()
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now()
	}

	s.mu.Lock()
	s.events = append(s.events, event)
	if s.limit > 0 && len(s.events) > s.limit {
		s.events = s.events[len(s.events)-s.limit:]
	}
	s.mu.Unlock()

	// Write-through 到数据库
	if s.dao != nil {
		if err := s.dao.CreateAuditEvent(ctx, &event); err != nil {
			s.logger.Warn("audit db write failed", zap.Error(err))
		}
	}

	return event, nil
}

func (s *MemoryStore) ListBySession(ctx context.Context, sessionID string) ([]agentmodel.AuditEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]agentmodel.AuditEvent, 0)
	for _, event := range s.events {
		if event.SessionID == sessionID {
			out = append(out, event)
		}
	}
	return out, nil
}
```

**Step 2: 验证编译**

```bash
cd d:/Repositories/MyRepository/VanPanel/VanPanelBackend && go build ./...
```

**Step 3: Commit**

```bash
git add internal/agent/audit/memory.go
git commit -m "feat(agent): add audit event constants and db write-through"
```

---

### Task 2: 修改 textReActLoop — 插入风险拦截

**Files:**
- Modify: `internal/agent/service/text_react.go`
- Modify: `internal/agent/service/service.go`

**Step 1: textReActLoop 增加 risk/audit 回调**

在 `textReActLoop` 结构体中增加 `riskEval` 和 `auditFn` 字段，修改 `newTextReActLoop` 和 `executeTool`。

`internal/agent/service/text_react.go` 修改 `textReActLoop` 结构体和相关方法:

```go
type textReActLoop struct {
	chatModel model.ChatModel
	tools     []tool.BaseTool
	maxStep   int
	logger    *zap.Logger
	riskEval  *risk.Evaluator                                    // 新增
	sessionID string                                              // 新增: 用于审计
	userID    uint                                                // 新增: 用于审计
	username  string                                              // 新增: 用于审计
	auditFn   func(ctx context.Context, action, toolName, reason string, riskLevel agentmodel.RiskLevel, allowed bool, args string, result string) // 新增
}

func newTextReActLoop(chatModel model.ChatModel, tools []tool.BaseTool, maxStep int, logger *zap.Logger) *textReActLoop {
	if maxStep <= 0 {
		maxStep = 10
	}
	return &textReActLoop{chatModel: chatModel, tools: tools, maxStep: maxStep, logger: logger}
}

// withGuard 注入安全护栏和审计回调
func (l *textReActLoop) withGuard(riskEval *risk.Evaluator, sessionID string, userID uint, username string, auditFn func(ctx context.Context, action, toolName, reason string, riskLevel agentmodel.RiskLevel, allowed bool, args string, result string)) *textReActLoop {
	l.riskEval = riskEval
	l.sessionID = sessionID
	l.userID = userID
	l.username = username
	l.auditFn = auditFn
	return l
}
```

修改 `executeTool` 方法，在工具执行前加入风险评估:

```go
func (l *textReActLoop) executeTool(ctx context.Context, call toolCallMatch) string {
	t, found := findTool(l.tools, call.Name)
	if !found {
		msg := fmt.Sprintf("工具 %s 不存在，请检查工具名称", call.Name)
		l.audit(ctx, ActionBlocked, call.Name, "工具不存在", agentmodel.RiskSafe, false, call.ArgsJSON, msg)
		return msg
	}

	// 安全校验
	if l.riskEval != nil {
		evalResult := l.riskEval.Evaluate(call.Name, call.ArgsJSON)

		// 记录评估事件
		l.audit(ctx, ActionEvaluate, call.Name, evalResult.Reason, agentmodel.RiskLevel(evalResult.Level), !evalResult.Blocked, call.ArgsJSON, "")

		if evalResult.Blocked {
			blockedMsg := fmt.Sprintf("[安全拦截] 操作被安全策略阻止\n原因: %s\n工具: %s\n建议: 请尝试更安全的替代方案",
				evalResult.Reason, call.Name)
			l.audit(ctx, ActionBlocked, call.Name, evalResult.Reason, agentmodel.RiskLevel(evalResult.Level), false, call.ArgsJSON, blockedMsg)
			return blockedMsg
		}
	}

	// 执行工具
	result, err := t.InvokableRun(ctx, call.ArgsJSON)
	if err != nil {
		errMsg := fmt.Sprintf("工具 %s 执行失败: %v", call.Name, err)
		l.audit(ctx, ActionExecute, call.Name, "", agentmodel.RiskSafe, true, call.ArgsJSON, errMsg)
		return errMsg
	}

	l.audit(ctx, ActionExecute, call.Name, "", agentmodel.RiskSafe, true, call.ArgsJSON, truncateString(result, 2000))
	return result
}

// audit 审计记录辅助方法
func (l *textReActLoop) audit(ctx context.Context, action, toolName, reason string, riskLevel agentmodel.RiskLevel, allowed bool, args string, result string) {
	if l.auditFn == nil {
		return
	}
	l.auditFn(ctx, action, toolName, reason, riskLevel, allowed, args, result)
}
```

需要在 `text_react.go` 顶部增加 import:

```go
import (
	// ... 现有 imports ...
	agentmodel "github.com/rizxfrog/VanPanelBackend/internal/agent/model"
	"github.com/rizxfrog/VanPanelBackend/internal/agent/risk"
)
```

**Step 2: head - 验证 import 路径正确**

```bash
head -10 d:/Repositories/MyRepository/VanPanel/VanPanelBackend/internal/agent/service/text_react.go
grep "agentmodel\|internal/agent/risk" d:/Repositories/MyRepository/VanPanel/VanPanelBackend/internal/agent/service/text_react.go
```

**Step 3: 验证编译**

```bash
go build ./internal/agent/service/...
```

---

### Task 3: AgentService 集成 — Query() 的审计

**Files:**
- Modify: `internal/agent/service/service.go`

**Step 1: agentService 增加 auditStore 字段**

修改 `agentService` 结构体和 `NewAgentService`:

```go
import (
	// ... 现有 imports ...
	agentaudit "github.com/rizxfrog/VanPanelBackend/internal/agent/audit"
	agentmodel "github.com/rizxfrog/VanPanelBackend/internal/agent/model"
)

type agentService struct {
	dao       dao.AgentDAO
	toolMgr   *manager.ToolManager
	riskEval  *risk.Evaluator
	auditStore agentaudit.Store       // 新增
	cfg       *Config
	logger    *zap.Logger
}

func NewAgentService(
	dao dao.AgentDAO,
	toolMgr *manager.ToolManager,
	riskEval *risk.Evaluator,
	auditStore agentaudit.Store,     // 新增参数
	cfg *Config,
	logger *zap.Logger,
) AgentService {
	return &agentService{
		dao:       dao,
		toolMgr:   toolMgr,
		riskEval:  riskEval,
		auditStore: auditStore,
		cfg:       cfg,
		logger:    logger,
	}
}
```

**Step 2: 增加 auditable 辅助方法**

```go
// auditEvent creates and stores an audit event.
func (s *agentService) auditEvent(ctx context.Context, action, toolName, reason string, riskLevel agentmodel.RiskLevel, allowed bool, args string, result string, sessionID string, userID uint, username string) {
	if s.auditStore == nil {
		return
	}
	metadata := make(map[string]interface{})
	if args != "" {
		metadata["args"] = args
	}
	if result != "" {
		metadata["result"] = result
	}
	event := agentmodel.AuditEvent{
		SessionID: sessionID,
		UserID:    userID,
		Username:  username,
		Action:    action,
		ToolName:  toolName,
		Risk:      riskLevel,
		Allowed:   allowed,
		Reason:    reason,
		Metadata:  metadata,
	}
	if _, err := s.auditStore.Append(ctx, event); err != nil {
		s.logger.Warn("audit append failed", zap.Error(err))
	}
}
```

**Step 3: Query() 增加审计 — 接收事件**

在 `Query()` 方法中，保存用户消息后增加:

```go
// 审计: 接收用户消息
s.auditEvent(ctx, agentaudit.ActionReceive, "", "", "", true, "", req.Question, req.SessionID, userID, "")
```

**Step 4: Query() 增加审计 — 完成事件**

在 `Query()` 返回前增加:

```go
// 审计: 对话完成
s.auditEvent(ctx, agentaudit.ActionComplete, "", "", "", true, "", truncateString(answer, 2000), req.SessionID, userID, "")
```

**Step 5: 验证编译**

```bash
go build ./...
```

遇到 import 相关问题需要检查 `agentmodel` import 是否已有。

**Step 6: Commit**

```bash
git add internal/agent/service/service.go internal/agent/service/text_react.go
git commit -m "feat(agent): wire risk evaluator and audit chain into Query and QueryStream"
```

---

### Task 4: 流式路径 (QueryStream) — 注入风险拦截和审计

**Files:**
- Modify: `internal/agent/service/service.go` (QueryStream 方法)

**Step 1: QueryStream 增加审计 — 接收事件**

在 `QueryStream()` 保存用户消息后增加:

```go
// 审计: 接收用户消息
s.auditEvent(ctx, agentaudit.ActionReceive, "", "", "", true, "", req.Question, req.SessionID, userID, "")
```

**Step 2: 注入 riskEval 和 auditFn 到 textReActLoop**

在创建 `textReActLoop` 后通过 `withGuard()` 注入:

```go
// 使用 textReActLoop 流式执行：ChatModel 直接流式输出 + 文本格式工具调用
tools := s.toolMgr.GetAllTools(ctx)
loop := newTextReActLoop(chatModel, tools, 10, s.logger)
loop.withGuard(s.riskEval, req.SessionID, uint(userID), "", s.auditEventCallback) // 新增
```

需要在 service.go 增加回调方法:

```go
// auditEventCallback returns a callback function for textReActLoop audit.
func (s *agentService) auditEventCallback(ctx context.Context, action, toolName, reason string, riskLevel agentmodel.RiskLevel, allowed bool, args string, result string) {
	// sessionID/userID/username already captured by closure in withGuard
	// Find the loop's fields — actually we pass them differently
}
```

Actually, the callback signature needs `sessionID`, `userID`, `username`. Let me adjust.

`withGuard` captures these via closure:

```go
func (l *textReActLoop) withGuard(riskEval *risk.Evaluator, sessionID string, userID uint, username string, auditFn func(ctx context.Context, action, toolName, reason string, riskLevel agentmodel.RiskLevel, allowed bool, args string, result string)) *textReActLoop {
```

And in `QueryStream()`:

```go
loop := newTextReActLoop(chatModel, tools, 10, s.logger)
loop.withGuard(s.riskEval, req.SessionID, uint(userID), "",
	func(ctx context.Context, action, toolName, reason string, riskLevel agentmodel.RiskLevel, allowed bool, args string, result string) {
		s.auditEvent(ctx, action, toolName, reason, riskLevel, allowed, args, result, req.SessionID, userID, "")
	},
)
```

**Step 3: QueryStream 增加审计 — 完成事件**

在 `QueryStream()` 发送 done 事件后、return 前增加:

```go
// 审计: 对话完成
s.auditEvent(ctx, agentaudit.ActionComplete, "", "", "", true, "", truncateString(finalContent, 2000), req.SessionID, userID, "")
```

**Step 4: 验证编译**

```bash
go build ./...
```

**Step 5: Commit**

```bash
git add internal/agent/service/service.go
git commit -m "feat(agent): wire risk evaluator and audit into QueryStream"
```

---

### Task 5: 同步路径 (Query) — Tool Wrapper 代理

**Files:**
- Create: `internal/agent/service/safe_tool.go`

**Step 1: 创建 safe tool wrapper**

`internal/agent/service/safe_tool.go`:

```go
package service

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	agentmodel "github.com/rizxfrog/VanPanelBackend/internal/agent/model"
	"github.com/rizxfrog/VanPanelBackend/internal/agent/risk"
)

// safeTool 包装 InvokableTool，在执行前进行风险校验和审计
type safeTool struct {
	inner    tool.InvokableTool
	riskEval *risk.Evaluator
	auditFn  func(ctx context.Context, action, toolName, reason string, riskLevel agentmodel.RiskLevel, allowed bool, args string, result string)
	info     *tool.Info
}

func wrapTool(t tool.BaseTool, riskEval *risk.Evaluator, auditFn func(ctx context.Context, action, toolName, reason string, riskLevel agentmodel.RiskLevel, allowed bool, args string, result string)) (tool.BaseTool, error) {
	it, ok := t.(tool.InvokableTool)
	if !ok {
		return t, nil
	}
	info, err := it.Info(context.Background())
	if err != nil {
		return t, nil
	}
	return &safeTool{inner: it, riskEval: riskEval, auditFn: auditFn, info: &info}, nil
}

func (st *safeTool) Info(ctx context.Context) (*tool.Info, error) {
	return st.inner.Info(ctx)
}

func (st *safeTool) InvokableRun(ctx context.Context, argsInJSON string, opts ...tool.Option) (string, error) {
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
			return blockedMsg, nil
		}
	}

	result, err := st.inner.InvokableRun(ctx, argsInJSON, opts...)
	if err != nil {
		return "", err
	}

	if st.auditFn != nil {
		st.auditFn(ctx, "tool.execute", st.info.Name, "",
			agentmodel.RiskSafe, true, argsInJSON, truncateString(result, 2000))
	}

	return result, nil
}
```

**Step 2: Query() 包装工具**

在 `Query()` 创建 react.NewAgent 之前，包装所有工具:

```go
// 包装工具：注入风险拦截和审计
tools := s.toolMgr.GetAllTools(ctx)
safeTools := make([]tool.BaseTool, 0, len(tools))
for _, t := range tools {
	wt, err := wrapTool(t, s.riskEval, func(ctx context.Context, action, toolName, reason string, riskLevel agentmodel.RiskLevel, allowed bool, args string, result string) {
		s.auditEvent(ctx, action, toolName, reason, riskLevel, allowed, args, result, req.SessionID, userID, "")
	})
	if err != nil {
		s.logger.Warn("wrap tool failed, using original", zap.Error(err))
		safeTools = append(safeTools, t)
		continue
	}
	safeTools = append(safeTools, wt)
}
```

然后将 `safeTools` 传给 `react.NewAgent`:

```go
agent, err := react.NewAgent(ctx, &react.AgentConfig{
	ToolCallingModel: chatModel,
	ToolsConfig:      compose.ToolsNodeConfig{Tools: safeTools},
	MaxStep:          10,
	MessageModifier:  react.NewPersonaModifier(personaPrompt),
})
```

**Step 3: 验证编译**

```bash
go build ./...
```

---

### Task 6: 更新 DI 连线

**Files:**
- Modify: `pkg/di/agent.go`

**Step 1: ProvideAgentService 增加 auditStore 参数**

```go
import (
	// ... 现有 imports ...
	agentAudit "github.com/rizxfrog/VanPanelBackend/internal/agent/audit"
)

func ProvideAgentService(
	dao agentDao.AgentDAO,
	toolMgr *agentToolManager.ToolManager,
	riskEval *agentRisk.Evaluator,
	auditStore *agentAudit.MemoryStore,  // 新增
	cfg *AgentConfig,
	l *zap.Logger,
) agentService.AgentService {
	svcCfg := &agentService.Config{
		LLM: agentService.LLMConfig{
			Provider:    cfg.LLM.Provider,
			BaseURL:     cfg.LLM.BaseURL,
			APIKey:      cfg.LLM.APIKey,
			Model:       cfg.LLM.Model,
			Temperature: cfg.LLM.Temperature,
			MaxTokens:   cfg.LLM.MaxTokens,
		},
		MaxHistory: cfg.MaxHistory,
	}
	return agentService.NewAgentService(dao, toolMgr, riskEval, auditStore, svcCfg, l)
}
```

**Step 2: 创建 ProvideAgentAuditStore**

```go
func ProvideAgentAuditStore(dao agentDao.AgentDAO, l *zap.Logger) *agentAudit.MemoryStore {
	return agentAudit.NewMemoryStore(dao, l)
}
```

**Step 3: 在 wire.go 的 AgentSet 中注册**

```go
var AgentSet = wire.NewSet(
	ProvideAgentConfig,
	ProvideAgentRiskConfig,
	ProvideAgentHubConfig,
	ProvideAgentToolManager,
	ProvideAgentRiskEvaluator,
	ProvideAgentAuditStore,  // 新增
	ProvideAgentService,
	ProvideHubService,
)
```

**Step 4: 重新生成 Wire 代码**

```bash
~/go/bin/wire ./pkg/di/
```

**Step 5: 验证编译**

```bash
go build ./...
```

**Step 6: Commit**

```bash
git add pkg/di/agent.go pkg/di/wire.go pkg/di/wire_gen.go internal/agent/service/safe_tool.go
git commit -m "feat(agent): wire audit store into DI and regenerate"
```

---

### Task 7: 最终验证和清理

**Step 1: 全量编译**

```bash
go build ./...
```

**Step 2: 检查审计事件写入**

手动启动服务，发送一条消息到 Agent，检查日志:

```bash
# 启动服务
go run main.go &

# 发送请求
curl -X POST http://localhost:8889/api/system/agent/query \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"question": "df -h", "session_id": ""}'
```

检查审计日志目录（GORM DB 中的 `agent_audit_events` 表）。

**Step 3: Go test**

```bash
go test ./internal/agent/... -v -count=1
```

**Step 4: Commit**

```bash
git add -A
git commit -m "feat(agent): complete risk evaluator and audit chain integration"
```
