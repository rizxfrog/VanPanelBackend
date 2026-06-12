# VanPanel Agent Tool Card SSE Enhancement Spec

## Goal

Enhance the existing Agent SSE streaming protocol so the Lit frontend can render rich Tool Cards — displaying each tool invocation with its arguments, execution status (running/success/error), result output, and parameter descriptions — matching the visual quality of OpenClaw's tool call display.

## Current State

### SSE Events (before change)

```
event: delta        → {"role":"assistant","content":"..."}
event: tool_call    → {"id":"call_abc","name":"net.lsof","arguments":"{\"port\":8080}"}
event: tool_result  → {"name":"net.lsof","result":"COMMAND  PID  ..."}
event: done         → {"session_id":"s1","result":{"answer":"..."}}
event: error        → {"error":"..."}
```

### Problems

1. **No `id` in `tool_result`**: When the same tool is called multiple times, the UI can't match result to call.
2. **No `status` in `tool_result`**: The UI can't distinguish success from error — error messages are stored in `result`.
3. **No param schema in tools API**: `GET /api/system/agent/tools` returns only `name` + `description`, Tool Card can't show argument definitions.
4. **Server-side truncation**: `safe_tool.go` truncates results to 2000 chars — UI should handle truncation instead.

### Current Tools API

```
GET /api/system/agent/tools
→ [{"name":"net.lsof","description":"列出端口占用和网络连接"}]
```

## Target State

### Enhanced SSE Events

```
event: delta        → {"role":"assistant","content":"..."}
event: tool_call    → {"id":"call_abc","name":"net.lsof","arguments":"{\"port\":8080}"}
event: tool_result  → {"id":"call_abc","name":"net.lsof","status":"success","result":"COMMAND  PID  ..."}
event: done         → {"session_id":"s1","result":{"answer":"..."}}
event: error        → {"error":"..."}
```

### Enhanced Tools API

```
GET /api/system/agent/tools
→ [{"name":"net.lsof","description":"列出端口占用和网络连接","params":{...JSON Schema...}}]
```

### Tool Card UI State Machine

```
tool_call received (id=call_abc, name="net.lsof", arguments)
  → Create Card: status=running, show name + parsed arguments
  
tool_result received (id=call_abc, name="net.lsof", status="success", result="...")
  → Match by id, update Card: status=success, populate result body
  
tool_result received (id=call_abc, status="error", result="error message")
  → Match by id, update Card: status=error, show error in red

done received
  → All Cards are in terminal state
```

## Changes Required

### File: `internal/agent/service/safe_tool.go`

| Change | Description |
|--------|-------------|
| `resultCallback` type | Change from `func(toolName, result string)` to `func(toolCallID, toolName, result, status string)` |
| `wrapToolWithCallback` signature | Add `toolCallID` storage via context or closure |
| `InvokableRun`: error path | Call `resultCallback` with `status="error"` on tool execution failure |
| `InvokableRun`: success path | Call `resultCallback` with `status="success"` |
| Remove `truncateString` | Only keep truncation for `auditFn` callback, not for `resultCallback` |

### File: `internal/agent/service/service.go`

| Line(s) | Change |
|---------|--------|
| 477-481 | `tool_result` SSE: add `"id"` and `"status"` fields |
| 608-612 | Same change for pipeline-enhanced stream path |
| 800-803 | `ListTools`: add `"params": info.ParamsOneOf` |

### File: `internal/agent/service/agent_stream.go`

No structural changes needed. The `safeTool` already intercepts `InvokableRun`. The `tool_call` ID is available in `msg.ToolCalls[].ID` — this needs to be passed through to the result callback.

### File: `main.go`

```go
//go:embed webui/dist/*
var webuiFS embed.FS

// In router setup:
// /  → redirect /ui/
// /ui/* → static files from webuiFS
// /login → login page
// * → index.html (SPA fallback for /ui/* routes)
```

## Files Not Touched

- All other Agent service files (chat, session, config services)
- All Agent API handler routes (unchanged)
- All Agent DAO layer
- All K8s/Prometheus/Workorder/Tree/Files APIs and services
- All middleware (JWT, Casbin, audit, CORS)
- DI/wire configuration (no new dependencies)
