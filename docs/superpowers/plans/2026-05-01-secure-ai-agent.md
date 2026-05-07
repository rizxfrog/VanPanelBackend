# Secure AI Agent Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first secure AI operations Agent MVP: diagnosis, tool planning, Risk Guard, audit timeline, and user-approved low-risk actions.

**Architecture:** VanPanelBackend is the trusted control plane for authentication, risk checks, approvals, audit, and controlled tool execution. VanAgentByPy is the AI planning plane for intent classification, tool routing, and response synthesis. VanPanelWebUI is the assistant interface and approval console; it talks only to VanPanelBackend.

**Tech Stack:** Go, Gin, Wire, in-memory MVP services, Python FastAPI/Pydantic, Vue 3, Ant Design Vue, Vben request client, Vitest, pytest.

---

## Scope Note

This plan implements the MVP from the approved spec, not the full autonomous Agent platform.

Included:

- Structured agent sessions.
- Backend Agent Gateway.
- Backend Risk Guard.
- Backend audit event timeline.
- Backend approval lifecycle.
- Backend controlled tool registry with safe read-only tools and low-risk action stubs.
- Python `/api/v1/agent/plan` and `/api/v1/agent/respond`.
- WebUI `/system/agent` assistant page with timeline and approvals.

Deferred:

- Persistent DB tables for audit and approvals.
- Multi-agent parallel execution.
- Arbitrary shell execution.
- High-risk remediation.
- Full MCP protocol replacement.
- Fine-tuned models.

---

## Repository Worktrees

Use isolated branches before implementation:

```powershell
git -C D:\Repositories\MyRepository\VanPanel\VanPanelBackend worktree add .worktrees/secure-ai-agent -b feature/secure-ai-agent
git -C D:\Repositories\MyRepository\VanPanel\VanPanelWebUI worktree add .worktrees/secure-ai-agent -b feature/secure-ai-agent
```

For `VanAgentByPy`, there is no existing `.worktrees` directory. Before implementation, create one and add it to `.gitignore` if needed:

```powershell
git -C D:\Repositories\MyRepository\VanPanel\VanAgentByPy check-ignore -q .worktrees
```

If not ignored, add this exact line to `VanAgentByPy\.gitignore` and commit it:

```text
.worktrees/
```

Then create:

```powershell
git -C D:\Repositories\MyRepository\VanPanel\VanAgentByPy worktree add .worktrees/secure-ai-agent -b feature/secure-ai-agent
```

---

## File Structure

Backend:

- Create: `internal/agent/model/types.go` for request, response, event, approval, tool, and risk models.
- Create: `internal/agent/risk/guard.go` for Risk Guard rules.
- Create: `internal/agent/risk/guard_test.go` for blocked and allowed actions.
- Create: `internal/agent/audit/memory.go` for MVP audit store.
- Create: `internal/agent/audit/memory_test.go` for ordered events.
- Create: `internal/agent/tools/registry.go` for controlled tool definitions.
- Create: `internal/agent/tools/registry_test.go` for schema/risk behavior.
- Create: `internal/agent/service/service.go` for session, query, approval, and tool execution orchestration.
- Create: `internal/agent/service/service_test.go` for gateway flow and approval flow.
- Create: `internal/agent/api/handler.go` for `/api/system/agent/*` APIs.
- Create: `internal/agent/api/handler_test.go` for route and response behavior.
- Modify: `pkg/di/wire.go`, `pkg/di/wire_gen.go`, `pkg/di/web.go` to register the Agent handler.

VanAgentByPy:

- Create: `app/models/agent_models.py` for structured plan/respond DTOs.
- Create: `app/core/agents/intent_classifier.py` for rule-first intent classification.
- Create: `app/core/agents/tool_router.py` for tool planning.
- Create: `app/core/agents/orchestrator.py` for plan/respond lifecycle.
- Create: `app/api/routes/agent.py` for `/api/v1/agent/plan` and `/api/v1/agent/respond`.
- Modify: `app/api/routes/__init__.py` to register the agent router.
- Test: `tests/test_secure_agent.py`.

WebUI:

- Create: `apps/web-antd/src/api/core/system/agent.ts`.
- Create: `apps/web-antd/src/api/core/system/agent.test.ts`.
- Create: `apps/web-antd/src/views/system/AgentAssistant.vue`.
- Create: `apps/web-antd/src/views/system/agent-assistant.css`.
- Modify: `apps/web-antd/src/router/routes/modules/files.ts` to add `/system/agent`.

---

### Task 1: Backend Models And Risk Guard

**Files:**
- Create: `internal/agent/model/types.go`
- Create: `internal/agent/risk/guard.go`
- Create: `internal/agent/risk/guard_test.go`

- [ ] **Step 1: Write failing Risk Guard tests**

Create `internal/agent/risk/guard_test.go`:

```go
package risk

import (
	"testing"

	agentmodel "github.com/GoSimplicity/AI-CloudOps/internal/agent/model"
)

func TestGuardBlocksHighRiskPatterns(t *testing.T) {
	guard := NewGuard()
	cases := []agentmodel.ToolCall{
		{Tool: "terminal.suggest", Parameters: map[string]any{"command": "rm -rf /"}},
		{Tool: "file.move_to_trash", Parameters: map[string]any{"path": "/etc/passwd"}},
		{Tool: "service.restart", Parameters: map[string]any{"service": "firewalld"}},
	}
	for _, tc := range cases {
		decision := guard.Evaluate(agentmodel.RiskRequest{
			UserID: "u1",
			Intent: "low_risk_action",
			ToolCall: tc,
		})
		if decision.Allowed {
			t.Fatalf("expected blocked decision for %#v", tc)
		}
		if decision.RiskLevel != agentmodel.RiskHigh {
			t.Fatalf("expected high risk for %#v, got %s", tc, decision.RiskLevel)
		}
	}
}

func TestGuardAllowsReadOnlyTools(t *testing.T) {
	guard := NewGuard()
	decision := guard.Evaluate(agentmodel.RiskRequest{
		UserID: "u1",
		Intent: "diagnosis",
		ToolCall: agentmodel.ToolCall{
			Tool: "disk.analyze",
			Parameters: map[string]any{"path": "/var/log"},
		},
	})
	if !decision.Allowed {
		t.Fatalf("expected allowed decision: %#v", decision)
	}
	if decision.RequiresApproval {
		t.Fatalf("read-only tool should not require approval: %#v", decision)
	}
}

func TestGuardRequiresApprovalForLowRiskTools(t *testing.T) {
	guard := NewGuard()
	decision := guard.Evaluate(agentmodel.RiskRequest{
		UserID: "u1",
		Intent: "low_risk_action",
		ToolCall: agentmodel.ToolCall{
			Tool: "container.restart",
			Parameters: map[string]any{"containerId": "nginx"},
		},
	})
	if !decision.Allowed || !decision.RequiresApproval {
		t.Fatalf("expected approval decision: %#v", decision)
	}
	if decision.RiskLevel != agentmodel.RiskLow {
		t.Fatalf("expected low risk, got %s", decision.RiskLevel)
	}
}
```

- [ ] **Step 2: Run test to verify failure**

Run:

```powershell
go test ./internal/agent/risk -v
```

Expected: FAIL because packages do not exist.

- [ ] **Step 3: Create model types**

Create `internal/agent/model/types.go`:

```go
package model

import "time"

type RiskLevel string

const (
	RiskSafe RiskLevel = "safe"
	RiskLow  RiskLevel = "low"
	RiskHigh RiskLevel = "high"
)

type Session struct {
	ID        string    `json:"id"`
	UserID    string    `json:"userId"`
	Username  string    `json:"username"`
	CreatedAt time.Time `json:"createdAt"`
}

type QueryRequest struct {
	SessionID string `json:"sessionId"`
	Message   string `json:"message"`
}

type QueryResponse struct {
	RequestID string      `json:"requestId"`
	Answer    string      `json:"answer"`
	Intent    string      `json:"intent"`
	RiskLevel RiskLevel   `json:"riskLevel"`
	Events    []AuditEvent `json:"events"`
	Approvals []Approval   `json:"approvals"`
}

type ToolCall struct {
	Tool       string         `json:"tool"`
	Parameters map[string]any `json:"parameters"`
	Reason     string         `json:"reason"`
}

type ToolPlan struct {
	Intent    string     `json:"intent"`
	RiskLevel RiskLevel  `json:"riskLevel"`
	ToolCalls []ToolCall `json:"toolCalls"`
}

type RiskRequest struct {
	UserID   string   `json:"userId"`
	Intent   string   `json:"intent"`
	ToolCall ToolCall `json:"toolCall"`
}

type RiskDecision struct {
	Allowed          bool      `json:"allowed"`
	RequiresApproval bool      `json:"requiresApproval"`
	RiskLevel        RiskLevel `json:"riskLevel"`
	Reason           string    `json:"reason"`
}

type Approval struct {
	ID         string         `json:"id"`
	RequestID  string         `json:"requestId"`
	UserID     string         `json:"userId"`
	ToolCall   ToolCall       `json:"toolCall"`
	RiskLevel  RiskLevel      `json:"riskLevel"`
	Status     string         `json:"status"`
	Reason     string         `json:"reason"`
	Parameters map[string]any `json:"parameters"`
	ExpiresAt  time.Time      `json:"expiresAt"`
	CreatedAt  time.Time      `json:"createdAt"`
}

type AuditEvent struct {
	ID        string    `json:"id"`
	RequestID string    `json:"requestId"`
	SessionID string    `json:"sessionId"`
	UserID    string    `json:"userId"`
	Stage     string    `json:"stage"`
	Message   string    `json:"message"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

type AgentPlanRequest struct {
	RequestID string `json:"request_id"`
	SessionID string `json:"session_id"`
	UserID    string `json:"user_id"`
	Message   string `json:"message"`
}

type AgentPlanResponse struct {
	Intent     string     `json:"intent"`
	Confidence float64   `json:"confidence"`
	Reason     string    `json:"reason"`
	RiskLevel  RiskLevel `json:"riskLevel"`
	ToolCalls  []ToolCall `json:"toolCalls"`
}

type AgentRespondRequest struct {
	RequestID string       `json:"request_id"`
	SessionID string       `json:"session_id"`
	Message   string       `json:"message"`
	Intent    string       `json:"intent"`
	ToolResults []ToolResult `json:"toolResults"`
	Approvals []Approval    `json:"approvals"`
}

type ToolResult struct {
	Tool   string `json:"tool"`
	Status string `json:"status"`
	Result any    `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

type AgentRespondResponse struct {
	Answer string `json:"answer"`
}
```

- [ ] **Step 4: Implement Risk Guard**

Create `internal/agent/risk/guard.go`:

```go
package risk

import (
	"fmt"
	"strings"

	agentmodel "github.com/GoSimplicity/AI-CloudOps/internal/agent/model"
)

type Guard struct {
	readOnlyTools map[string]bool
	lowRiskTools map[string]bool
	blockedTokens []string
	protectedPaths []string
	protectedServices map[string]bool
}

func NewGuard() *Guard {
	return &Guard{
		readOnlyTools: map[string]bool{
			"system.inspect": true,
			"process.list": true,
			"disk.analyze": true,
			"log.query": true,
			"file.scan": true,
			"terminal.suggest": true,
			"container.inspect": true,
			"prometheus.query": true,
		},
		lowRiskTools: map[string]bool{
			"log.compress": true,
			"log.truncate": true,
			"container.restart": true,
			"service.restart": true,
			"file.move_to_trash": true,
		},
		blockedTokens: []string{
			"rm -rf", "mkfs", "fdisk", "chmod 777", "shutdown firewall", "iptables -F",
			"systemctl stop firewalld", "sudo ", "su -", ":(){", ">/etc/passwd",
		},
		protectedPaths: []string{"/etc/passwd", "/etc/shadow", "/etc/sudoers", "/var/lib/mysql", "/var/lib/postgresql"},
		protectedServices: map[string]bool{"firewalld": true, "sshd": true, "auditd": true},
	}
}

func (g *Guard) Evaluate(req agentmodel.RiskRequest) agentmodel.RiskDecision {
	text := strings.ToLower(req.ToolCall.Tool + " " + fmt.Sprint(req.ToolCall.Parameters))
	for _, token := range g.blockedTokens {
		if strings.Contains(text, strings.ToLower(token)) {
			return agentmodel.RiskDecision{Allowed: false, RiskLevel: agentmodel.RiskHigh, Reason: "blocked dangerous token: " + token}
		}
	}
	for _, path := range g.protectedPaths {
		if strings.Contains(text, strings.ToLower(path)) {
			return agentmodel.RiskDecision{Allowed: false, RiskLevel: agentmodel.RiskHigh, Reason: "protected path is not allowed: " + path}
		}
	}
	if service, ok := req.ToolCall.Parameters["service"].(string); ok && g.protectedServices[strings.ToLower(service)] {
		return agentmodel.RiskDecision{Allowed: false, RiskLevel: agentmodel.RiskHigh, Reason: "protected service is not allowed: " + service}
	}
	if g.readOnlyTools[req.ToolCall.Tool] {
		return agentmodel.RiskDecision{Allowed: true, RiskLevel: agentmodel.RiskSafe, Reason: "read-only tool"}
	}
	if g.lowRiskTools[req.ToolCall.Tool] {
		return agentmodel.RiskDecision{Allowed: true, RequiresApproval: true, RiskLevel: agentmodel.RiskLow, Reason: "low-risk action requires approval"}
	}
	return agentmodel.RiskDecision{Allowed: false, RiskLevel: agentmodel.RiskHigh, Reason: "unknown or unregistered tool"}
}
```

- [ ] **Step 5: Run tests**

Run:

```powershell
go test ./internal/agent/risk -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```powershell
git add internal/agent/model/types.go internal/agent/risk/guard.go internal/agent/risk/guard_test.go
git commit -m "feat(agent): add risk guard"
```

---

### Task 2: Backend Audit And Approval Stores

**Files:**
- Create: `internal/agent/audit/memory.go`
- Create: `internal/agent/audit/memory_test.go`
- Create: `internal/agent/service/approval_store.go`
- Test: `internal/agent/service/approval_store_test.go`

- [ ] **Step 1: Write audit store tests**

Create `internal/agent/audit/memory_test.go`:

```go
package audit

import (
	"testing"

	agentmodel "github.com/GoSimplicity/AI-CloudOps/internal/agent/model"
)

func TestMemoryStoreAppendsEventsInOrder(t *testing.T) {
	store := NewMemoryStore()
	store.Append(agentmodel.AuditEvent{RequestID: "r1", Stage: "received", Message: "request"})
	store.Append(agentmodel.AuditEvent{RequestID: "r1", Stage: "planned", Message: "plan"})
	events := store.List("r1")
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Stage != "received" || events[1].Stage != "planned" {
		t.Fatalf("events not ordered: %#v", events)
	}
	if events[0].ID == "" || events[0].CreatedAt.IsZero() {
		t.Fatalf("event metadata was not assigned: %#v", events[0])
	}
}
```

- [ ] **Step 2: Write approval store tests**

Create `internal/agent/service/approval_store_test.go`:

```go
package service

import (
	"testing"
	"time"

	agentmodel "github.com/GoSimplicity/AI-CloudOps/internal/agent/model"
)

func TestApprovalStoreConfirmsExactApproval(t *testing.T) {
	store := NewApprovalStore()
	approval := store.Create("r1", "u1", agentmodel.ToolCall{
		Tool: "container.restart",
		Parameters: map[string]any{"containerId": "nginx"},
	}, "restart unhealthy container", time.Minute)
	confirmed, err := store.Confirm(approval.ID, "u1")
	if err != nil {
		t.Fatalf("confirm failed: %v", err)
	}
	if confirmed.Status != "confirmed" {
		t.Fatalf("unexpected status: %s", confirmed.Status)
	}
}

func TestApprovalStoreRejectsWrongUserAndExpired(t *testing.T) {
	store := NewApprovalStore()
	approval := store.Create("r1", "u1", agentmodel.ToolCall{Tool: "container.restart"}, "reason", -time.Minute)
	if _, err := store.Confirm(approval.ID, "u2"); err == nil {
		t.Fatal("expected wrong user error")
	}
	if _, err := store.Confirm(approval.ID, "u1"); err == nil {
		t.Fatal("expected expired approval error")
	}
}
```

- [ ] **Step 3: Run tests to verify failure**

Run:

```powershell
go test ./internal/agent/audit ./internal/agent/service -run "TestMemoryStore|TestApprovalStore" -v
```

Expected: FAIL because stores do not exist.

- [ ] **Step 4: Implement audit memory store**

Create `internal/agent/audit/memory.go`:

```go
package audit

import (
	"sync"
	"time"

	agentmodel "github.com/GoSimplicity/AI-CloudOps/internal/agent/model"
	"github.com/google/uuid"
)

type MemoryStore struct {
	mu sync.Mutex
	events map[string][]agentmodel.AuditEvent
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{events: map[string][]agentmodel.AuditEvent{}}
}

func (s *MemoryStore) Append(event agentmodel.AuditEvent) agentmodel.AuditEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	if event.ID == "" {
		event.ID = uuid.NewString()
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now()
	}
	s.events[event.RequestID] = append(s.events[event.RequestID], event)
	return event
}

func (s *MemoryStore) List(requestID string) []agentmodel.AuditEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := append([]agentmodel.AuditEvent(nil), s.events[requestID]...)
	return result
}
```

- [ ] **Step 5: Implement approval store**

Create `internal/agent/service/approval_store.go`:

```go
package service

import (
	"fmt"
	"sync"
	"time"

	agentmodel "github.com/GoSimplicity/AI-CloudOps/internal/agent/model"
	"github.com/google/uuid"
)

type ApprovalStore struct {
	mu sync.Mutex
	items map[string]agentmodel.Approval
}

func NewApprovalStore() *ApprovalStore {
	return &ApprovalStore{items: map[string]agentmodel.Approval{}}
}

func (s *ApprovalStore) Create(requestID string, userID string, call agentmodel.ToolCall, reason string, ttl time.Duration) agentmodel.Approval {
	s.mu.Lock()
	defer s.mu.Unlock()
	approval := agentmodel.Approval{
		ID: uuid.NewString(),
		RequestID: requestID,
		UserID: userID,
		ToolCall: call,
		RiskLevel: agentmodel.RiskLow,
		Status: "pending",
		Reason: reason,
		Parameters: call.Parameters,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(ttl),
	}
	s.items[approval.ID] = approval
	return approval
}

func (s *ApprovalStore) Confirm(id string, userID string) (agentmodel.Approval, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[id]
	if !ok {
		return agentmodel.Approval{}, fmt.Errorf("approval not found")
	}
	if item.UserID != userID {
		return agentmodel.Approval{}, fmt.Errorf("approval belongs to another user")
	}
	if time.Now().After(item.ExpiresAt) {
		return agentmodel.Approval{}, fmt.Errorf("approval expired")
	}
	if item.Status != "pending" {
		return agentmodel.Approval{}, fmt.Errorf("approval is not pending")
	}
	item.Status = "confirmed"
	s.items[id] = item
	return item, nil
}

func (s *ApprovalStore) Reject(id string, userID string) (agentmodel.Approval, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[id]
	if !ok {
		return agentmodel.Approval{}, fmt.Errorf("approval not found")
	}
	if item.UserID != userID {
		return agentmodel.Approval{}, fmt.Errorf("approval belongs to another user")
	}
	item.Status = "rejected"
	s.items[id] = item
	return item, nil
}
```

- [ ] **Step 6: Run tests**

Run:

```powershell
go test ./internal/agent/audit ./internal/agent/service -run "TestMemoryStore|TestApprovalStore" -v
```

Expected: PASS.

- [ ] **Step 7: Commit**

```powershell
git add internal/agent/audit internal/agent/service/approval_store.go internal/agent/service/approval_store_test.go
git commit -m "feat(agent): add audit and approval stores"
```

---

### Task 3: Backend Tool Registry

**Files:**
- Create: `internal/agent/tools/registry.go`
- Create: `internal/agent/tools/registry_test.go`

- [ ] **Step 1: Write tool registry tests**

Create `internal/agent/tools/registry_test.go`:

```go
package tools

import (
	"context"
	"testing"

	agentmodel "github.com/GoSimplicity/AI-CloudOps/internal/agent/model"
)

func TestRegistryExecutesReadOnlyTool(t *testing.T) {
	registry := NewRegistry()
	result, err := registry.Execute(context.Background(), agentmodel.ToolCall{Tool: "system.inspect"})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if result.Status != "success" || result.Tool != "system.inspect" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestRegistryRejectsUnknownTool(t *testing.T) {
	registry := NewRegistry()
	_, err := registry.Execute(context.Background(), agentmodel.ToolCall{Tool: "unknown.tool"})
	if err == nil {
		t.Fatal("expected unknown tool error")
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

Run:

```powershell
go test ./internal/agent/tools -v
```

Expected: FAIL because registry does not exist.

- [ ] **Step 3: Implement registry**

Create `internal/agent/tools/registry.go`:

```go
package tools

import (
	"context"
	"fmt"
	"runtime"
	"time"

	agentmodel "github.com/GoSimplicity/AI-CloudOps/internal/agent/model"
)

type Handler func(ctx context.Context, call agentmodel.ToolCall) (any, error)

type Registry struct {
	handlers map[string]Handler
}

func NewRegistry() *Registry {
	r := &Registry{handlers: map[string]Handler{}}
	r.handlers["system.inspect"] = systemInspect
	r.handlers["process.list"] = stubReadOnly("process.list")
	r.handlers["disk.analyze"] = stubReadOnly("disk.analyze")
	r.handlers["log.query"] = stubReadOnly("log.query")
	r.handlers["file.scan"] = stubReadOnly("file.scan")
	r.handlers["terminal.suggest"] = stubReadOnly("terminal.suggest")
	r.handlers["container.inspect"] = stubReadOnly("container.inspect")
	r.handlers["prometheus.query"] = stubReadOnly("prometheus.query")
	r.handlers["container.restart"] = stubLowRisk("container.restart")
	r.handlers["service.restart"] = stubLowRisk("service.restart")
	r.handlers["log.compress"] = stubLowRisk("log.compress")
	r.handlers["log.truncate"] = stubLowRisk("log.truncate")
	r.handlers["file.move_to_trash"] = stubLowRisk("file.move_to_trash")
	return r
}

func (r *Registry) Execute(ctx context.Context, call agentmodel.ToolCall) (agentmodel.ToolResult, error) {
	handler, ok := r.handlers[call.Tool]
	if !ok {
		return agentmodel.ToolResult{}, fmt.Errorf("tool not found: %s", call.Tool)
	}
	result, err := handler(ctx, call)
	if err != nil {
		return agentmodel.ToolResult{Tool: call.Tool, Status: "error", Error: err.Error()}, err
	}
	return agentmodel.ToolResult{Tool: call.Tool, Status: "success", Result: result}, nil
}

func (r *Registry) List() []string {
	names := make([]string, 0, len(r.handlers))
	for name := range r.handlers {
		names = append(names, name)
	}
	return names
}

func systemInspect(context.Context, agentmodel.ToolCall) (any, error) {
	return map[string]any{
		"os": runtime.GOOS,
		"arch": runtime.GOARCH,
		"time": time.Now().Format(time.RFC3339),
	}, nil
}

func stubReadOnly(name string) Handler {
	return func(context.Context, agentmodel.ToolCall) (any, error) {
		return map[string]any{"tool": name, "mode": "read_only", "message": "MVP stub result"}, nil
	}
}

func stubLowRisk(name string) Handler {
	return func(context.Context, agentmodel.ToolCall) (any, error) {
		return map[string]any{"tool": name, "mode": "approved_low_risk", "message": "MVP approved action stub"}, nil
	}
}
```

- [ ] **Step 4: Run tests**

Run:

```powershell
go test ./internal/agent/tools -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add internal/agent/tools
git commit -m "feat(agent): add controlled tool registry"
```

---

### Task 4: Backend Agent Service And API

**Files:**
- Create: `internal/agent/service/service.go`
- Create: `internal/agent/service/service_test.go`
- Create: `internal/agent/api/handler.go`
- Create: `internal/agent/api/handler_test.go`
- Modify: `pkg/di/wire.go`
- Modify: `pkg/di/wire_gen.go`
- Modify: `pkg/di/web.go`

- [ ] **Step 1: Write service flow test**

Create `internal/agent/service/service_test.go`:

```go
package service

import (
	"context"
	"testing"

	agentaudit "github.com/GoSimplicity/AI-CloudOps/internal/agent/audit"
	agentmodel "github.com/GoSimplicity/AI-CloudOps/internal/agent/model"
	agentrisk "github.com/GoSimplicity/AI-CloudOps/internal/agent/risk"
	agenttools "github.com/GoSimplicity/AI-CloudOps/internal/agent/tools"
)

type fakePlanner struct{}

func (fakePlanner) Plan(context.Context, agentmodel.AgentPlanRequest) (agentmodel.AgentPlanResponse, error) {
	return agentmodel.AgentPlanResponse{
		Intent: "diagnosis",
		Confidence: 0.9,
		Reason: "disk question",
		RiskLevel: agentmodel.RiskSafe,
		ToolCalls: []agentmodel.ToolCall{{Tool: "system.inspect", Reason: "inspect host"}},
	}, nil
}

func (fakePlanner) Respond(context.Context, agentmodel.AgentRespondRequest) (agentmodel.AgentRespondResponse, error) {
	return agentmodel.AgentRespondResponse{Answer: "系统状态已检查"}, nil
}

func TestServiceQueryRunsSafeToolAndAudits(t *testing.T) {
	svc := NewService(fakePlanner{}, agentrisk.NewGuard(), agentaudit.NewMemoryStore(), NewApprovalStore(), agenttools.NewRegistry())
	session := svc.CreateSession("u1", "admin")
	resp, err := svc.Query(context.Background(), "u1", agentmodel.QueryRequest{SessionID: session.ID, Message: "检查系统"})
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if resp.Answer == "" || len(resp.Events) == 0 {
		t.Fatalf("unexpected response: %#v", resp)
	}
	if len(resp.Approvals) != 0 {
		t.Fatalf("safe query should not create approvals: %#v", resp.Approvals)
	}
}
```

- [ ] **Step 2: Write API route test**

Create `internal/agent/api/handler_test.go`:

```go
package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	agentmodel "github.com/GoSimplicity/AI-CloudOps/internal/agent/model"
	"github.com/gin-gonic/gin"
)

type fakeAgentService struct{}

func (fakeAgentService) CreateSession(userID string, username string) agentmodel.Session {
	return agentmodel.Session{ID: "s1", UserID: userID, Username: username}
}
func (fakeAgentService) Query(context.Context, string, agentmodel.QueryRequest) (agentmodel.QueryResponse, error) {
	return agentmodel.QueryResponse{RequestID: "r1", Answer: "ok", Intent: "diagnosis"}
}
func (fakeAgentService) Events(string) []agentmodel.AuditEvent { return nil }
func (fakeAgentService) ConfirmApproval(context.Context, string, string) (agentmodel.QueryResponse, error) {
	return agentmodel.QueryResponse{RequestID: "r1", Answer: "approved"}, nil
}
func (fakeAgentService) RejectApproval(string, string) (agentmodel.Approval, error) {
	return agentmodel.Approval{ID: "a1", Status: "rejected"}, nil
}
func (fakeAgentService) Tools() []string { return []string{"system.inspect"} }

func TestRegisterRoutes(t *testing.T) {
	router := gin.New()
	NewAgentHandler(fakeAgentService{}).RegisterRouters(router)
	req := httptest.NewRequest(http.MethodGet, "/api/system/agent/tools", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", resp.Code)
	}
	if !strings.Contains(resp.Body.String(), "system.inspect") {
		t.Fatalf("unexpected body: %s", resp.Body.String())
	}
}
```

- [ ] **Step 3: Run tests to verify failure**

Run:

```powershell
go test ./internal/agent/service ./internal/agent/api -v
```

Expected: FAIL because service and handler do not exist.

- [ ] **Step 4: Implement service**

Create `internal/agent/service/service.go`:

```go
package service

import (
	"context"
	"fmt"
	"time"

	agentaudit "github.com/GoSimplicity/AI-CloudOps/internal/agent/audit"
	agentmodel "github.com/GoSimplicity/AI-CloudOps/internal/agent/model"
	agentrisk "github.com/GoSimplicity/AI-CloudOps/internal/agent/risk"
	agenttools "github.com/GoSimplicity/AI-CloudOps/internal/agent/tools"
	"github.com/google/uuid"
)

type Planner interface {
	Plan(context.Context, agentmodel.AgentPlanRequest) (agentmodel.AgentPlanResponse, error)
	Respond(context.Context, agentmodel.AgentRespondRequest) (agentmodel.AgentRespondResponse, error)
}

type AgentService interface {
	CreateSession(userID string, username string) agentmodel.Session
	Query(context.Context, string, agentmodel.QueryRequest) (agentmodel.QueryResponse, error)
	Events(requestID string) []agentmodel.AuditEvent
	ConfirmApproval(context.Context, string, string) (agentmodel.QueryResponse, error)
	RejectApproval(string, string) (agentmodel.Approval, error)
	Tools() []string
}

type Service struct {
	planner Planner
	guard *agentrisk.Guard
	audit *agentaudit.MemoryStore
	approvals *ApprovalStore
	tools *agenttools.Registry
	sessions map[string]agentmodel.Session
}

func NewService(planner Planner, guard *agentrisk.Guard, audit *agentaudit.MemoryStore, approvals *ApprovalStore, tools *agenttools.Registry) *Service {
	return &Service{planner: planner, guard: guard, audit: audit, approvals: approvals, tools: tools, sessions: map[string]agentmodel.Session{}}
}

func (s *Service) CreateSession(userID string, username string) agentmodel.Session {
	session := agentmodel.Session{ID: uuid.NewString(), UserID: userID, Username: username, CreatedAt: time.Now()}
	s.sessions[session.ID] = session
	return session
}

func (s *Service) Query(ctx context.Context, userID string, req agentmodel.QueryRequest) (agentmodel.QueryResponse, error) {
	requestID := uuid.NewString()
	session, ok := s.sessions[req.SessionID]
	if !ok {
		return agentmodel.QueryResponse{}, fmt.Errorf("session not found")
	}
	s.audit.Append(agentmodel.AuditEvent{RequestID: requestID, SessionID: session.ID, UserID: userID, Stage: "received", Message: req.Message})
	plan, err := s.planner.Plan(ctx, agentmodel.AgentPlanRequest{RequestID: requestID, SessionID: session.ID, UserID: userID, Message: req.Message})
	if err != nil {
		return agentmodel.QueryResponse{}, err
	}
	s.audit.Append(agentmodel.AuditEvent{RequestID: requestID, SessionID: session.ID, UserID: userID, Stage: "planned", Message: plan.Intent})
	var results []agentmodel.ToolResult
	var approvals []agentmodel.Approval
	highestRisk := agentmodel.RiskSafe
	for _, call := range plan.ToolCalls {
		decision := s.guard.Evaluate(agentmodel.RiskRequest{UserID: userID, Intent: plan.Intent, ToolCall: call})
		s.audit.Append(agentmodel.AuditEvent{RequestID: requestID, SessionID: session.ID, UserID: userID, Stage: "risk_guard", Message: decision.Reason, Metadata: map[string]any{"tool": call.Tool, "allowed": decision.Allowed}})
		if decision.RiskLevel == agentmodel.RiskHigh {
			highestRisk = agentmodel.RiskHigh
			results = append(results, agentmodel.ToolResult{Tool: call.Tool, Status: "blocked", Error: decision.Reason})
			continue
		}
		if decision.RiskLevel == agentmodel.RiskLow {
			highestRisk = agentmodel.RiskLow
		}
		if decision.RequiresApproval {
			approvals = append(approvals, s.approvals.Create(requestID, userID, call, decision.Reason, 10*time.Minute))
			continue
		}
		result, err := s.tools.Execute(ctx, call)
		if err != nil {
			results = append(results, result)
			continue
		}
		results = append(results, result)
	}
	respond, err := s.planner.Respond(ctx, agentmodel.AgentRespondRequest{RequestID: requestID, SessionID: session.ID, Message: req.Message, Intent: plan.Intent, ToolResults: results, Approvals: approvals})
	if err != nil {
		return agentmodel.QueryResponse{}, err
	}
	s.audit.Append(agentmodel.AuditEvent{RequestID: requestID, SessionID: session.ID, UserID: userID, Stage: "responded", Message: respond.Answer})
	return agentmodel.QueryResponse{RequestID: requestID, Answer: respond.Answer, Intent: plan.Intent, RiskLevel: highestRisk, Events: s.audit.List(requestID), Approvals: approvals}, nil
}

func (s *Service) Events(requestID string) []agentmodel.AuditEvent { return s.audit.List(requestID) }
func (s *Service) Tools() []string { return s.tools.List() }
func (s *Service) ConfirmApproval(ctx context.Context, approvalID string, userID string) (agentmodel.QueryResponse, error) {
	approval, err := s.approvals.Confirm(approvalID, userID)
	if err != nil {
		return agentmodel.QueryResponse{}, err
	}
	decision := s.guard.Evaluate(agentmodel.RiskRequest{UserID: userID, Intent: "low_risk_action", ToolCall: approval.ToolCall})
	if !decision.Allowed || decision.RiskLevel == agentmodel.RiskHigh {
		return agentmodel.QueryResponse{}, fmt.Errorf("approval execution blocked: %s", decision.Reason)
	}
	result, err := s.tools.Execute(ctx, approval.ToolCall)
	if err != nil {
		return agentmodel.QueryResponse{}, err
	}
	return agentmodel.QueryResponse{RequestID: approval.RequestID, Answer: "已执行审批操作", RiskLevel: agentmodel.RiskLow, Events: s.audit.List(approval.RequestID), Approvals: []agentmodel.Approval{approval}, Intent: result.Tool}, nil
}
func (s *Service) RejectApproval(id string, userID string) (agentmodel.Approval, error) {
	return s.approvals.Reject(id, userID)
}
```

- [ ] **Step 5: Implement API handler**

Create `internal/agent/api/handler.go`:

```go
package api

import (
	agentmodel "github.com/GoSimplicity/AI-CloudOps/internal/agent/model"
	agentservice "github.com/GoSimplicity/AI-CloudOps/internal/agent/service"
	"github.com/GoSimplicity/AI-CloudOps/pkg/base"
	"github.com/GoSimplicity/AI-CloudOps/pkg/jwt"
	"github.com/gin-gonic/gin"
)

type AgentHandler struct {
	service agentservice.AgentService
}

func NewAgentHandler(service agentservice.AgentService) *AgentHandler {
	return &AgentHandler{service: service}
}

func (h *AgentHandler) RegisterRouters(server *gin.Engine) {
	group := server.Group("/api/system/agent")
	group.POST("/sessions", h.CreateSession)
	group.POST("/query", h.Query)
	group.GET("/sessions/:id/events", h.Events)
	group.POST("/approvals/:id/confirm", h.ConfirmApproval)
	group.POST("/approvals/:id/reject", h.RejectApproval)
	group.GET("/tools", h.Tools)
}

func (h *AgentHandler) CreateSession(ctx *gin.Context) {
	userID, username := currentUser(ctx)
	base.SuccessWithData(ctx, h.service.CreateSession(userID, username))
}

func (h *AgentHandler) Query(ctx *gin.Context) {
	var req agentmodel.QueryRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		base.ErrorWithMessage(ctx, err.Error())
		return
	}
	userID, _ := currentUser(ctx)
	resp, err := h.service.Query(ctx, userID, req)
	if err != nil {
		base.ErrorWithMessage(ctx, err.Error())
		return
	}
	base.SuccessWithData(ctx, resp)
}

func (h *AgentHandler) Events(ctx *gin.Context) {
	base.SuccessWithData(ctx, h.service.Events(ctx.Param("id")))
}

func (h *AgentHandler) ConfirmApproval(ctx *gin.Context) {
	userID, _ := currentUser(ctx)
	resp, err := h.service.ConfirmApproval(ctx, ctx.Param("id"), userID)
	if err != nil {
		base.ErrorWithMessage(ctx, err.Error())
		return
	}
	base.SuccessWithData(ctx, resp)
}

func (h *AgentHandler) RejectApproval(ctx *gin.Context) {
	userID, _ := currentUser(ctx)
	resp, err := h.service.RejectApproval(ctx.Param("id"), userID)
	if err != nil {
		base.ErrorWithMessage(ctx, err.Error())
		return
	}
	base.SuccessWithData(ctx, resp)
}

func (h *AgentHandler) Tools(ctx *gin.Context) {
	base.SuccessWithData(ctx, h.service.Tools())
}

func currentUser(ctx *gin.Context) (string, string) {
	value, ok := ctx.Get("user")
	if !ok {
		return "anonymous", "anonymous"
	}
	claims, ok := value.(jwt.UserClaims)
	if !ok {
		return "anonymous", "anonymous"
	}
	return claims.Uid, claims.Username
}
```

- [ ] **Step 6: Wire DI**

Add imports to `pkg/di/wire.go`:

```go
agentApi "github.com/GoSimplicity/AI-CloudOps/internal/agent/api"
agentAudit "github.com/GoSimplicity/AI-CloudOps/internal/agent/audit"
agentRisk "github.com/GoSimplicity/AI-CloudOps/internal/agent/risk"
agentService "github.com/GoSimplicity/AI-CloudOps/internal/agent/service"
agentTools "github.com/GoSimplicity/AI-CloudOps/internal/agent/tools"
```

Register providers:

```go
agentApi.NewAgentHandler
agentRisk.NewGuard
agentAudit.NewMemoryStore
agentService.NewApprovalStore
agentTools.NewRegistry
ProvideAgentPlanner
ProvideAgentService
```

Create `pkg/di/agent.go`:

```go
package di

import (
	agentaudit "github.com/GoSimplicity/AI-CloudOps/internal/agent/audit"
	agentrisk "github.com/GoSimplicity/AI-CloudOps/internal/agent/risk"
	agentservice "github.com/GoSimplicity/AI-CloudOps/internal/agent/service"
	agenttools "github.com/GoSimplicity/AI-CloudOps/internal/agent/tools"
)

func ProvideAgentPlanner() agentservice.Planner {
	return agentservice.NewHTTPPlanner("http://localhost:8080")
}

func ProvideAgentService(planner agentservice.Planner, guard *agentrisk.Guard, audit *agentaudit.MemoryStore, approvals *agentservice.ApprovalStore, tools *agenttools.Registry) agentservice.AgentService {
	return agentservice.NewService(planner, guard, audit, approvals, tools)
}
```

Create `internal/agent/service/planner_http.go`:

```go
package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	agentmodel "github.com/GoSimplicity/AI-CloudOps/internal/agent/model"
)

type HTTPPlanner struct {
	baseURL string
	client *http.Client
}

func NewHTTPPlanner(baseURL string) *HTTPPlanner {
	return &HTTPPlanner{
		baseURL: strings.TrimRight(baseURL, "/"),
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (p *HTTPPlanner) Plan(ctx context.Context, req agentmodel.AgentPlanRequest) (agentmodel.AgentPlanResponse, error) {
	var result agentmodel.AgentPlanResponse
	if err := p.post(ctx, "/api/v1/agent/plan", req, &result); err != nil {
		return agentmodel.AgentPlanResponse{}, err
	}
	return result, nil
}

func (p *HTTPPlanner) Respond(ctx context.Context, req agentmodel.AgentRespondRequest) (agentmodel.AgentRespondResponse, error) {
	var result agentmodel.AgentRespondResponse
	if err := p.post(ctx, "/api/v1/agent/respond", req, &result); err != nil {
		return agentmodel.AgentRespondResponse{}, err
	}
	return result, nil
}

func (p *HTTPPlanner) post(ctx context.Context, path string, payload any, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := p.client.Do(request)
	if err != nil {
		return fmt.Errorf("agent service unavailable: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("agent service returned status %d", response.StatusCode)
	}
	return json.NewDecoder(response.Body).Decode(out)
}
```

Modify `pkg/di/web.go` to accept `agentHdl *agentApi.AgentHandler` and call:

```go
agentHdl.RegisterRouters(server)
```

Run:

```powershell
go run -mod=mod github.com/google/wire/cmd/wire ./pkg/di
```

- [ ] **Step 7: Run backend tests**

Run:

```powershell
go test ./internal/agent/... ./pkg/di
```

Expected: PASS.

- [ ] **Step 8: Commit**

```powershell
git add internal/agent pkg/di
git commit -m "feat(agent): expose secure agent gateway"
```

---

### Task 5: VanAgentByPy Plan And Respond APIs

**Files:**
- Create: `app/models/agent_models.py`
- Create: `app/core/agents/intent_classifier.py`
- Create: `app/core/agents/tool_router.py`
- Create: `app/core/agents/orchestrator.py`
- Create: `app/api/routes/agent.py`
- Modify: `app/api/routes/__init__.py`
- Create: `tests/test_secure_agent.py`

- [ ] **Step 1: Write Python tests**

Create `tests/test_secure_agent.py`:

```python
from app.core.agents.intent_classifier import IntentClassifier
from app.core.agents.tool_router import ToolRouter
from app.core.agents.orchestrator import AgentOrchestrator
from app.models.agent_models import AgentPlanRequest


def test_intent_classifier_blocks_destructive_request():
    classifier = IntentClassifier()
    result = classifier.classify("执行 rm -rf /")
    assert result.intent == "high_risk_action"
    assert result.riskLevel == "high"


def test_tool_router_maps_disk_question_to_read_only_tool():
    router = ToolRouter()
    plan = router.plan("diagnosis", "帮我分析磁盘为什么满了")
    assert plan.toolCalls[0].tool == "disk.analyze"
    assert plan.riskLevel == "safe"


def test_orchestrator_returns_structured_plan():
    orchestrator = AgentOrchestrator()
    plan = orchestrator.plan(AgentPlanRequest(
        request_id="r1",
        session_id="s1",
        user_id="u1",
        message="帮我分析磁盘为什么满了",
    ))
    assert plan.intent == "diagnosis"
    assert plan.toolCalls
```

- [ ] **Step 2: Run tests to verify failure**

Run:

```powershell
pytest tests/test_secure_agent.py -v
```

Expected: FAIL because modules do not exist.

- [ ] **Step 3: Implement models**

Create `app/models/agent_models.py`:

```python
from typing import Any, Dict, List, Optional
from pydantic import BaseModel, Field


class ToolCall(BaseModel):
    tool: str
    parameters: Dict[str, Any] = Field(default_factory=dict)
    reason: str = ""


class IntentResult(BaseModel):
    intent: str
    confidence: float
    reason: str
    riskLevel: str


class AgentPlanRequest(BaseModel):
    request_id: str
    session_id: str
    user_id: str
    message: str


class AgentPlanResponse(BaseModel):
    intent: str
    confidence: float
    reason: str
    riskLevel: str
    toolCalls: List[ToolCall] = Field(default_factory=list)


class ToolResult(BaseModel):
    tool: str
    status: str
    result: Optional[Any] = None
    error: Optional[str] = None


class Approval(BaseModel):
    id: str
    toolCall: ToolCall
    status: str
    reason: str


class AgentRespondRequest(BaseModel):
    request_id: str
    session_id: str
    message: str
    intent: str
    toolResults: List[ToolResult] = Field(default_factory=list)
    approvals: List[Approval] = Field(default_factory=list)


class AgentRespondResponse(BaseModel):
    answer: str
```

- [ ] **Step 4: Implement classifier/router/orchestrator**

Create `app/core/agents/intent_classifier.py`:

```python
from app.models.agent_models import IntentResult


class IntentClassifier:
    def classify(self, message: str) -> IntentResult:
        text = message.lower()
        if "rm -rf" in text or "chmod 777" in text or "/etc/passwd" in text:
            return IntentResult(intent="high_risk_action", confidence=0.99, reason="dangerous operation detected", riskLevel="high")
        if "重启" in text or "restart" in text:
            return IntentResult(intent="low_risk_action", confidence=0.75, reason="restart request", riskLevel="low")
        if "为什么" in text or "分析" in text or "检查" in text or "满了" in text:
            return IntentResult(intent="diagnosis", confidence=0.85, reason="diagnosis request", riskLevel="safe")
        return IntentResult(intent="recommendation", confidence=0.6, reason="general recommendation", riskLevel="safe")
```

Create `app/core/agents/tool_router.py`:

```python
from app.models.agent_models import AgentPlanResponse, ToolCall


class ToolRouter:
    def plan(self, intent: str, message: str) -> AgentPlanResponse:
        text = message.lower()
        if intent == "high_risk_action":
            return AgentPlanResponse(intent=intent, confidence=0.99, reason="blocked high risk request", riskLevel="high", toolCalls=[])
        if "磁盘" in message or "disk" in text or "满了" in message:
            return AgentPlanResponse(intent=intent, confidence=0.85, reason="disk diagnosis", riskLevel="safe", toolCalls=[
                ToolCall(tool="disk.analyze", parameters={"path": "/"}, reason="check disk usage"),
                ToolCall(tool="system.inspect", parameters={}, reason="collect system overview"),
            ])
        if "容器" in message or "container" in text:
            return AgentPlanResponse(intent=intent, confidence=0.8, reason="container diagnosis", riskLevel="safe", toolCalls=[
                ToolCall(tool="container.inspect", parameters={}, reason="inspect containers"),
            ])
        if intent == "low_risk_action" and ("重启" in message or "restart" in text):
            return AgentPlanResponse(intent=intent, confidence=0.75, reason="restart needs approval", riskLevel="low", toolCalls=[
                ToolCall(tool="service.restart", parameters={"service": "demo"}, reason="restart requested service after approval"),
            ])
        return AgentPlanResponse(intent=intent, confidence=0.6, reason="general system inspection", riskLevel="safe", toolCalls=[
            ToolCall(tool="system.inspect", parameters={}, reason="collect system overview"),
        ])
```

Create `app/core/agents/orchestrator.py`:

```python
from app.core.agents.intent_classifier import IntentClassifier
from app.core.agents.tool_router import ToolRouter
from app.models.agent_models import AgentPlanRequest, AgentPlanResponse, AgentRespondRequest, AgentRespondResponse


class AgentOrchestrator:
    def __init__(self):
        self.classifier = IntentClassifier()
        self.router = ToolRouter()

    def plan(self, request: AgentPlanRequest) -> AgentPlanResponse:
        intent = self.classifier.classify(request.message)
        plan = self.router.plan(intent.intent, request.message)
        plan.confidence = intent.confidence
        plan.reason = intent.reason
        if intent.riskLevel == "high":
            plan.riskLevel = "high"
        return plan

    def respond(self, request: AgentRespondRequest) -> AgentRespondResponse:
        if request.approvals:
            return AgentRespondResponse(answer="已生成需要确认的低风险操作，请在页面中确认后执行。")
        blocked = [item for item in request.toolResults if item.status == "blocked"]
        if blocked:
            return AgentRespondResponse(answer=f"请求包含高风险操作，已被安全护栏拦截：{blocked[0].error}")
        return AgentRespondResponse(answer="诊断完成，已根据可用工具收集系统上下文。请查看工具结果和审计时间线。")
```

- [ ] **Step 5: Implement FastAPI route**

Create `app/api/routes/agent.py`:

```python
from fastapi import APIRouter

from app.core.agents.orchestrator import AgentOrchestrator
from app.models.agent_models import AgentPlanRequest, AgentPlanResponse, AgentRespondRequest, AgentRespondResponse

router = APIRouter(tags=["agent"])
orchestrator = AgentOrchestrator()


@router.post("/plan", response_model=AgentPlanResponse)
async def plan(request: AgentPlanRequest) -> AgentPlanResponse:
    return orchestrator.plan(request)


@router.post("/respond", response_model=AgentRespondResponse)
async def respond(request: AgentRespondRequest) -> AgentRespondResponse:
    return orchestrator.respond(request)
```

Modify `app/api/routes/__init__.py`:

```python
try:
    from .agent import router as agent_router

    api_v1.include_router(agent_router, prefix="/agent")
    logger.info("已注册安全Agent路由")
except Exception as e:
    logger.warning(f"注册安全Agent路由失败: {str(e)}")
```

- [ ] **Step 6: Run Python tests**

Run:

```powershell
pytest tests/test_secure_agent.py -v
```

Expected: PASS.

- [ ] **Step 7: Commit**

```powershell
git add app/models/agent_models.py app/core/agents/intent_classifier.py app/core/agents/tool_router.py app/core/agents/orchestrator.py app/api/routes/agent.py app/api/routes/__init__.py tests/test_secure_agent.py
git commit -m "feat(agent): add secure planning endpoints"
```

---

### Task 6: WebUI Agent API And Page

**Files:**
- Create: `apps/web-antd/src/api/core/system/agent.ts`
- Create: `apps/web-antd/src/api/core/system/agent.test.ts`
- Create: `apps/web-antd/src/views/system/AgentAssistant.vue`
- Create: `apps/web-antd/src/views/system/agent-assistant.css`
- Modify: `apps/web-antd/src/router/routes/modules/files.ts`

- [ ] **Step 1: Write API normalization tests**

Create `apps/web-antd/src/api/core/system/agent.test.ts`:

```ts
import { describe, expect, it } from 'vitest';

import { normalizeAgentResponse } from './agent';

describe('agent api', () => {
  it('unwraps envelopes', () => {
    expect(normalizeAgentResponse({ code: 0, data: { answer: 'ok' } })).toEqual({ answer: 'ok' });
  });

  it('throws failed envelope message', () => {
    expect(() => normalizeAgentResponse({ code: 1, data: null, message: 'blocked' })).toThrow('blocked');
  });
});
```

- [ ] **Step 2: Run tests to verify failure**

Run:

```powershell
pnpm exec vitest run apps/web-antd/src/api/core/system/agent.test.ts
```

Expected: FAIL because API file does not exist.

- [ ] **Step 3: Implement API**

Create `apps/web-antd/src/api/core/system/agent.ts`:

```ts
import { preferences } from '@vben/preferences';
import { useAccessStore } from '@vben/stores';

import { baseRequestClient } from '#/api/request';

interface ApiEnvelope<T> {
  code: number;
  data: T;
  message?: string;
}

export interface AgentSession {
  id: string;
  userId: string;
  username: string;
  createdAt: string;
}

export interface AgentApproval {
  id: string;
  requestId: string;
  toolCall: { tool: string; parameters: Record<string, any>; reason: string };
  riskLevel: string;
  status: string;
  reason: string;
  expiresAt: string;
}

export interface AgentEvent {
  id: string;
  requestId: string;
  stage: string;
  message: string;
  createdAt: string;
}

export interface AgentQueryResponse {
  requestId: string;
  answer: string;
  intent: string;
  riskLevel: string;
  events: AgentEvent[];
  approvals: AgentApproval[];
}

function authHeaders() {
  const accessStore = useAccessStore();
  return {
    'Accept-Language': preferences.app.locale,
    Authorization: accessStore.accessToken ? `Bearer ${accessStore.accessToken}` : undefined,
  };
}

export function normalizeAgentResponse<T>(payload: ApiEnvelope<T> | T): T {
  if (payload && typeof payload === 'object' && 'code' in payload && 'data' in payload) {
    const envelope = payload as ApiEnvelope<T>;
    if (envelope.code === 0) return envelope.data;
    throw new Error(envelope.message || 'Agent API request failed');
  }
  return payload as T;
}

export async function createAgentSession() {
  const response = await baseRequestClient.post<any>('/system/agent/sessions', undefined, { headers: authHeaders() });
  return normalizeAgentResponse<AgentSession>(response.data);
}

export async function queryAgent(sessionId: string, message: string) {
  const response = await baseRequestClient.post<any>('/system/agent/query', { sessionId, message }, { headers: authHeaders() });
  return normalizeAgentResponse<AgentQueryResponse>(response.data);
}

export async function confirmAgentApproval(id: string) {
  const response = await baseRequestClient.post<any>(`/system/agent/approvals/${id}/confirm`, undefined, { headers: authHeaders() });
  return normalizeAgentResponse<AgentQueryResponse>(response.data);
}

export async function rejectAgentApproval(id: string) {
  const response = await baseRequestClient.post<any>(`/system/agent/approvals/${id}/reject`, undefined, { headers: authHeaders() });
  return normalizeAgentResponse<AgentApproval>(response.data);
}
```

- [ ] **Step 4: Implement page and route**

Create `apps/web-antd/src/views/system/AgentAssistant.vue`:

```vue
<template>
  <div class="agent-assistant">
    <a-card :bordered="false" class="agent-assistant__chat">
      <div class="agent-assistant__messages">
        <div v-for="item in messages" :key="item.id" :class="['agent-assistant__message', item.role]">
          <strong>{{ item.role === 'user' ? '你' : 'Agent' }}</strong>
          <p>{{ item.content }}</p>
        </div>
      </div>
      <a-textarea v-model:value="input" :rows="3" placeholder="描述你要诊断的问题，例如：帮我分析磁盘为什么满了" />
      <a-button type="primary" :loading="loading" @click="send">发送</a-button>
    </a-card>

    <a-card :bordered="false" title="审计时间线">
      <a-timeline>
        <a-timeline-item v-for="event in events" :key="event.id">
          <strong>{{ event.stage }}</strong>
          <div>{{ event.message }}</div>
        </a-timeline-item>
      </a-timeline>
    </a-card>

    <a-card v-if="approvals.length" :bordered="false" title="待确认操作">
      <a-list :data-source="approvals">
        <template #renderItem="{ item }">
          <a-list-item>
            <a-list-item-meta :title="item.toolCall.tool" :description="item.reason" />
            <a-space>
              <a-button type="primary" @click="confirm(item.id)">确认执行</a-button>
              <a-button @click="reject(item.id)">拒绝</a-button>
            </a-space>
          </a-list-item>
        </template>
      </a-list>
    </a-card>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { message } from 'ant-design-vue';

import { confirmAgentApproval, createAgentSession, queryAgent, rejectAgentApproval, type AgentApproval, type AgentEvent } from '#/api/core/system/agent';
import './agent-assistant.css';

const input = ref('');
const loading = ref(false);
const sessionId = ref('');
const events = ref<AgentEvent[]>([]);
const approvals = ref<AgentApproval[]>([]);
const messages = ref<{ id: string; role: 'agent' | 'user'; content: string }[]>([]);

async function ensureSession() {
  if (sessionId.value) return;
  const session = await createAgentSession();
  sessionId.value = session.id;
}

async function send() {
  if (!input.value.trim()) return;
  await ensureSession();
  const text = input.value.trim();
  messages.value.push({ id: `u-${Date.now()}`, role: 'user', content: text });
  input.value = '';
  loading.value = true;
  try {
    const result = await queryAgent(sessionId.value, text);
    messages.value.push({ id: result.requestId, role: 'agent', content: result.answer });
    events.value = result.events || [];
    approvals.value = result.approvals || [];
  } catch (error: any) {
    message.error(error?.message || 'Agent 请求失败');
  } finally {
    loading.value = false;
  }
}

async function confirm(id: string) {
  const result = await confirmAgentApproval(id);
  messages.value.push({ id: `confirm-${id}`, role: 'agent', content: result.answer });
  events.value = result.events || events.value;
  approvals.value = approvals.value.filter((item) => item.id !== id);
}

async function reject(id: string) {
  await rejectAgentApproval(id);
  approvals.value = approvals.value.filter((item) => item.id !== id);
}

onMounted(() => {
  void ensureSession();
});
</script>
```

Create `apps/web-antd/src/views/system/agent-assistant.css`:

```css
.agent-assistant {
  display: grid;
  grid-template-columns: minmax(0, 1.4fr) minmax(320px, 0.8fr);
  gap: 16px;
  min-height: calc(100vh - 120px);
  padding: 18px;
  background: linear-gradient(135deg, #eef6f4 0%, #f7f3e8 100%);
}

.agent-assistant__chat {
  min-height: 560px;
}

.agent-assistant__messages {
  height: 420px;
  margin-bottom: 14px;
  overflow: auto;
}

.agent-assistant__message {
  max-width: 82%;
  padding: 12px 14px;
  margin-bottom: 12px;
  border-radius: 14px;
}

.agent-assistant__message.user {
  margin-left: auto;
  background: #1677ff;
  color: white;
}

.agent-assistant__message.agent {
  background: #fff;
  border: 1px solid rgb(0 0 0 / 6%);
}

@media (width <= 960px) {
  .agent-assistant {
    grid-template-columns: 1fr;
  }
}
```

Modify `apps/web-antd/src/router/routes/modules/files.ts` under `/system` children:

```ts
{
  name: 'SystemAgent',
  path: '/system/agent',
  component: () => import('#/views/system/AgentAssistant.vue'),
  meta: {
    icon: 'lucide:bot',
    title: '智能助手',
  },
},
```

- [ ] **Step 5: Run frontend tests and typecheck**

Run:

```powershell
pnpm exec vitest run apps/web-antd/src/api/core/system/agent.test.ts
pnpm --filter @vben/web-antd typecheck
```

Expected: PASS.

- [ ] **Step 6: Commit**

```powershell
git add apps/web-antd/src/api/core/system/agent.ts apps/web-antd/src/api/core/system/agent.test.ts apps/web-antd/src/views/system/AgentAssistant.vue apps/web-antd/src/views/system/agent-assistant.css apps/web-antd/src/router/routes/modules/files.ts
git commit -m "feat(agent): add secure assistant webui"
```

---

### Task 7: End-To-End Verification

**Files:**
- Modify only if verification reveals defects.

- [ ] **Step 1: Backend verification**

Run in `VanPanelBackend` worktree:

```powershell
go test ./internal/agent/... ./pkg/di
go test ./...
```

Expected: PASS.

- [ ] **Step 2: VanAgentByPy verification**

Run in `VanAgentByPy` worktree:

```powershell
pytest tests/test_secure_agent.py -v
```

Expected: PASS.

- [ ] **Step 3: WebUI verification**

Run in `VanPanelWebUI` worktree:

```powershell
pnpm exec vitest run apps/web-antd/src/api/core/system/agent.test.ts
pnpm --filter @vben/web-antd typecheck
```

Expected: PASS.

- [ ] **Step 4: Manual smoke test**

Start VanAgentByPy:

```powershell
python -m app.main
```

Start VanPanelBackend and WebUI using the repo's normal dev commands.

Verify:

- Open `/system/agent`.
- Send `帮我分析磁盘为什么满了`.
- Confirm response contains diagnosis text and audit timeline.
- Send `执行 rm -rf /`.
- Confirm response says Risk Guard blocked the request.
- Send `重启 demo 服务`.
- Confirm an approval card appears instead of immediate execution.
- Confirm approval and verify the response says the approved operation executed.

- [ ] **Step 5: Commit verification fixes**

If fixes are needed, run `git status --short`, stage only the files changed by the verification fix, and commit:

```powershell
git status --short
git commit -m "fix(agent): address verification issues"
```

If no fixes are needed, do not create an empty commit.
