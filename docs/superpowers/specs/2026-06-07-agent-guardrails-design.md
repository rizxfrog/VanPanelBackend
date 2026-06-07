# Agent 安全护栏与审计链路 — 实现规格

**日期**: 2026-06-07  
**范围**: P0 安全校验器接入 + P1 审计链路接入  
**目标文件**: `internal/agent/service/service.go`, `internal/agent/audit/memory.go`

---

## 背景

Agent 模块 (`internal/agent/`) 已完成框架搭建：18 个内置 MCP 工具、ReAct 流式推理（textReActLoop）、RiskEvaluator（配置化正则风险检测）、AuditStore（内存审计存储）。但存在两条关键链路断裂：

1. **RiskEvaluator 已注入 AgentService 但从未被调用** — LLM 生成的工具指令直接执行，安全护栏形同虚设
2. **AuditStore 已存在但从未被写入** — 推理链路 "接收→感知→推理→校验→执行" 无迹可寻

本规格定义两条拦截链的接入方式。

---

## 设计目标

### 安全校验（P0）

在 tool 执行路径上插入 RiskEvaluator，用配置化的正则规则对 LLM 生成的工具名称和参数进行"二次过滤"。

**策略**: 阻断 + 告知 LLM — 高危指令阻止执行，返回 `[安全拦截]` 错误信息给 LLM 让其重新推理。

### 审计链路（P1）

全量详记 5 类审计事件，完整覆盖"接收指令 → 感知环境 → 推理决策 → 安全校验 → 执行结果"闭环。

---

## 架构变更

### 修改文件

| 文件 | 改动类型 | 说明 |
|------|---------|------|
| `internal/agent/service/service.go` | 修改 | 在 Query() 和 QueryStream() 中插入 risk + audit 调用 |
| `internal/agent/audit/memory.go` | 修改 | 增加事件类型常量，增强 API |

### 不改文件

| 文件 | 原因 |
|------|------|
| `internal/agent/service/text_react.go` | 工具执行在 service.go 中通过回调完成 |
| `internal/agent/risk/evaluator.go` | 已完整，只需调用 |
| `internal/agent/tool/builtin/` | 不需要改动 |

---

## 拦截流程

```
LLM 生成 tool_call
       ↓
  【审计】audit.receive  (仅在首轮记录)
       ↓
  【校验】riskEval.Evaluate(toolName, args)
       ↓
    ├─ safe  → 【审计】tool.evaluate(safe) → 执行工具 → 【审计】tool.execute(ok)
    │                                               ↓
    │                                        返回 result 给 LLM
    │
    ├─ low   → 【审计】tool.evaluate(low, requires_approval) → 执行工具 → 【审计】tool.execute(ok)
    │                                               ↓
    │                                        返回 result 给 LLM
    │                                        （审批流留到 P2）
    │
    └─ high  → 【审计】tool.blocked(reason)
                     ↓
              不执行工具，返回 "[安全拦截] 原因: xxx" 给 LLM
                     ↓
              LLM 重新推理
```

---

## 风险分级策略

| 风险等级 | 行为 | 审计 action | 关键字段 |
|---------|------|-------------|---------|
| `safe` | 直接执行 | `tool.execute` | `allowed: true, risk: safe` |
| `low` | 执行 + 标记需审批 | `tool.execute` | `allowed: true, risk: low, requires_approval: true` |
| `high` | 阻断不执行 | `tool.blocked` | `allowed: false, risk: high, reason: "..."` |

被阻断的 tool 返回给 LLM 的格式:

```
[安全拦截] 操作被安全策略阻止
原因: 命令包含高危模式: rm -rf /
工具: shell.exec
建议: 请尝试更安全的替代方案
```

---

## 审计事件定义

| 时机 | action | 内容 |
|------|--------|------|
| 接收用户消息 | `agent.receive` | session_id, user_id, question |
| 工具调用前评估 | `tool.evaluate` | tool_name, args, risk_level, reason, allowed |
| 工具执行后 | `tool.execute` | tool_name, args, result (截断 2000 字), duration_ms |
| 工具被拦截 | `tool.blocked` | tool_name, args, risk_level, reason |
| 对话结束 | `agent.complete` | session_id, total_steps, answer (截断), duration_ms |

每条事件包含: `id`, `session_id`, `user_id`, `username`, `action`, `tool_name`, `risk`, `allowed`, `reason`, `metadata`, `created_at`。

### 存储

- 写入 `MemoryStore`（内存），上限 2000 条（FIFO 淘汰）
- 同时写入 DB（`AuditDAO.CreateAuditEvent`），用于持久化和查询
- TTL 不重要（审计日志长期保留）

---

## 实现步骤

### Step 1: 增强 AuditStore

`internal/agent/audit/memory.go`:

- 增加事件类型常量 (`ActionReceive`, `ActionEvaluate`, `ActionExecute`, `ActionBlocked`, `ActionComplete`)
- 暴露 `Append(event *model.AuditEvent)` 方法
- 增加 `DAO` 依赖，Write-Through 写 DB

### Step 2: 增强 AgentService

`internal/agent/service/service.go`:

- `agentService` 结构体增加 `auditStore` 字段
- 改造 `NewAgentService` 构造函数，接收 AuditStore
- **Query()**:
  1. 开始时写 `agent.receive`
  2. 在 sync react agent 的工具调用回调中：evaluate → execute/block → audit
  3. 结束时写 `agent.complete`
- **QueryStream()**:
  1. 开始时写 `agent.receive`
  2. 在 `textReActLoop.Stream()` 的 tool execution 阶段：
     - 每个 tool_call 前 evaluate
     - safe/low 执行后 audit
     - high 返回拦截信息并 audit
  3. 结束时写 `agent.complete`

### Step 3: 更新 DI 连线

`pkg/di/agent.go`:

- `ProvideAgentService` 增加 `auditStore` 参数
- 确保 AuditStore DAO 注入

### Step 4: 验证

- `go build ./...` 编译通过
- 手动测试：发送 "帮我清理系统垃圾" → 确认风险拦截生效
- 检查审计日志：确认 5 类事件完整记录

---

## 未来扩展（非本次范围）

- **P2 最小权限执行**: 审批流 UI + 受限用户执行
- **LLM 注入检测**: 在 user message 阶段增加 prompt injection 检测
- **审计日志 UI**: 前端审计日志查看/搜索/导出页面
- **审批流 UI**: 前端风险审批卡片（原 Vben Admin 已有安全审批组件可复用）
