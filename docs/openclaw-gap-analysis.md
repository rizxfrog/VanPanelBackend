# VanPanelBackend vs OpenClaw 功能差距分析报告

> 生成日期：2026-06-20（v8 — 2026-06-24 更新：聊天增强+会话管理+Agent管理+工具系统+配置管理+模型管理+Cron 定时任务已完成）

## 概述

本项目的目标是用 Go 后端 + 内嵌 Web UI 替代 OpenClaw 的 Node.js 网关 + 前端。前端（`webui/`）是 OpenClaw Control UI 的 Lit 实现，**23 个页面全部已实现**。后端 Gateway RPC 层（`internal/gateway/rpc/`）中**约 50% 的方法仍是桩实现**（2026-06-24 已完成聊天增强、会话管理、Agent管理、工具系统、配置管理、模型管理、Cron 定时任务七大模块）。此外，后端 Agent 子系统内部存在多个关键缺陷（记忆系统死代码、LLM 提供商硬编码、数据库表缺失迁移等），即使 Gateway 桥接完成，这些问题仍会导致功能不可用。

---

## 架构关系

```
┌──────────────────────────────────────────────┐
│  webui (Lit SPA)                             │
│  ├── gateway.ts  ── WebSocket RPC ──────────┐│
│  ├── mcp-manager.ts ── fetch /api/* ────┐   ││
│  ├── control-ui-bootstrap.ts ── fetch ──┤   ││
│  ├── app-channels.ts ── fetch ──────────┤   ││
│  └── chat media ── fetch ───────────────┤   ││
└─────────────────────────────────────────┼───┼┘
                                          │   │
         WebSocket (v4 protocol)          │   │ HTTP REST
                  │                       │   │
                  ▼                       ▼   ▼
┌─────────────────────────────────────────────────┐
│  Go Backend (:8889)                             │
│  ├── internal/gateway/ (WebSocket RPC)          │
│  │   └── rpc/ (28 files, ~55% stub)            │
│  ├── /api/system/agent/ (REST, 37 endpoints)    │
│  ├── /api/files/ (REST, 21 endpoints)           │
│  ├── /api/user/ /api/role/ (REST, 27 endpoints) │
│  └── 其他 REST 路由 (23 endpoints)              │
└─────────────────────────────────────────────────┘
         │                        │
         ▼                        ▼
┌─────────────────────┐  ┌──────────────────┐
│  internal/agent/    │  │  其他 domain     │
│  18 sub-packages    │  │  k8s/prometheus/ │
│  (大部分已实现)     │  │  workorder/tree/ │
│  ⚠ 但有内部缺陷    │  │  system/files/   │
└─────────────────────┘  └──────────────────┘
```

**关键发现**：前端采用**双传输架构**——WebSocket RPC 用于实时通信，直接 HTTP fetch 用于 MCP 插件管理、引导配置、Nostr 频道、聊天媒体等。后端同时存在完整的 REST API（108 个端点）和 Gateway WebSocket RPC，但前端主要使用 Gateway RPC，REST API 仅用于 MCP 管理等少数场景。

---

## 详细差距分析

### 一、已完全可用的功能

| 功能域 | Gateway 方法 | 状态 |
|--------|-------------|------|
| WebSocket 连接/握手 | `connect` | ✅ 完整实现 |
| 聊天发送（流式） | `chat.send` | ✅ 完整实现，通过 AgentService.QueryStream() |
| 聊天历史 | `chat.history` | ✅ 完整实现 |
| 会话 CRUD | `sessions.list/create/delete/patch` | ✅ 完整实现 |
| 认证（token/password） | `auth` | ✅ 完整实现 |
| 在线状态追踪 | `presence` | ✅ 完整实现 |
| 健康监控 | `health_state` | ✅ 完整实现 |
| 心跳 | `tick` | ✅ 完整实现 |
| MCP 插件管理（REST） | `fetch /api/system/agent/hub/*` | ✅ REST 端点已实现 |
| 远程 MCP 配置（REST） | `fetch /api/system/agent/remote-mcps/*` | ✅ REST 端点已实现 |
| 内置工具管理（REST） | `fetch /api/system/agent/builtin-tools/*` | ✅ REST 端点已实现 |
| Agent 管理 | `agents.list/create/update/delete` | ✅ DB CRUD + 文件系统存储 |
| 工具系统 | `tools.catalog/effective/invoke` | ✅ 动态工具目录 + 实际执行 |
| 配置管理 | `config.get/set/apply/patch/schema/schema.lookup` | ✅ DB 运行时配置 + YAML 默认值回退 |
| 模型管理 | `models.list/authStatus/authLogout` | ✅ AgentService.GetModelCatalog 桥接 + API key 授权状态 |
| Cron 定时任务 | `cron.status/list/get/add/update/remove/run/runs` | ✅ 新增 internal/cron/ 子系统 + 内存调度 + Asynq 手动执行 |

### 二、前端已实现但后端为 Stub 的功能

#### 1. 聊天增强

| Gateway 方法 | 前端调用方 | 后端状态 | 影响 |
|-------------|-----------|---------|------|
| `chat.abort` | chat controller | ✅ 已实现 | RunTracker 取消运行上下文 |
| `chat.metadata` | chat settings | ✅ 已实现 | 通过 AgentService.GetModelCatalog() |
| `chat.message.get` | - | ✅ 已实现 | 通过 AgentService.GetMessage()，支持 MaxChars 截断 |

#### 2. 会话管理

| Gateway 方法 | 前端调用方 | 后端状态 | 影响 |
|-------------|-----------|---------|------|
| `sessions.subscribe` | sessions controller | ✅ 已实现 | SubscriptionHub 追踪订阅 |
| `sessions.messages.subscribe` | sessions controller | ✅ 已实现 | 按 sessionKey 订阅消息推送 |
| `sessions.messages.unsubscribe` | sessions controller | ✅ 已实现 | 按 sessionKey 取消订阅 |
| `sessions.reset` | - | ✅ 已实现 | AgentService.ResetSession() 清除会话消息 |
| `sessions.compact` | - | ✅ 已实现 | AgentService.CompactSession() 保留最近 N 条 |
| `sessions.compaction.*` (5个) | - | ✅ 已实现 | 内存存储：列表/详情/快照/恢复 |
| `sessions.preview` | - | ✅ 已实现 | 返回最近 3 条消息摘要 |
| `sessions.describe` | - | ✅ 已实现 | 返回会话元数据（消息数/状态/模型） |
| `sessions.changed 广播` | sessions controller | ✅ 已实现 | 创建/删除/修改/重置/压缩/恢复后推送 |

#### 3. Agent 管理

| Gateway 方法 | 前端调用方 | 后端状态 | 影响 |
|-------------|-----------|---------|------|
| `agents.list` | agents controller | ✅ 已实现 | DB 持久化 + 首次调用自动种入默认 main agent |
| `agents.create/update/delete` | - | ✅ 已实现 | 完整 CRUD，Persist 到 cl_agent_agents 表 |
| `agents.files.list/get/set` | agent-files controller | ✅ 已实现 | 文件系统存储于 data/agents/<agentId>/ |
| `agent.identity.get` | agent-identity controller | ✅ 已实现 | DB 查询返回 name + agentId |

#### 4. 工具系统

| Gateway 方法 | 前端调用方 | 后端状态 | 影响 |
|-------------|-----------|---------|------|
| `tools.catalog` | agents controller | ✅ 已实现 | 从 ToolManager 动态分组返回 23 个工具定义 |
| `tools.effective` | agents controller | ✅ 已实现 | 动态返回当前生效的工具列表 |
| `tools.invoke` | - | ✅ 已实现 | 通过 AgentService.InvokeTool() 实际执行内置/MCP 工具 |

#### 5. 配置管理

| Gateway 方法 | 前端调用方 | 后端状态 | 影响 |
|-------------|-----------|---------|------|
| `config.get/set/apply/patch/schema/schema.lookup` | config controller | ✅ 已实现 | DB 运行时配置覆盖 YAML 默认值；重启后自动从 DB 恢复 |

> 注：后端有 Agent 级别的 REST config 端点（`/api/system/agent/config/*`）和 DB 持久化 `ConfigService`。Gateway 层现在桥接到同一套 DB 配置，`config.get` 优先返回 DB 值，不存在时回退到 Viper/YAML 默认值，并标注 `source`。

#### 6. 模型管理

| Gateway 方法 | 后端状态 | 影响 |
|-------------|---------|------|
| `models.list` | ✅ 已实现 | 返回 `ModelCatalogEntry[]`，含 id/name/provider/alias/contextWindow/input |
| `models.authStatus` | ✅ 已实现 | 返回当前 LLM 提供商认证状态（依据内存中 `LLM.APIKey` 是否非空） |
| `models.authLogout` | ✅ 已实现 | 删除 `agent.llm.api_key` 的 DB 运行时覆盖并重载配置 |

> 注：`models.authLogout` 仅清除 DB 中的运行时覆盖值。如果 `.env` / YAML 中仍配置了 `AGENT_LLM_API_KEY`，重启或登出后仍会回退到该默认值并保持 `authorized=true`。

#### 7. Cron 定时任务

| Gateway 方法 | 后端状态 | 影响 |
|-------------|---------|------|
| `cron.status/list/get/add/update/remove/run/runs` | ✅ 已实现 | 可创建、编辑、删除、启停、手动运行并查看运行历史 |

> 注：新增 `internal/cron/` 子系统，包含 `cl_agent_cron_jobs` / `cl_agent_cron_runs` 表、DAO/Service、内存调度器（10s tick）及 Asynq 手动任务队列。`cron.run` 通过 Asynq 异步执行，`cron.runs` 返回运行日志。已用 agent-browser 验证：创建 systemEvent 任务、启停切换、手动运行后运行历史正确显示。

#### 8. 技能系统

| Gateway 方法 | 后端状态 | 影响 |
|-------------|---------|------|
| `skills.status/search/detail/install/update` | ❌ Stubs | 无法使用技能系统 |
| `skills.proposals.*` (8个) | ❌ Stubs | 无法使用技能工作坊 |

> 注：后端有完整的 `internal/agent/skill/` 实现，但未桥接到 Gateway。

#### 9. 用量与计费

| Gateway 方法 | 后端状态 | 影响 |
|-------------|---------|------|
| `usage.status/cost` / `sessions.usage/timeseries/logs` | ❌ Stubs | 无法查看用量和费用 |

> 注：后端有完整的 `internal/agent/insight/` 分析引擎（含 9 种模型定价表），但未桥接到 Gateway。

#### 10. 频道/消息集成

| Gateway 方法 | 后端状态 | 影响 |
|-------------|---------|------|
| `channels.status/config.reload/config.save/logout` | ❌ Stubs | 无法配置消息频道 |

> 注：前端已实现 Discord/Telegram/Slack/WhatsApp/Nostr/Signal/iMessage/Google Chat 的配置 UI，但后端全部为桩。前端还通过 REST 调用 Nostr 频道的 profile 管理接口。

#### 11. 语音/对话

| Gateway 方法 | 后端状态 | 影响 |
|-------------|---------|------|
| `talk.start/stop/send` / `voicewake.*` (5个) / `tts.*` (3个) | ❌ 全部 Stub | 语音对话、唤醒、TTS 均不可用 |

#### 12. 设备与节点管理

| Gateway 方法 | 后端状态 | 影响 |
|-------------|---------|------|
| `node.list/describe/invoke` / `node.pair.*` (5个) | ❌ Stubs | 节点管理不可用 |
| `device.pair.*` (3个) / `device.token.*` (3个) | ❌ Stubs | 设备配对和令牌管理不可用 |

#### 13. 执行审批（Human-in-the-Loop）

| Gateway 方法 | 后端状态 | 影响 |
|-------------|---------|------|
| `exec.approvals.get/set` / `exec.approval.*` (6个) / `plugin.approval.*` (4个) | ❌ 全部 Stub | 无法审批/拒绝危险操作 |

> 注：后端有内存审批实现（`approval_store.go`），但未桥接到 Gateway。

#### 14. 工作面板与任务

| Gateway 方法 | 后端状态 | 影响 |
|-------------|---------|------|
| `workboard.cards.*` (7个) / `tasks.list/get/cancel` | ❌ Stubs | 看板和任务管理不可用 |

#### 15. 其他功能

| Gateway 方法 | 后端状态 | 影响 |
|-------------|---------|------|
| `doctor.memory.*` (8个) | ❌ Stubs | 记忆/梦境系统不可用 |
| `wiki.import.insights/wiki.palace` | ❌ Stubs | Wiki 功能不可用 |
| `logs.tail` | ❌ Stub | 实时日志不可用 |
| `push.*` (4个) | ❌ Stub | 推送通知不可用 |
| `wizard.*` (4个) | ❌ Stub | 设置向导不可用 |
| `artifacts.list/get/download` | ❌ Stubs | 产物管理不可用 |
| `environments.list/status` | ❌ Stubs | 环境管理不可用 |
| `commands.list` | ❌ Stub | 命令列表不可用 |
| `system-presence` | ❌ Stub | 系统在线状态不可用 |

---

## 后端已实现但未桥接到 Gateway 的功能

| 后端模块 | 路径 | 已实现功能 | 缺失的桥接 |
|---------|------|-----------|-----------|
| 工具管理 | `agent/hub/` | 插件市场 CRUD、远程 MCP CRUD、内置工具管理 | Gateway 的 `plugins.*`、`tools.*` |
| 技能系统 | `agent/skill/` | 文件系统技能管理、YAML 前置元数据、Pin/Unpin | Gateway 的 `skills.*` |
| 搜索引擎 | `agent/search/` | PostgreSQL 全文搜索、游标分页 | Gateway 的 `chat.message.get` |
| 分析引擎 | `agent/insight/` | Token/成本/工具使用报告、9 种模型定价 | Gateway 的 `usage.*` |
| 风险评估 | `agent/risk/` | 可配置规则引擎 + 硬编码危险命令检测 | Gateway 的 `exec.approvals.*` |
| 安全守卫 | `agent/guard/` | 三层守卫链（SPI → 规则引擎 → LLM 审计） | Gateway 的审批流程 |
| 审批存储 | `agent/service/` | 内存审批存储 + TTL 过期 | Gateway 的 `exec.approval.*` |
| MCP 服务端 | `agent/mcp/server/` | Agent 作为 MCP Server 暴露工具 | 无 Gateway 桥接 |
| 工具执行 | `agent/tool/` | 23 个内置工具 + MCP 工具适配器 | Gateway 的 `tools.invoke` |
| 审计事件 | `agent/audit/` | 内存环形缓冲 + DB 持久化 | Gateway 的 `logs.tail` |
| 配置服务 | `agent/service/config_service.go` | 配置 CRUD + 动态规则加载 | Gateway 的 `config.*` |

---

## 后端内部缺陷（即使桥接完成仍会导致功能不可用）

以下问题不是"未桥接"，而是后端 Agent 子系统**自身的 bug 或设计缺陷**。

### 缺陷 1：Nudge 记忆审查系统是死代码 🔴

**文件**：`internal/agent/nudge/nudge.go:81,126`，`pkg/di/wire_gen.go:82`

`SetLLMCall()` 方法在 Wire 构造 `MemoryNudgeReviewer` 后**从未被调用**。`Review()` 方法（第 126 行）检查 `r.llmCall == nil` 后直接 return，输出 warning "LLM call function not set"。

**影响**：整个记忆审查 nudge 系统完全不工作。`service.go` 中的 `RecordTurn` 和 `ShouldNudge` 计数器在正常运行，但触发审查后始终被跳过。MEMORY.md 永远不会被写入。

**修复**：在 `wire_gen.go` 的初始化流程中调用 `nudgeReviewer.SetLLMCall(llmFunc)`。

### 缺陷 2：记忆系统硬编码 userID=0 🔴

**文件**：`internal/agent/memory/provider.go:26`

```go
memories, err := p.dao.ListUserMemoriesByUser(ctx, 0, 20)
```

UserID 写死为 0，未从 context 中提取。所有用户共享同一个记忆池。

### 缺陷 3：记忆 DAO 忽略 userID 过滤 🔴

**文件**：`internal/agent/dao/agent_dao.go:665-675`

`ListUserMemoriesByUser` 方法的 SQL 查询**从未添加 `WHERE user_id = ?` 条件**，始终返回全局最近 20 条记忆。即使修复了缺陷 2，记忆隔离仍然无效。

### 缺陷 4：7 个 Agent 数据库表缺失 AutoMigrate 🔴

**文件**：`pkg/di/init.go:17-32`

`InitTables` 函数只迁移了 `AgentConfig`，以下 7 个表未包含：

| 表名 | 模型 | 影响 |
|------|------|------|
| `cl_agent_sessions` | `AgentSession` | 会话管理失败 |
| `cl_agent_messages` | `AgentMessage` | 消息存储失败 |
| `cl_agent_builtin_tools` | `BuiltinTool` | 工具管理失败 |
| `cl_agent_mcp_plugins` | `MCPPlugin` | 插件市场失败 |
| `cl_agent_mcp_plugin_installs` | `MCPPluginInstall` | 插件安装失败 |
| `cl_agent_remote_mcp_configs` | `RemoteMCPConfig` | 远程 MCP 失败 |
| `cl_agent_audit_events` | `AgentAuditEvent` | 审计日志失败 |

**影响**：在全新数据库上启动时，Agent 系统的大部分功能会静默失败。

### 缺陷 5：LLM Provider 字段被忽略，始终使用 OpenAI 🟡

**文件**：`internal/agent/service/service.go:125-136`

`createChatModel` 方法始终实例化 `einoopenai.ChatModel`，无论配置中 `agent.llm.provider` 设置为什么值。`Provider` 字段是装饰性的，无法使用 Anthropic、Ollama 或其他提供商。

### 缺陷 6：开发配置缺少 API Key 🟡

**文件**：`config/config.development.yaml:76-86`

`agent.llm` 配置只有 `provider: "openai"`、`temperature: 0.7`、`max_tokens: 4096`，**没有 `api_key`、`base_url`、`model` 字段**。依赖 Go 代码中的默认值（`BaseURL` 默认 `https://api.openai.com/v1`，`APIKey` 默认空字符串）。空 API Key 会导致运行时 LLM 调用失败。

### 缺陷 7：零内置技能 🟡

**目录**：`data/skills/`

技能目录下只有一个空的 `.usage.json`（内容为 `{}`），没有任何 SKILL.md 文件。技能系统基础设施完整但无内容，对终端用户不可用。

### 缺陷 8：LLM 调用无重试/容错 🟡

**文件**：`internal/agent/service/service.go:233,531`

`Query` 和 `QueryStream` 方法直接调用 `agent.Generate()` 或 `runAgentStream()`，没有任何重试逻辑、超时处理或 fallback 模型。单次 API 失败直接返回错误给用户。

### 缺陷 9：流式路径工具调用计数始终为 0 🟠

**文件**：`internal/agent/service/service.go:541-542,694-695`

流式路径始终设置 `toolCallCount := 0`（注释："streaming path doesn't expose tool call count"）。导致流式模式下技能 nudge（基于工具调用次数触发）永远不会触发。

### 缺陷 10：UserMemory 模型定义位置不一致 🟠

**文件**：`internal/agent/dao/agent_dao.go:16-25`

`UserMemory` 结构体定义在 DAO 层而非 `internal/model/`，与其他所有持久化实体的定义位置不一致。也没有 `TableName()` 方法，GORM 会自动推导表名（可能为 `user_memories`），不符合项目 `cl_` 前缀的命名规范。

### 缺陷 11：Agent 领域类型重复定义 🟠

**文件**：`internal/model/agent.go` vs `internal/agent/model/types.go`

两个包分别定义了 `AgentMessage`、`AgentSession`、`AuditEvent` 等类型，存在潜在的类型漂移风险。

### 缺陷 12：配置 YAML 缺少 Agent 子模块配置节 🟠

`config.development.yaml` 缺少 `agent.nudge`、`agent.skill`、`agent.search` 等配置节。Go 结构体中有默认值，但这些配置对运维人员不可见，无法调优。

---

## 前端直接 HTTP 调用（非 WebSocket）

前端并非只通过 WebSocket 通信，以下场景使用直接 `fetch()` 调用后端 REST API：

| 调用方 | 目标端点 | 后端状态 |
|--------|---------|---------|
| `mcp-manager.ts` | `/api/system/agent/hub/plugins/*` | ✅ 已实现 |
| `mcp-manager.ts` | `/api/system/agent/remote-mcps/*` | ✅ 已实现 |
| `mcp-manager.ts` | `/api/system/agent/builtin-tools/*` | ✅ 已实现 |
| `control-ui-bootstrap.ts` | `/api/system/agent/config` | ✅ 已实现 |
| `app-channels.ts` | `/api/channels/nostr/:accountId/profile` | ❌ 未验证 |
| `app-chat.ts` | `/avatar/:agentId?meta=1` | ❌ 未验证 |
| `chat/grouped-render.ts` | `/api/chat/media/outgoing/*` | ❌ 未验证 |
| `realtime-talk-webrtc.ts` | `https://api.openai.com/v1/realtime/calls` (外部) | N/A |

---

## 后端 REST API 全貌

后端共有 **108 个 HTTP 端点**，分布在 10 个路由组：

| 路由组 | 端点数 | 前端是否使用 | 说明 |
|--------|--------|-------------|------|
| `/api/system/agent/` | 37 | 部分（REST 部分被 mcp-manager 使用） | Agent 查询、会话、插件、MCP、配置、技能、搜索、分析 |
| `/api/files/` | 21 | ❌ 未使用 | 文件管理 CRUD + 分享 |
| `/api/role/` | 14 | ❌ 未使用 | 角色 RBAC 管理 |
| `/api/user/` | 13 | ❌ 未使用 | 用户认证和管理 |
| `/api/audit/` | 10 | ❌ 未使用 | 审计日志 |
| `/api/apis/` | 6 | ❌ 未使用 | API 注册管理 |
| `/api/share/` | 4 | ❌ 未使用 | 公共文件分享 |
| `/api/system/terminal/` | 4 | ❌ 未使用 | Web 终端 |
| `/api/system/` | 3 | ❌ 未使用 | 系统信息/指标 |
| `/api/not_auth/` | 2 | ❌ 未使用 | 无认证路由（空实现） |

**注意**：用户管理、角色 RBAC、文件管理、审计日志等 REST API 已完整实现，但前端（OpenClaw Control UI）不使用这些端点——它依赖 Gateway RPC 层。这意味着这些 REST API 可能是为旧版前端或其他客户端准备的，或者 Gateway 层需要重新实现这些功能。

---

## 优先级建议

### P0 — 基础设施修复（无则整个 Agent 系统不可用）

1. **修复 7 个 Agent 表的 AutoMigrate** — 新数据库上 Agent 系统完全无法工作
2. **修复 LLM API Key 配置** — 开发环境下 LLM 调用必然失败
3. ~~**修复 `tools.invoke` 桥接**~~ ✅ 已完成 — 工具调用是 Agent 核心能力
4. **修复 Nudge 系统 `SetLLMCall` 未调用** — 记忆审查死代码
5. **修复记忆系统 userID=0 和 DAO 过滤** — 记忆隔离完全失效

### P1 — 核心体验

6. ~~**`chat.abort`**~~ ✅ 已实现 — RunTracker 取消运行上下文 + 流式适配器响应 Aborted 状态
7. ~~**`sessions.messages.subscribe`**~~ ✅ 已实现 — SubscriptionHub + ChatStreamAdapter 广播，订阅者实时接收消息推送
8. ~~**`config.*` 系列**~~ ✅ 已实现 — DB 运行时配置覆盖 YAML 默认值，重启自动恢复
9. ~~**`models.list/authStatus/authLogout`**~~ ✅ 已实现 — AgentService.GetModelCatalog 桥接到 Gateway，支持模型列表和授权状态管理
10. **LLM Provider 多提供商支持** — 目前硬编码为 OpenAI

### P2 — 管理功能

11. ~~**`cron.*` 系列**~~ ✅ 已实现 — 可管理定时任务、启停、手动运行并查看历史
12. **`skills.*` 系列** — 无法使用技能系统（且需要内置技能内容）
13. **`usage.*` 系列** — 无法查看用量和费用
14. **`workboard.*` / `tasks.*`** — 看板和任务管理
15. **`logs.tail`** — 实时日志查看

### P3 — 运维功能

16. **`node.*` / `device.*`** — 设备和节点管理
17. **`exec.approval.*`** — 危险操作的人工审批
18. ~~**`agent.*` CRUD**~~ ✅ 已实现 — GatewayAgent 模型 + DB 持久化 + 文件系统存储
19. **`doctor.memory.*`** — 记忆/梦境系统（依赖 Nudge 修复）
20. **LLM 调用重试/容错** — 单次 API 失败不应直接报错

### P4 — 增强功能

21. **`channels.*`** — 消息频道集成（Discord/Telegram 等）
22. **`talk.*` / `tts.*`** — 语音对话
23. **`push.*`** — 推送通知
24. **`wizard.*`** — 设置向导
25. **`artifacts.*`** / **`environments.*`** / **`commands.*`**

---

## 建议的修复与桥接策略

### Phase 0：基础设施修复（1-2 天）

```
1. pkg/di/init.go — 在 InitTables 中添加 7 个 Agent 表的 AutoMigrate
2. config/config.development.yaml — 添加 agent.llm.api_key / base_url / model
3. internal/agent/memory/provider.go — 从 context 提取 userID
4. internal/agent/dao/agent_dao.go — ListUserMemoriesByUser 添加 WHERE user_id = ? 过滤
5. pkg/di/wire_gen.go — 构造 nudgeReviewer 后调用 SetLLMCall
```

### Phase 1：核心桥接（部分完成，剩余约 1-2 天）

```
✅ rpc/chat.go → agent/service (abort) — 2026-06-23 已完成
✅ rpc/sessions.go → agent/service (subscribe + 事件广播) — 2026-06-23 已完成
✅ rpc/tools.go → agent/tool/mcp/manager (tools.invoke + 动态 catalog) — 2026-06-23 已完成
```

### Phase 2：管理桥接（P1-P2，3-5 天）

```
✅ rpc/config.go → agent/service/config_service + Viper (DB 运行时覆盖 + YAML 默认值) — 2026-06-23 已完成
✅ rpc/models.go → LLM provider 配置 / AgentService.GetModelCatalog — 2026-06-23 已完成
rpc/cron.go → internal/cron/
rpc/skills.go → agent/skill/
rpc/usage.go → agent/insight/
rpc/workboard.go → 新建
```

### Phase 3：运维桥接（P3，3-5 天）

```
rpc/nodes.go → agent/hub/
rpc/exec_approval.go → agent/service/approval_store
rpc/doctor.go → agent/memory/ + agent/nudge/
```

### Phase 4：增强功能（P4，5-7 天）

```
rpc/channels.go → 新建消息频道适配层
rpc/talk.go → 新建语音适配层
LLM 多提供商支持 + 重试逻辑
内置技能内容
```

**总工作量估计**：Phase 0-4 合计约 15-22 天。
