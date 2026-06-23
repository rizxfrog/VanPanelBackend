# Gateway 聊天增强 & 会话管理 — 事件广播补完

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 完成 Gateway RPC 层聊天增强和会话管理的最后缺口：事件广播到订阅者、compaction stub 实现、preview/describe 实现。

**Architecture:** 现有 RPC 处理器已完整实现（abort/metadata/message.get/subscribe/reset/compact 等），但 `sessions.changed` 事件和消息推送从未实际广播给订阅者。需要让 RPC 处理器能访问 `BroadcastManager`，在会话变更后推送事件。

**Tech Stack:** Go · gorilla/websocket · GORM

**现状对照：**

| 方法 | 状态 | 差距 |
|------|------|------|
| `chat.abort` | ✅ 已实现 | 无 |
| `chat.metadata` | ✅ 已实现 | 无 |
| `chat.message.get` | ✅ 已实现 | 无 |
| `sessions.subscribe` | ✅ 已实现 | 无 |
| `sessions.messages.subscribe` | ✅ 已实现 | 无 |
| `sessions.messages.unsubscribe` | ✅ 已实现 | 无 |
| `sessions.reset` | ✅ 已实现 | 无 |
| `sessions.compact` | ✅ 已实现 | 无 |
| `sessions.compaction.list` | ❌ Stub（空数组） | 需实现 |
| `sessions.compaction.get` | ❌ Stub（nil） | 需实现 |
| `sessions.compaction.branch` | ❌ Stub（{ok:true}） | 需实现 |
| `sessions.compaction.restore` | ❌ Stub（{ok:true}） | 需实现 |
| `sessions.preview` | ❌ Stub（nil） | 需实现 |
| `sessions.describe` | ❌ Stub（nil） | 需实现 |
| **事件广播** | ❌ 缺失 | 订阅者收不到推送 |

---

## Task 1: 让 RPC 处理器能访问 BroadcastManager

**Files:**
- Modify: `internal/gateway/rpc/chat.go:23-33` — 添加 broadcastMgr 包变量和 setter
- Modify: `main.go:195-196` — 调用新增的 SetBroadcastManager

**Step 1: 在 chat.go 中添加 BroadcastManager 引用**

在 `internal/gateway/rpc/chat.go` 中找到现有变量声明区域（第 18-33 行），在 `SetRunTracker` 之后添加：

```go
// broadcastMgr is set during server initialization for event broadcasting.
var broadcastMgr *gateway.BroadcastManager

// SetBroadcastManager sets the BroadcastManager for gateway RPC handlers.
func SetBroadcastManager(bm *gateway.BroadcastManager) {
	broadcastMgr = bm
}
```

**Step 2: 在 main.go 中调用 SetBroadcastManager**

在 `internal/main.go` 找到 `gatewayRpc.SetSubscriptionHub(subHub)` 之后（约第 196 行），添加：

```go
gatewayRpc.SetBroadcastManager(broadcastMgr)
```

**Step 3: 编译验证**

```bash
go build main.go
```

验证编译通过。

**Step 4: Commit**

```bash
git add internal/gateway/rpc/chat.go main.go
git commit -m "feat: add BroadcastManager reference to gateway RPC handlers"
```

---

## Task 2: 会话变更时广播 sessions.changed 事件

**Files:**
- Modify: `internal/gateway/rpc/sessions.go:114-138` — handleSessionsCreate 广播 create
- Modify: `internal/gateway/rpc/sessions.go:140-160` — handleSessionsDelete 广播 delete
- Modify: `internal/gateway/rpc/sessions.go:162-205` — handleSessionsPatch 广播 patch
- Modify: `internal/gateway/rpc/sessions.go:286-314` — handleSessionsReset 广播 reset
- Modify: `internal/gateway/rpc/sessions.go:316-348` — handleSessionsCompact 广播 compact

**Step 1: 创建通知辅助函数**

在 `internal/gateway/rpc/sessions.go` 中 `parseSessionKey` 函数之前添加：

```go
// notifySessionChanged 向所有订阅了会话列表的连接广播 sessions.changed 事件
func notifySessionChanged(key, reason string, session map[string]interface{}) {
	if broadcastMgr == nil || subHub == nil {
		return
	}
	payload := map[string]interface{}{
		"key":    key,
		"reason": reason,
	}
	if session != nil {
		payload["session"] = session
	}
	for _, connID := range subHub.GetSessionSubscribers() {
		broadcastMgr.BroadcastTo(connID, "sessions.changed", payload)
	}
}

// notifySessionMessage 向订阅了某会话消息的连接广播消息事件
func notifySessionMessage(sessionKey string, payload interface{}) {
	if broadcastMgr == nil || subHub == nil {
		return
	}
	for _, connID := range subHub.GetMessageSubscribers(sessionKey) {
		broadcastMgr.BroadcastTo(connID, "chat", payload)
	}
}
```

**Step 2: 在 handleSessionsCreate 中调用通知**

在 handleSessionsCreate 末尾 `return` 之前添加通知：

```go
notifySessionChanged("agent:main:"+strconv.Itoa(result.ID), "create", sessionToRow(result))
```

**Step 3: 在 handleSessionsDelete 中调用通知**

在删除成功后 `return` 之前添加：

```go
notifySessionChanged(reqParams.Key, "delete", nil)
```

**Step 4: 在 handleSessionsPatch 中调用通知**

在更新成功后 `return` 之前添加：

```go
notifySessionChanged("agent:main:"+strconv.Itoa(existing.ID), "patch", sessionToRow(existing))
```

**Step 5: 在 handleSessionsReset 中调用通知**

在重置成功后 `return` 之前添加：

```go
notifySessionChanged(req.Key, "reset", sessionToRow(session))
```

**Step 6: 在 handleSessionsCompact 中调用通知**

在压缩成功后 `return` 之前添加：

```go
notifySessionChanged(req.Key, "compact", nil)
```

**Step 7: 编译验证**

```bash
go build main.go
```

Expected: 编译通过。

**Step 8: Commit**

```bash
git add internal/gateway/rpc/sessions.go
git commit -m "feat: broadcast sessions.changed events on session mutations"
```

---

## Task 3: 聊天消息推送给订阅者

**Files:**
- Modify: `internal/gateway/rpc/chat.go:44-120` — handleChatSend 广播消息事件
- Modify: `internal/gateway/adapter/chat_stream.go:86-91, 266-290, 292-311` — 流式适配器广播

**Step 1: 在 chat.send 广播 chat 事件给消息订阅者**

在 `handleChatSend` 中，goroutine 内发送 chat 事件的同时（约第 86-114 行），在 `conn.SendEvent("chat", ...)` 调用后添加广播。

因为 chat 事件由 `ChatStreamAdapter` 通过 `conn.SendEvent("chat", ...)` 发送，我们需要在适配器中添加广播逻辑。但适配器没有 BroadcastManager 引用。

**更简洁的方案**：在 `ChatStreamAdapter` 构造函数中接收 `broadcastMgr` 和 `subHub` 引用，在每次发送 chat 事件时同步广播给同一 session 的其他订阅者。

修改 `internal/gateway/adapter/chat_stream.go`：

在 `ChatStreamAdapter` 结构体中添加字段：

```go
type ChatStreamAdapter struct {
	conn       *gateway.GatewayConnection
	runID      string
	sessionKey string
	agentID    string
	seq        atomic.Int32
	builder    strings.Builder
	ctx        context.Context
	toolBlocks []gateway.ContentBlock
	bm         *gateway.BroadcastManager  // 新增
	subHub     *gateway.SubscriptionHub   // 新增 (需要引用 gateway 包)
}
```

**更简洁方案（避免循环依赖）**：不在适配器中广播，而是在 `chat.go` 的 goroutine 中，每次收到 adapter 写入后广播。但 adapter 是 io.Writer，不方便拦截个别事件。

**最终方案**：在 `ChatStreamAdapter` 内添加方法级广播逻辑。`BroadcastManager` 和 `SubscriptionHub` 已经在 `gateway` 包中，`adapter` 包已经 import `gateway`，无循环依赖。

添加 `sendChatEvent` 方法替代直接调用 `conn.SendEvent("chat", ...)`：

```go
func (a *ChatStreamAdapter) sendChatEvent(event gateway.ChatEvent) {
	a.conn.SendEvent("chat", event)
	if a.bm != nil && a.subHub != nil {
		for _, connID := range a.subHub.GetMessageSubscribers(a.sessionKey) {
			if connID != a.conn.ID {
				a.bm.BroadcastTo(connID, "chat", event)
			}
		}
	}
}
```

修改所有 `a.conn.SendEvent("chat", ...)` 调用为 `a.sendChatEvent(...)`。

修改构造函数 `NewChatStreamAdapter` 接受额外参数：

```go
func NewChatStreamAdapter(ctx context.Context, conn *gateway.GatewayConnection, runID, sessionKey, agentID string, bm *gateway.BroadcastManager, subHub *gateway.SubscriptionHub) *ChatStreamAdapter {
```

**Step 2: 更新 chat.go 中调用 NewChatStreamAdapter 的地方**

在 `handleChatSend` goroutine 中（约第 86 行）：

```go
chatAdapter := adapter.NewChatStreamAdapter(runCtx, conn, runID, req.SessionKey, req.AgentID, broadcastMgr, subHub)
```

**Step 3: 编译验证**

```bash
go build main.go
```

Expected: 编译通过。

**Step 4: Commit**

```bash
git add internal/gateway/adapter/chat_stream.go internal/gateway/rpc/chat.go
git commit -m "feat: broadcast chat events to message subscribers on same session"
```

---

## Task 4: 实现 sessions.compaction.*（压缩分支管理）

**Files:**
- Modify: `internal/gateway/rpc/sessions.go:32-36` — 注册和实现 compaction handlers

Compaction 分支是会话历史的压缩快照。用 in-memory map 实现，带基本 CRUD。

**Step 1: 添加 compaction 存储结构**

在 `internal/gateway/rpc/sessions.go` 的 `var subHub` 附近（约第 16 行后）添加：

```go
// compactionStore 是会话压缩分支的内存存储（按 sessionKey 索引）
// map[sessionKey] → []branch
var compactionStore = make(map[string][]compactionBranch)
var compactionMu sync.Mutex

type compactionBranch struct {
	ID        string `json:"id"`
	SessionKey string `json:"sessionKey"`
	Label     string `json:"label"`
	MessageCount int  `json:"messageCount"`
	TokenCount   int  `json:"tokenCount,omitempty"`
	CreatedAt    int64 `json:"createdAt"`
	Summary      string `json:"summary,omitempty"`
}
```

需要在 import 中添加 `"sync"`。

**Step 2: 实现 handleSessionsCompactionList**

替换 `handleSessionsEmptyArray` 注册，改为专用 handler：

```go
func handleSessionsCompactionList(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	var req struct {
		Key string `json:"key"`
	}
	json.Unmarshal(params, &req)

	compactionMu.Lock()
	defer compactionMu.Unlock()

	branches, ok := compactionStore[req.Key]
	if !ok {
		return []interface{}{}, nil
	}

	result := make([]interface{}, 0, len(branches))
	for _, b := range branches {
		result = append(result, map[string]interface{}{
			"id":           b.ID,
			"sessionKey":   b.SessionKey,
			"label":        b.Label,
			"messageCount": b.MessageCount,
			"tokenCount":   b.TokenCount,
			"createdAt":    b.CreatedAt,
			"summary":      b.Summary,
		})
	}
	return result, nil
}
```

更新 init() 中的注册：

```go
gateway.RegisterMethod("sessions.compaction.list", string(gateway.ScopeRead), handleSessionsCompactionList)
```

**Step 3: 实现 handleSessionsCompactionGet**

```go
func handleSessionsCompactionGet(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	var req struct {
		Key      string `json:"key"`
		BranchID string `json:"branchId"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, fmt.Errorf("解析参数失败: %w", err)
	}

	compactionMu.Lock()
	defer compactionMu.Unlock()

	branches := compactionStore[req.Key]
	for _, b := range branches {
		if b.ID == req.BranchID {
			return map[string]interface{}{
				"id":           b.ID,
				"sessionKey":   b.SessionKey,
				"label":        b.Label,
				"messageCount": b.MessageCount,
				"tokenCount":   b.TokenCount,
				"createdAt":    b.CreatedAt,
				"summary":      b.Summary,
			}, nil
		}
	}
	return nil, nil
}
```

更新注册：

```go
gateway.RegisterMethod("sessions.compaction.get", string(gateway.ScopeRead), handleSessionsCompactionGet)
```

**Step 4: 实现 handleSessionsCompactionBranch**

```go
func handleSessionsCompactionBranch(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	var req struct {
		Key   string `json:"key"`
		Label string `json:"label,omitempty"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, fmt.Errorf("解析参数失败: %w", err)
	}
	if req.Key == "" {
		return nil, fmt.Errorf("会话 key 不能为空")
	}

	// Count current messages for this session
	messageCount := 0
	if agentSvc != nil {
		id, err := parseSessionKey(req.Key)
		if err == nil {
			messages, _ := agentSvc.ListMessages(ctx, &model.ListAgentMessagesReq{
				SessionID: strconv.Itoa(id),
				ListReq:   model.ListReq{Page: 1, Size: 1},
			})
			if messages != nil {
				messageCount = messages.Total
			}
		}
	}

	label := req.Label
	if label == "" {
		label = fmt.Sprintf("快照 %s", time.Now().Format("2006-01-02 15:04:05"))
	}

	branchID := fmt.Sprintf("branch-%d", time.Now().UnixNano())
	branch := compactionBranch{
		ID:           branchID,
		SessionKey:   req.Key,
		Label:        label,
		MessageCount: messageCount,
		CreatedAt:    time.Now().UnixMilli(),
	}

	compactionMu.Lock()
	compactionStore[req.Key] = append(compactionStore[req.Key], branch)
	compactionMu.Unlock()

	return map[string]interface{}{
		"ok":       true,
		"branchId": branchID,
	}, nil
}
```

更新注册：

```go
gateway.RegisterMethod("sessions.compaction.branch", string(gateway.ScopeWrite), handleSessionsCompactionBranch)
```

**Step 5: 实现 handleSessionsCompactionRestore**

```go
func handleSessionsCompactionRestore(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	var req struct {
		Key      string `json:"key"`
		BranchID string `json:"branchId"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, fmt.Errorf("解析参数失败: %w", err)
	}
	if req.Key == "" || req.BranchID == "" {
		return nil, fmt.Errorf("key 和 branchId 不能为空")
	}

	compactionMu.Lock()
	branches := compactionStore[req.Key]
	found := false
	for _, b := range branches {
		if b.ID == req.BranchID {
			found = true
			break
		}
	}
	compactionMu.Unlock()

	if !found {
		return nil, fmt.Errorf("压缩分支 %s 不存在", req.BranchID)
	}

	// Restore: reset session to clear messages, then notify
	if err := requireAgentSvc(); err != nil {
		return nil, err
	}
	id, err := parseSessionKey(req.Key)
	if err != nil {
		return nil, err
	}
	session, err := agentSvc.ResetSession(ctx, req.Key, defaultUserID)
	if err != nil {
		return nil, fmt.Errorf("恢复会话失败: %w", err)
	}

	return map[string]interface{}{
		"ok":       true,
		"sessionId": strconv.Itoa(session.ID),
	}, nil
}
```

更新注册：

```go
gateway.RegisterMethod("sessions.compaction.restore", string(gateway.ScopeAdmin), handleSessionsCompactionRestore)
```

**Step 6: 编译验证**

```bash
go build main.go
```

**Step 7: Commit**

```bash
git add internal/gateway/rpc/sessions.go
git commit -m "feat: implement sessions.compaction.* CRUD handlers"
```

---

## Task 5: 实现 sessions.preview 和 sessions.describe

**Files:**
- Modify: `internal/gateway/rpc/sessions.go:40-41` — 替换 stub

**Step 1: 实现 handleSessionsPreview**

```go
func handleSessionsPreview(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	var req struct {
		Key     string `json:"key"`
		AgentID string `json:"agentId,omitempty"`
	}
	json.Unmarshal(params, &req)

	if req.Key == "" {
		return nil, nil
	}

	// Get last few messages as preview
	if agentSvc == nil {
		return nil, nil
	}
	id, err := parseSessionKey(req.Key)
	if err != nil {
		return nil, nil
	}
	session, err := agentSvc.GetSession(ctx, id)
	if err != nil {
		return nil, nil
	}
	messages, err := agentSvc.ListMessages(ctx, &model.ListAgentMessagesReq{
		SessionID: strconv.Itoa(id),
		ListReq:   model.ListReq{Page: 1, Size: 3},
	})
	if err != nil || messages == nil {
		return nil, nil
	}

	var previews []map[string]interface{}
	for i := len(messages.Items) - 1; i >= 0; i-- {
		msg := messages.Items[i]
		previews = append(previews, map[string]interface{}{
			"role":    msg.Role,
			"content": truncateString(msg.Content, 200),
		})
	}

	return map[string]interface{}{
		"key":      req.Key,
		"label":    session.Title,
		"messages": previews,
		"status":   session.Status,
	}, nil
}
```

**Step 2: 实现 handleSessionsDescribe**

```go
func handleSessionsDescribe(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	var req struct {
		Key string `json:"key"`
	}
	json.Unmarshal(params, &req)

	if req.Key == "" || agentSvc == nil {
		return nil, nil
	}
	id, err := parseSessionKey(req.Key)
	if err != nil {
		return nil, nil
	}

	session, err := agentSvc.GetSession(ctx, id)
	if err != nil {
		return nil, nil
	}
	messages, err := agentSvc.ListMessages(ctx, &model.ListAgentMessagesReq{
		SessionID: strconv.Itoa(id),
		ListReq:   model.ListReq{Page: 1, Size: 1},
	})
	if err != nil {
		return nil, nil
	}

	msgCount := 0
	if messages != nil {
		msgCount = messages.Total
	}

	return map[string]interface{}{
		"key":          req.Key,
		"label":        session.Title,
		"status":       session.Status,
		"messageCount": msgCount,
		"createdAt":    session.CreatedAt.UnixMilli(),
		"updatedAt":    session.UpdatedAt.UnixMilli(),
		"modelName":    session.ModelName,
	}, nil
}
```

**Step 3: 添加 truncateString 辅助函数**

```go
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
```

**Step 4: 更新注册**

```go
gateway.RegisterMethod("sessions.preview", string(gateway.ScopeRead), handleSessionsPreview)
gateway.RegisterMethod("sessions.describe", string(gateway.ScopeRead), handleSessionsDescribe)
```

**Step 5: 编译验证**

```bash
go build main.go
```

**Step 6: Commit**

```bash
git add internal/gateway/rpc/sessions.go
git commit -m "feat: implement sessions.preview and sessions.describe handlers"
```

---

## 验证清单

- [ ] `go build main.go` 编译通过
- [ ] `go vet ./...` 无错误
- [ ] 已在 sessions handlers 中注册所有 compaction/preview/describe 方法

---

## 依赖关系

```
Task 1 (BroadcastManager 引用) 
  └─> Task 2 (sessions.changed 广播)
  └─> Task 3 (消息推送广播)
Task 4 (compaction CRUD) [独立]
Task 5 (preview/describe) [独立]
```

Tasks 1-3 有顺序依赖，Tasks 4-5 可以并行执行。
