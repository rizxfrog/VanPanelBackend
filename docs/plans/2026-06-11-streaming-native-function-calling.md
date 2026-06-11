# Streaming Native Function Calling Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace custom text-based tool calling in QueryStream/QueryStreamWithPipeline with standard OpenAI function calling via eino's react.Agent.Stream().

**Architecture:** Replace `textReActLoop` with `react.NewAgent` (same config as sync Query path). Use `agent.Stream()` to get streaming messages from eino framework, then convert the stream output to SSE events (delta/tool_call/tool_result/done). The react.Agent handles the ReAct loop internally — max 10 steps, tools auto-called, results fed back. Safety/audit via existing `wrapTool` + `safeTool`.

**Tech Stack:** Go 1.25.5, eino v0.9.2, einoopenai v0.1.13

---

## Current vs Target

### Current (textReActLoop)
```
chatModel.Stream(messages) → token deltas → accumulate fullContent
→ parseTextToolCalls(fullContent) via regex
→ executeTool() manually
→ loop back
```

### Target (react.Agent.Stream)
```
react.NewAgent(ToolCallingModel, Tools) → agent.Stream(messages)
→ StreamReader[*Message] with native Content + ToolCalls
→ convert to SSE events
→ agent handles ReAct loop internally
```

---

### Task 0: Explore Understanding

**Goal:** Verify no other code depends on text_react.go exports beyond service.go

**Step 1: Search for textReActLoop references**

Check: `textReActLoop|newTextReActLoop|parseTextToolCalls|stripToolCallXML|injectToolPrompt|parseJSONToolCalls|jsonToolCallRegex|textToolCallRegex`

Expected: Only used in service.go (QueryStream, QueryStreamWithPipeline, Query/QueryWithPipeline) and text_react.go itself.

---

### Task 1: Build ReAct Agent Stream Runner

**Files:**
- Create: `internal/agent/service/agent_stream.go`

**Purpose:** Extract the common "build ReAct Agent, call Stream, convert to SSE" logic into a reusable function, shared by QueryStream and QueryStreamWithPipeline.

**Step 1: Write `runAgentStream` function**

```go
package service

import (
    "context"
    "encoding/json"
    "fmt"
    "io"

    "github.com/cloudwego/eino/components/tool"
    "github.com/cloudwego/eino/compose"
    "github.com/cloudwego/eino/flow/agent/react"
    "github.com/cloudwego/eino/schema"
    "go.uber.org/zap"

    agentaudit "github.com/rizxfrog/VanPanelBackend/internal/agent/audit"
)

// runAgentStream creates a ReAct agent with native function calling, runs it in stream mode,
// and writes SSE events (delta / tool_call / tool_result / done) to the writer.
// Returns the final text content accumulated from all deltas.
func runAgentStream(
    ctx context.Context,
    chatModel modelInterface, // ChatModel implementing ToolCallingChatModel
    safeTools []tool.BaseTool,
    messages []*schema.Message,
    writer io.Writer,
    writeSSE func(io.Writer, string, interface{}) error,
    logger *zap.Logger,
    sessionID string,
    userID int,
    personaPrompt string,
    auditFn func(ctx context.Context, action, toolName, reason string, riskLevel string, allowed bool, args string, result string),
) (string, error) {
    
    // Create ReAct Agent with standard function calling
    agent, err := react.NewAgent(ctx, &react.AgentConfig{
        ToolCallingModel: chatModel,
        ToolsConfig:      compose.ToolsNodeConfig{Tools: safeTools},
        MaxStep:          10,
        MessageModifier:  react.NewPersonaModifier(personaPrompt),
    })
    if err != nil {
        return "", fmt.Errorf("创建 Agent 失败: %w", err)
    }

    // Stream
    sr, err := agent.Stream(ctx, messages)
    if err != nil {
        return "", fmt.Errorf("Agent 流式执行失败: %w", err)
    }
    defer sr.Close()

    var finalContent string
    for {
        msg, err := sr.Recv()
        if err == io.EOF {
            break
        }
        if err != nil {
            return finalContent, fmt.Errorf("Stream 接收错误: %w", err)
        }

        // Text delta
        if msg.Content != "" {
            finalContent += msg.Content
            writeSSE(writer, "delta", map[string]interface{}{
                "role":    string(schema.Assistant),
                "content": msg.Content,
            })
        }

        // Tool call delta (standard OpenAI format)
        for _, tc := range msg.ToolCalls {
            auditFn(ctx, "tool.call", tc.Function.Name, "", "safe", true, tc.Function.Arguments, "")

            writeSSE(writer, "tool_call", map[string]interface{}{
                "id":        tc.ID,
                "name":      tc.Function.Name,
                "arguments": tc.Function.Arguments,
            })
        }
    }

    // Send done event
    writeSSE(writer, "done", map[string]interface{}{
        "session_id": sessionID,
        "result": map[string]string{
            "answer": finalContent,
        },
    })

    return finalContent, nil
}
```

---

### Task 2: Improve SSE Events to Surface Tool Results

**Files:**
- Modify: `internal/agent/service/agent_stream.go`

**Purpose:** The react.Agent framework supports `toolResultSender` callbacks that fire when tools execute. Use this to send `tool_result` SSE events.

**Step 1: Add setToolResultSendersToCtx before calling agent.Stream()**

```go
senders := &reactToolResultSenders{} // defined in react package as toolResultSenders
ctx = setToolResultSendersToCtx(ctx, &toolResultSenders{
    sender: func(toolName, callID, result string) {
        writeSSE(writer, "tool_result", map[string]interface{}{
            "name":   toolName,
            "result": truncateString(result, 2000),
        })
    },
})
```

Wait — `toolResultSenders` and `setToolResultSendersToCtx` are unexported in the react package. Let me check...

Actually, these are in `react` package and are lowercase/unexported. We cannot use them directly from our `service` package.

**Alternative approach:** Since the react.Agent handles the ReAct loop internally (model call → tool call → model call → tool result → ...), the `agent.Stream()` output already includes the tool results as subsequent model messages. We can detect tool results in the stream:
- When we see a `ToolCall`, send `tool_call` SSE
- The next messages in the stream will be the model's response after receiving tool results
- We can also wrap tools to send `tool_result` from inside the tool execution

Use `wrapTool` (same as sync path) and modify `safeTool.InvokableRun` to also send SSE events. But `safeTool` doesn't have access to SSE writer...

**Simplest approach:** Use eino's `compose.ToolMiddleware` with a stream callback. The `react` package's `newToolResultCollectorMiddleware()` internally uses context-based callbacks. Since we can't access unexported types, we can:
1. Use `compose.ToolsNodeConfig.ToolCallMiddlewares` to add a middleware that sends `tool_result` SSE events
2. The middleware has access to `input.Name`, `input.CallID`, and `output.Result`

Wait, actually let me re-read the react.NewAgent code. It inserts `newToolResultCollectorMiddleware()` into `config.ToolsConfig.ToolCallMiddlewares` before creating the tools node. But we pass our tools through `ToolsConfig` which has `ToolCallMiddlewares` field.

Actually, the middleware approach won't work because our `safeTool` is the one executing — we need to hook into its execution.

Let me take a simpler approach: modify `safeTool` to accept an optional SSE writer callback.

Or even simpler: just add the tool_result from within `safeTool.InvokableRun` by passing a callback through context.

Actually, the cleanest approach for now: modify `wrapTool` to accept a `resultCallback` parameter, and modify `safeTool` to call it when a tool executes. Then in QueryStream, pass an SSE writer callback.

But this injects SSE concerns into the tool layer...

**Decision:** Keep it simple. In `runAgentStream`, add a `createSafeTools` helper that creates tools with a result callback that sends `tool_result` SSE. Modify `safeTool` to accept an optional result callback.

**Step 2: Add resultCallback to safeTool**

Modify `safe_tool.go`:

```go
type safeTool struct {
    inner          tool.InvokableTool
    riskEval       *risk.Evaluator
    auditFn        func(...)
    info           *schema.ToolInfo
    resultCallback func(toolName, result string)  // NEW
}

// Add new constructor that accepts callback
func wrapToolWithCallback(
    t tool.BaseTool,
    riskEval *risk.Evaluator,
    auditFn func(...),
    resultCallback func(toolName, result string),
) (tool.BaseTool, error) {
    it, ok := t.(tool.InvokableTool)
    if !ok {
        return t, fmt.Errorf("tool is not InvokableTool")
    }
    info, err := it.Info(context.Background())
    if err != nil {
        return t, fmt.Errorf("tool Info failed: %w", err)
    }
    return &safeTool{
        inner: it, riskEval: riskEval, auditFn: auditFn, info: info,
        resultCallback: resultCallback,
    }, nil
}
```

Then in `InvokableRun`, after getting the result:
```go
if st.resultCallback != nil {
    st.resultCallback(st.info.Name, truncateString(result, 2000))
}
```

---

### Task 3: Rewrite QueryStream

**Files:**
- Modify: `internal/agent/service/service.go` — QueryStream method (lines 403-492)

**Step 1: Replace textReActLoop logic with runAgentStream**

The current implementation (simplified):
```go
// Old
tools := s.toolMgr.GetAllTools(ctx)
loop := newTextReActLoop(chatModel, tools, 10, s.logger)
loop.withGuard(s.riskEval, req.SessionID, uint(userID), "", auditFn)
finalContent, err := loop.Stream(ctx, messages, writer, s.writeSSEEvent)
```

New implementation:
```go
// 获取所有工具并包装安全层
rawTools := s.toolMgr.GetAllTools(ctx)
safeTools := make([]tool.BaseTool, 0, len(rawTools))
for _, t := range rawTools {
    wt, err := wrapToolWithCallback(t, s.riskEval,
        func(ctx context.Context, action, toolName, reason string, riskLevel agentmodel.RiskLevel, allowed bool, args string, result string) {
            s.auditEvent(ctx, action, toolName, reason, riskLevel, allowed, args, result, req.SessionID, userID, "")
        },
        func(toolName, result string) {
            s.writeSSEEvent(writer, "tool_result", map[string]interface{}{
                "name":   toolName,
                "result": result,
            })
        },
    )
    if err != nil {
        s.logger.Warn("wrap tool failed, using original", zap.Error(err))
        safeTools = append(safeTools, t)
        continue
    }
    safeTools = append(safeTools, wt)
}

// 使用 react.Agent 流式执行（标准 function calling）
finalContent, err := runAgentStream(
    ctx, chatModel, safeTools, messages, writer, s.writeSSEEvent,
    s.logger, req.SessionID, userID, personaPrompt,
    func(ctx context.Context, action, toolName, reason string, riskLevel agentmodel.RiskLevel, allowed bool, args string, result string) {
        s.auditEvent(ctx, action, toolName, reason, riskLevel, allowed, args, result, req.SessionID, userID, "")
    },
)
```

**Step 2: Remove manual done/audit/persist — runAgentStream handles done, caller handles audit/persist**

The `runAgentStream` sends the `done` event. QueryStream still needs to audit and persist after it returns.

---

### Task 4: Rewrite QueryStreamWithPipeline

**Files:**
- Modify: `internal/agent/service/service.go` — QueryStreamWithPipeline method (lines 494-615)

**Step 1: Same pattern as QueryStream, with enrichedPrompt**

The only difference from QueryStream is:
- Pipeline intent analysis + memory enrichment
- `enrichedPrompt` instead of `personaPrompt`

Replace the textReActLoop block (lines 574-582) with the same react.Agent pattern + runAgentStream, using `enrichedPrompt`.

---

### Task 5: Remove textReActLoop and Related Code

**Files:**
- Delete: `internal/agent/service/text_react.go`

**Step 1: Verify no other references**

Check that no other files import or reference anything from text_react.go.

**Step 2: Delete text_react.go**

```bash
git rm internal/agent/service/text_react.go
```

**Step 3: Remove import of unused packages**

Check service.go imports — if regexp, strings, etc. are now only used by text_react.go, remove them.

---

### Task 6: Fix safe_tool.go for Optional Result Callback

**Files:**
- Modify: `internal/agent/service/safe_tool.go`

**Step 1: Add resultCallback field and wrapToolWithCallback constructor**

Add to struct, add new constructor, modify InvokableRun to call callback on success.

**Step 2: Keep backward compatibility**

Keep existing `wrapTool()` unchanged (used by sync path Query/QueryWithPipeline).

---

### Task 7: Remove Debug Logging

**Files:**
- Modify: `internal/agent/service/service.go` — createChatModel (lines 117-144)
- Modify: `pkg/di/agent.go` — ProvideAgentService (lines 90-112)
- Modify: `pkg/di/viper.go` — InitViper (lines 86-89)
- Modify: `main.go` — run() (lines 30-35)

**Purpose:** Remove the debug logging added during troubleshooting.

**Step 1: Revert debug changes**

Revert the 4 debug log blocks added earlier.

---

### Task 8: Compile and Verify

**Step 1: Build**

```bash
cd /home/van/github/van/VanPanelBackend && go build ./...
```

Expected: Success

**Step 2: Run all tests**

```bash
go test ./internal/agent/... -v
```

**Step 3: Manual verification of streaming**

Start the server and test with a tool-using query via the streaming endpoint.
