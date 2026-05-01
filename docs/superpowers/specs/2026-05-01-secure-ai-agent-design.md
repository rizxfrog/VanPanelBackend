# VanPanel Secure AI Agent Design

## Goal

Add a secure AI operations assistant to VanPanel for the "Kylin OS secure intelligent operations Agent" scenario. The first version supports diagnosis, recommendations, and user-approved low-risk actions. It must not allow the model to directly execute arbitrary commands or modify critical system resources without policy checks and explicit approval.

## Architecture

The system is split across three existing products:

```text
VanPanelWebUI
  -> Smart assistant UI, conversation, streaming output, approval confirmation

VanPanelBackend
  -> Agent gateway, authentication, authorization, risk guard, approval, audit, controlled tool execution

VanAgentByPy
  -> Agent orchestration, intent classification, LLM/RAG/MCP reasoning, tool planning
```

This keeps security-sensitive platform control in Go while preserving Python for AI, RAG, MCP, and agent orchestration.

## First-Version Scope

In scope:

- WebUI smart assistant page.
- Agent Gateway in VanPanelBackend.
- Agent Orchestrator in VanAgentByPy.
- Intent Classifier.
- Risk Guard.
- Tool Router.
- Audit Trail.
- Read-only tools.
- User-approved low-risk actions.

Out of scope for the first version:

- Fully autonomous execution.
- Arbitrary shell command execution.
- Direct model access to SSH terminals.
- Direct model access to unrestricted filesystem writes.
- High-risk remediation actions.
- Fine-tuned model training.
- Multi-agent parallel execution.

## Request Flow

```text
User request
  -> VanPanelWebUI assistant page
  -> VanPanelBackend Agent Gateway
  -> VanAgentByPy Agent Orchestrator
  -> Intent Classifier
  -> Tool Router proposes tool calls
  -> VanPanelBackend Risk Guard checks tool calls
  -> Safe read-only tools run immediately
  -> Low-risk write tools create approval requests
  -> User confirms in WebUI
  -> VanPanelBackend executes controlled tool
  -> Audit Trail records every stage
  -> Final response is returned to WebUI
```

## Component Responsibilities

### VanPanelWebUI

Add a system assistant page:

```text
/system/agent
```

Responsibilities:

- Conversation UI.
- Streaming or incremental response rendering.
- Show detected intent and risk level.
- Show tool calls and evidence.
- Show approval prompts for low-risk write actions.
- Show audit timeline for the current request.

The WebUI must only call VanPanelBackend. It must not call VanAgentByPy directly.

### VanPanelBackend

Add a new backend module:

```text
internal/agent/model
internal/agent/service
internal/agent/api
internal/agent/risk
internal/agent/audit
internal/agent/tools
```

Responsibilities:

- Authenticate every assistant request.
- Proxy safe AI requests to VanAgentByPy.
- Enforce tool allowlists and user permissions.
- Run Risk Guard before any tool execution.
- Create approval requests for low-risk write actions.
- Execute approved low-risk actions through controlled tool adapters.
- Record audit events.
- Normalize errors and redact secrets.

### VanAgentByPy

Extend the existing Python AIOps service with:

```text
app/core/agents/orchestrator.py
app/core/agents/intent_classifier.py
app/core/agents/tool_router.py
app/core/agents/risk_context.py
```

Responsibilities:

- Classify user intent.
- Decide whether RAG, MCP, tools, or RCA are needed.
- Produce a structured tool plan instead of directly executing unsafe actions.
- Call safe remote tools through VanPanelBackend when required.
- Generate final natural language responses using tool results and audit context.

VanAgentByPy can execute its own read-only reasoning and RAG logic, but platform tool execution should be routed through VanPanelBackend for permission and audit.

## Agent Pipeline

The logical pipeline is:

```text
Agent Orchestrator
  -> Intent Classifier
  -> Risk Guard
  -> Tool Router
  -> MCP/Tools
  -> Audit Trail
```

### Agent Orchestrator

Coordinates the full request lifecycle:

- Receives normalized user query.
- Loads conversation context.
- Calls intent classifier.
- Requests tool plans.
- Sends tool execution requests to VanPanelBackend.
- Synthesizes final response.

### Intent Classifier

Classifies user requests into categories:

- `diagnosis`: investigate a problem.
- `explanation`: explain system state or command meaning.
- `recommendation`: suggest next steps.
- `read_only_operation`: query logs, metrics, files, processes, containers.
- `low_risk_action`: restart whitelisted service, compress logs, move file to trash.
- `high_risk_action`: destructive or privileged operation.
- `unknown`: cannot classify safely.

Every classification must include confidence and reason.

### Risk Guard

Risk Guard is enforced in VanPanelBackend and may also be mirrored in VanAgentByPy for early feedback.

Risk levels:

- `safe`: read-only tool call.
- `low`: reversible or bounded write action requiring user confirmation.
- `medium`: requires elevated approval and is out of scope for first version.
- `high`: blocked.

Risk Guard checks:

- Tool name is registered.
- Tool is allowed for the current user.
- Parameters match schema.
- Target path, service, container, or query is within allowed scope.
- Action is not destructive.
- Request does not contain prompt injection patterns.
- Action is consistent with classified intent.

### Tool Router

Maps intent to tool calls. It must return structured JSON plans:

```json
{
  "intent": "diagnosis",
  "riskLevel": "safe",
  "toolCalls": [
    {
      "tool": "disk.analyze",
      "parameters": { "path": "/" },
      "reason": "Check disk usage because user reported system cleanup need"
    }
  ]
}
```

The router must not return raw shell commands as executable actions. Command strings are allowed only as suggestions through `terminal.suggest`.

## Tool Catalog

First-version read-only tools:

```text
system.inspect       Read OS, uptime, CPU, memory, disk overview
process.list         List processes and high resource consumers
disk.analyze         Analyze disk usage for a bounded path
log.query            Query bounded log files or journal slices
file.scan            Scan metadata, size, file type, sensitivity markers
terminal.suggest     Suggest shell commands but do not execute them
container.inspect    Inspect containers, status, logs, stats
prometheus.query     Execute allowlisted PromQL templates
```

First-version low-risk write tools requiring approval:

```text
log.compress         Compress an approved log file
log.truncate         Truncate an approved ordinary log file
container.restart    Restart a non-critical container
service.restart      Restart a whitelisted service
file.move_to_trash   Move an approved file into a VanPanel trash directory
```

High-risk tools are not implemented in the first version.

## Blocked Actions

Risk Guard must block:

```text
rm -rf
direct deletion of system files
modifying /etc/passwd
modifying /etc/shadow
modifying /etc/sudoers
chmod 777
disk formatting
partition changes
firewall shutdown
database directory deletion
Docker volume deletion
arbitrary shell execution
privilege escalation commands
```

The first version can still explain why an action is unsafe and suggest a safer alternative.

## Approval Model

Low-risk write actions produce an approval request:

```json
{
  "approvalId": "uuid",
  "riskLevel": "low",
  "tool": "container.restart",
  "target": "nginx",
  "reason": "Container is unhealthy and restart is reversible",
  "parameters": { "containerId": "abc123" },
  "expiresAt": "2026-05-01T12:00:00Z"
}
```

Approval rules:

- Approval must be tied to user ID, session ID, and exact tool parameters.
- Expired approvals cannot be reused.
- Approved parameters cannot be changed after confirmation.
- Approval execution must re-run Risk Guard.
- Approval and execution must both be audited.

## Audit Trail

Audit events must record:

- Request received.
- User identity.
- Session ID.
- Intent classification.
- Tool plan.
- Risk Guard decision.
- Tool execution request.
- Approval created.
- Approval confirmed or rejected.
- Tool execution result.
- Final answer generated.

Audit records must not store secrets, tokens, private keys, or full sensitive file contents.

## API Design

VanPanelBackend APIs:

```text
POST /api/system/agent/sessions
POST /api/system/agent/query
GET  /api/system/agent/sessions/:id/events
POST /api/system/agent/approvals/:id/confirm
POST /api/system/agent/approvals/:id/reject
GET  /api/system/agent/audit/:requestId
GET  /api/system/agent/tools
```

VanAgentByPy APIs used by VanPanelBackend:

```text
POST /api/v1/agent/plan
POST /api/v1/agent/respond
POST /api/v1/assistant/query
GET  /api/v1/assistant/ready
GET  /tools
POST /tools/execute
```

If VanAgentByPy does not yet expose `/api/v1/agent/plan` and `/api/v1/agent/respond`, they should be added rather than overloading the existing assistant query endpoint for all platform control decisions.

## Error Handling

Expected errors:

- VanAgentByPy unavailable.
- LLM unavailable.
- Tool not found.
- Tool parameters invalid.
- Risk Guard blocked action.
- Approval expired.
- User lacks permission.
- Tool execution failed.
- Tool result exceeds output limit.

Errors should be returned as structured assistant events so the WebUI can show a timeline rather than a generic failure toast.

## Deployment Model

Recommended deployment:

```text
VanPanelWebUI
VanPanelBackend
VanAgentByPy main API service
VanAgentByPy MCP service
Redis for cache/vector/audit if needed
Prometheus if metrics queries are enabled
```

On Kylin/LoongArch, VanAgentByPy dependencies must be validated separately because Python ML and vector dependencies may have architecture-specific wheels or build requirements.

## Testing

Backend tests:

- Risk Guard blocks high-risk patterns.
- Risk Guard allows safe read-only tools.
- Approval cannot change parameters.
- Approval expires.
- Audit events are written in order.
- Agent Gateway handles VanAgentByPy unavailable.
- Tool schema validation rejects invalid parameters.

Python tests:

- Intent classifier returns stable categories.
- Tool router returns structured plans.
- Orchestrator handles safe, low-risk, and blocked flows.
- Prompt injection examples are classified as risky.

Frontend tests:

- Assistant response event rendering.
- Approval prompt rendering.
- Approval confirm/reject calls.
- Tool timeline display.

Manual verification:

- Ask "帮我分析磁盘为什么满了" and confirm only read-only tools run.
- Ask "帮我清理系统垃圾" and confirm low-risk approval is created before write action.
- Ask "执行 rm -rf /" and confirm Risk Guard blocks it.
- Ask for a terminal command and confirm only a suggestion is returned.

