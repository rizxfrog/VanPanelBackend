# Architecture Design Philosophy

**Date**: 2026-06-07
**Purpose**: Defines the three core architectural pillars that differentiate this intelligent ops Agent from general-purpose AI assistants. Serves as a reference for all future design and implementation decisions.

---

## Overview

This project is an **Intelligent Operations Agent** targeting KylinOS (麒麟操作系统). Unlike general-purpose AI coding assistants, the architecture is built around three foundational design principles:

1. **Security by Design** — No operation executes without independent review
2. **Persistent Memory** — Multi-level memory spanning sessions and knowledge domains
3. **Layered Extensibility** — Three distinct extension tiers targeting different user roles

These principles are not optional features — they are the core differentiator. Every architectural decision must be evaluated against whether it strengthens or weakens these pillars.

---

## Pillar 1: Security by Design

### Principle

**No agent-generated operation reaches the execution layer without passing through at least two independent review gates.**

### Architecture

```
Agent ToolCall → SPI Rules → Rule Engine → Auditor Model → Approval Decider → Execute
                   (Plugin)    (Pattern)     (LLM, independent)  (Human-in-loop)
```

| Gate | Type | Latency | Scope |
|------|------|---------|-------|
| SPI Custom Rules | User-defined checks | ~0ms | Extensible by platform developers |
| Rule Engine | Regex + path blacklists | ~0ms | Known dangerous patterns (rm -rf, mkfs, /etc access) |
| Auditor Model | Independent smaller LLM | ~500-2000ms | Semantic analysis: intent alignment, injection, scope reasonability |
| Approval Decider | Human-in-the-loop queue | Varies | Operations requiring explicit admin approval |

### Key Constraints

- The auditor model MUST be a **different model** than the main agent model — using the same model for self-review creates blind spots
- The auditor system prompt MUST be **immutable** and include explicit anti-injection hardening
- Every decision (pass / block / escalate) is **audit-logged** with full traceability
- Failure of any review layer defaults to **deny** (fail-closed), except when the auditor is unavailable — in that case, low-risk operations fall back to human approval rather than blocking

### Audit Trail Requirements

Each operation must produce a trace with these event types:

| Event | Records |
|-------|---------|
| `agent.receive` | Raw user input, session ID, timestamp |
| `tool.evaluate` | Tool name, arguments, risk level, evaluator verdict |
| `tool.execute` | Tool name, arguments, exit code, output (truncated) |
| `tool.blocked` | Reason for blocking, which gate stopped it |
| `agent.complete` | Final answer, total steps, duration |

---

## Pillar 2: Persistent Memory

### Principle

**Agent context persists across sessions. Past interactions inform future decisions. External knowledge augments reasoning without polluting the model.**

### Three-Level Memory Hierarchy

| Level | Name | Store | Lifetime | Retrieval |
|-------|------|-------|----------|-----------|
| **L1** | Working Memory | MySQL (`agent_messages`) | Current session | Sequential history (last N messages) |
| **L2** | Long-Term Memory | MySQL (`user_memories`) + vector index | Cross-session, 30-day TTL | Semantic similarity + keyword match |
| **L3** | RAG Knowledge Base | External vector store | Independent lifecycle | Dense retrieval + re-rank |

### L1: Working Memory

- **What**: Current session's message history
- **Flow**: User message → DB write → loaded into ReAct context as `[]*schema.Message`
- **Constraints**: Capped at `MaxHistory` (default 20). Long sessions require summary compression to avoid context window overflow.

### L2: Long-Term Memory

- **What**: Cross-session knowledge extracted and persisted
- **Content types**:
  - `preference` — "This user prefers concise responses" / "Frequently investigates disk issues"
  - `pattern` — "Past N log-cleanup operations targeted /var/log/nginx/*"
  - `knowledge` — "PID 1234 was identified as a memory leak"
  - `solution` — "Disk full → lsof find large files → cleanup" verified chain
- **Write triggers**:
  - Session end: LLM asynchronously extracts key information
  - Approved dangerous operations: worth remembering for future reference
- **Retrieval**: Inserted into the Pipeline's context enrichment stage (stage ②) before agent reasoning

### L3: RAG Knowledge Base

- **What**: External domain knowledge (ops runbooks, KylinOS manuals, historical incident patterns)
- **Configuration**: `rag.enabled` flag, default `false`. Does not block core functionality.
- **Integration**: Retrieval results merge with L2 memories during stage ② enrichment

### Memory Lifecycle

```
Session Active → L1 messages (real-time)
Session Ends   → L2 extraction (async, LLM-summarized)
Time passes    → L2 eviction (30-day TTL + importance decay)
RAG content    → Externally managed, agent reads only
```

---

## Pillar 3: Layered Extensibility

### Principle

**Extension points target different user roles at different abstraction levels. No single extension mechanism tries to serve everyone.**

### Three Extension Tiers

| Tier | Name | Target Role | Mechanism | Examples |
|------|------|-------------|-----------|----------|
| **L1** | MCP Plugins | Third-party developers | Independent process, stdio/SSE/HTTP | Custom monitoring tools, database inspectors |
| **L2** | Skill Orchestration | Ops engineers / domain experts | YAML declarative workflow | "One-click health check", "Log analysis pipeline" |
| **L3** | SPI Extension Points | Platform developers | Go interfaces | Custom intent analyzers, guard rules, memory backends |

### L1: MCP Plugins

- Follows the [Model Context Protocol](https://modelcontextprotocol.io/) specification
- Two transport modes: **Local** (stdio subprocess) and **Remote** (SSE/HTTP with auth)
- Plugin marketplace: upload (manifest + binary, SHA-256 verification), install, toggle, uninstall
- Tools discovered via MCP are presented to the agent transparently — identical to builtin tools

### L2: Skill Orchestration

- YAML-based declarative workflow language
- A Skill is a **pre-composed sequence of tool calls** with optional LLM aggregation
- No runtime code execution — the Skill executor reuses the existing `ToolManager` scheduler
- Risk level declared per Skill (applies to all steps uniformly)

```yaml
skill:
  name: health-check
  display: "System Health Check"
  risk: low
  steps:
    - tool: proc.ps
      params: {top: 10}
    - tool: disk.df
    - tool: sys.free
    - tool: net.ss
      params: {state: listening}
  aggregate:
    type: llm_summary
    prompt: "Generate a health report from the following data..."
```

### L3: SPI Extension Points

Go interfaces that allow platform developers to replace default implementations:

| Interface | Stage | Purpose |
|-----------|-------|---------|
| `IntentAnalyzer` | ① | Custom injection detection and intent classification |
| `GuardRule` | ④ | Custom safety rules injected into GuardChain (with priority ordering) |
| `MemoryProvider` | ② | Custom memory storage backends (default: MySQL keyword, upgrade: vector DB) |
| `Notifier` | — | Custom alert notification channels |
| `AuditWriter` | ⑥ | Custom audit log format and storage |
| `ToolResolver` | ③ | Custom tool discovery and resolution |

### Remote Agent Nodes

Three deployment modes for operating on remote servers:

| Mode | Mechanism | Capability | Use Case |
|------|-----------|------------|----------|
| MCP Remote Agent | Lightweight Go binary (~5MB) on target, exposes tools via SSE/WS | Full perception + execution | Recommended |
| SSH Direct | Reuses `pkg/ssh`, command dispatch | Execution only | Legacy / fallback |
| Database MCP | MCP plugin connecting to remote MySQL/Redis/Prometheus | Data query | Database inspection |

---

## Integration: The 6-Stage Pipeline

All three pillars converge in a unified request processing pipeline:

```
User Input
    │
① Intent Analysis + Injection Detection    ← Pillar 1 (first filter) + Pillar 3 (SPI: IntentAnalyzer)
    │
② Context Enrichment                        ← Pillar 2 (L1+L2+L3 retrieval)
    │
③ Agent Reasoning (ReAct)                   ← Core LLM (unchanged from base agent)
    │
④ Guard Security Review                     ← Pillar 1 (Rule Engine + Auditor) + Pillar 3 (SPI: GuardRule)
    │
⑤ Least-Privilege Execution                 ← Pillar 1 (non-root) + Pillar 3 (MCP / Remote Agent)
    │
⑥ Audit Logging                             ← Pillar 1 (full trace)
```

### Surgical Integration

Pipeline stages wrap existing code without modifying it:

- `Query()` and `QueryStream()` remain available as the **fast path** (no pipeline overhead)
- `QueryWithPipeline()` and `QueryStreamWithPipeline()` add the full 6-stage pipeline
- Pipeline components are **nil-safe** — if a component is not configured, the stage gracefully no-ops
- All extension points accept `context.Context` and return structured error types

---

## Design Constraints

1. **No operation executes without traceability** — every tool call must produce an audit event
2. **Failures are non-fatal** — a pipeline stage failure (e.g., auditor timeout) must not crash the agent. Degrade gracefully.
3. **Extensions never bypass security** — MCP tools, Skills, and SPI rules all pass through the same GuardChain
4. **Configuration over code** — RAG, auditor model, memory backends are all configurable flags. The system must work with all of them disabled (degraded mode).
5. **Backward compatibility is mandatory** — existing API endpoints and their request/response formats must not change
