# Phase 1: 自进化基础设施 - 设计规范

> 基于 Hermes Agent 自进化机制分析，为 VanPanelBackend 构建技能创建、记忆积累、会话搜索的学习闭环。

**参考:** OpenClaw `/home/van/github/openclaw/CODEBASE_INDEX.md` · Hermes `/home/van/github/hermes-agent/CODEBASE_INDEX.md`

---

## 1. 目标

构建 VanPanelBackend 的自进化基础设施，包含 5 个独立模块：

| 模块 | 目标 | 灵感来源 |
|------|------|----------|
| Skill 系统 | Agent 将经验保存为可复用的 SKILL.md | Hermes `skill_manager_tool.py` + `curator.py` |
| Skill Manager Tool | Agent 操作 skill 的 Eino 工具 | Hermes `skills_tool.py` |
| Memory Nudge | 后台审查对话，自动写入 MEMORY.md | Hermes `background_review.py` |
| 全文搜索 | PostgreSQL tsvector 搜索历史对话 | Hermes `hermes_state.py` FTS5 |
| Insights Engine | Token/费用/工具使用分析报告 | Hermes `insights.py` |

---

## 2. 架构概览

```
internal/agent/
├── skill/           [NEW] Skill 系统 (文件系统 + 使用追踪)
│   ├── types.go         SkillMeta, SkillState, SkillSource
│   ├── skill.go         SkillStore — 文件 CRUD + Prompt 注入
│   ├── manager.go       SkillManagerTool — Eino InvokableTool
│   ├── usage.go         .usage.json 使用追踪
│   └── doc.go
├── nudge/           [NEW] Memory Nudge
│   ├── nudge.go         对话审查 fork + MEMORY.md 写入
│   └── doc.go
├── search/          [NEW] PostgreSQL 全文搜索
│   ├── engine.go        tsvector/tsquery + ts_headline + ts_rank
│   └── doc.go
├── insight/         [NEW] Insights Engine
│   ├── engine.go        Token/费用/工具分析 + 趋势图
│   └── doc.go
├── service/service.go    [MODIFY] 集成 Nudge post-turn hook
├── dao/agent_dao.go      [MODIFY] 新增搜索/统计查询
└── model/agent.go        [MODIFY] AgentMessage 增强 metadata

pkg/di/
├── agent.go              [MODIFY] 新增 Provider 函数
├── config.go             [MODIFY] 新增配置结构
└── wire.go               [MODIFY] 注册到 AgentSet
```

**存储方案:** 纯文件系统 (Skill + Memory) + PostgreSQL (全文搜索 + Insights)

---

## 3. Skill 系统设计

### 3.1 决策记录

| 决策 | 选项 | 选择 | 理由 |
|------|------|------|------|
| 存储方式 | 纯文件 vs 文件+DB | **纯文件** | Hermes 风格，天然可移植，无需 DB 同步 |
| 创建来源 | Agent only vs User only vs Both | **Both** | `created_by` 字段区分 |
| Prompt 格式 | OpenClaw XML vs Hermes 分类 | **Hermes 分类** | 60字符截断，强制语气，省 token |
| 加载方式 | 全量注入 vs 按需 `skill_view` | **按需加载** | Progressive disclosure，省 token |

### 3.2 目录结构

```
data/skills/
├── kubernetes/
│   └── pod-debugging/
│       ├── SKILL.md          # YAML frontmatter + Markdown
│       ├── references/       # 参考文件 (可选)
│       └── templates/        # 输出模板 (可选)
├── network/
│   └── diagnostics/
│       └── SKILL.md
└── .usage.json               # 使用追踪 sidecar
```

### 3.3 SKILL.md 格式 (agentskills.io 兼容)

```markdown
---
name: k8s-pod-debugging
description: 诊断和修复 Kubernetes Pod 启动失败、CrashLoopBackOff 等问题
version: 1.0.0
created_by: agent
tags: [kubernetes, debugging, pods]
---

# K8s Pod Debugging

## 步骤
1. 先用 `kubectl describe pod` 查看 Events
2. 如果有 OOMKilled, 检查 memory limits
...
```

### 3.4 Skill 生命周期

```
active ──(30天未用)──> stale ──(90天未用)──> archived
  ↑                                                │
  └────────────────(再次使用)──────────────────────┘

pinned: 豁免所有自动过渡
```

### 3.5 Prompt 注入格式 (Hermes 风格)

```
## Skills (mandatory)
在回复前扫描以下 skills。如果某个 skill 匹配或部分相关，必须用
skill_view(name) 加载并遵循其指令。宁可多加载。

当你完成一个复杂任务 (5次以上工具调用)、修复了一个棘手错误、
或发现了一个非平凡的工作流后，用 skill_manage(action="create")
将方法保存为 skill，供未来复用。

使用某个 skill 时如果发现它过时、不完整或错误，立即用
skill_manage(action="patch") 修补 — 不要等待。

<available_skills>
  kubernetes: Kubernetes 集群管理
    - k8s-pod-debugging: 诊断和修复 Kubernetes Pod 启动失败、CrashLoopBac...
    - k8s-deployment: 管理 Deployment 的创建、更新和回滚...
  network: 网络诊断工具
    - network-diagnostics: 网络连通性诊断、DNS、路由、防火墙规则检查...
</available_skills>
```

描述截断到 60 字符。分类从目录名自动推断。

---

## 4. Skill Manager Tool 设计

### 4.1 工具签名

```
工具名: skill_manage
类型: Eino InvokableTool

action: list | view | create | edit | patch | delete | 
        write_file | remove_file | pin | unpin
```

### 4.2 Action 详情

| Action | 参数 | 行为 |
|--------|------|------|
| `list` | 无 | 返回所有 skill 的 name + description (不含正文) |
| `view` | `name` [, `file_path`] | 加载 SKILL.md 全文或引用文件 |
| `create` | `name`, `category`, `description`, `content` | 创建新 SKILL.md + 目录 |
| `edit` | `name`, `content` | 替换 SKILL.md 全文 |
| `patch` | `name`, `old_string`, `new_string` | find-and-replace |
| `delete` | `name` | 删除 skill 目录 |
| `write_file` | `name`, `file_path`, `content` | 写入 references/ 或 templates/ |
| `remove_file` | `name`, `file_path` | 删除支持文件 |
| `pin` | `name` | 标记为固定，防止自动归档 |
| `unpin` | `name` | 取消固定 |

### 4.3 安全约束

- `name` 参数限制 `[a-z0-9-]+`，防止路径穿越
- `file_path` 限制相对路径，不可包含 `..`
- `created_by: "user"` 的 skill 不可被 Agent 的 edit/patch/delete 修改
- 所有写入限制在 `data/skills/` 下

### 4.4 注册

与其他内置工具并列注册到 `builtin/registry.go`，通过 ToolManager 加载。

---

## 5. Memory Nudge 设计

### 5.1 流程

```
AgentService.Query/QueryStream 完成
    ↓
记录 turnCount++, toolCallCount += N
    ↓
shouldNudge?
    ├── memoryNudge (每 10 轮)
    │   ├── go forkLLMReview()
    │   │   ├── prompt = MEMORY_REVIEW_PROMPT
    │   │   ├── llm.Chat() (temperature=0.3)
    │   │   ├── parse JSON response
    │   │   └── 如果 action=add → 写入 MEMORY.md (带去重)
    │   └── return (不阻塞)
    └── skillNudge (每 10 次工具调用, Phase 2)
```

### 5.2 Memory Review Prompt

```
回顾上面对话，考虑是否要保存到记忆中。

关注:
1. 用户是否透露了个人信息 — 角色、偏好、个人细节?
2. 用户是否表达了对你行为方式的期望、工作风格偏好?
3. 用户是否提到了重要事实、决策或约束?

如果没有值得记录的，回复 {"action": "skip"}
如果有值得记录的，回复:
{"action": "add", "content": "...", "importance": "high|medium|low"}
```

### 5.3 MEMORY.md 格式

```markdown
## 2026-06-12
- 用户偏好使用 kubectl describe 而非 kubectl get events 来诊断 Pod 问题
- 用户要求所有 Kubernetes 操作前先确认 namespace
```

### 5.4 配置

```yaml
agent:
  nudge:
    memory_interval: 10    # 每 N 轮触发记忆审查
    skill_interval: 10     # 每 N 次工具调用触发 skill 审查 (Phase 2)
```

---

## 6. PostgreSQL 全文搜索设计

### 6.1 方案选择

| 方案 | | 选择 |
|------|------|------|
| SQLite FTS5 侧库 | 需额外数据库 + 手动同步 | ✗ |
| PostgreSQL tsvector | 零额外组件 + GENERATED 列自动更新 | ✓ |

### 6.2 数据库迁移

```sql
ALTER TABLE cl_agent_messages 
ADD COLUMN search_vector tsvector 
GENERATED ALWAYS AS (to_tsvector('simple', coalesce(content, ''))) STORED;

CREATE INDEX idx_messages_search ON cl_agent_messages USING GIN(search_vector);
```

### 6.3 搜索模式

| 模式 | 查询方式 | 用途 |
|------|----------|------|
| DISCOVERY | `search_vector @@ plainto_tsquery('simple', ?)` + `ts_rank()` + `ts_headline()` | 全库搜索 |
| SCROLL | `WHERE session_id = ? AND id > ? ORDER BY id ASC` | 会话内浏览 |
| BROWSE | `GROUP BY session_id ORDER BY last_active_at DESC` | 最近会话列表 |

### 6.4 CJK 支持

使用 `'simple'` dictionary — 不做词干化/停用词移除，逐字匹配。中、日、韩文本天然兼容。

---

## 7. Insights Engine 设计

### 7.1 报告内容

```
───── VanPanel Insights (最近 30 天) ─────

📊 总览
  会话数: 47    消息数: 892
  输入 Token: 1.2M    输出 Token: 380K
  估算费用: $2.34

📈 每日趋势
  06-12 ████████ 6 sessions  120 msg  56K tokens
  ...

🔧 工具使用 Top 5
  shell.exec  89次  ████████████████████
  ...

🤖 模型分布
  gpt-4o  38 sessions  1.4M tokens
  ...
```

### 7.2 数据来源

在 `AgentMessage.Metadata` 中存储每次 LLM 调用的 token 使用信息：

```json
{
  "tokens_input": 1523,
  "tokens_output": 478,
  "model": "gpt-4o",
  "tool_calls": [{"name": "shell.exec", "args": "..."}],
  "cost_usd": 0.0032
}
```

### 7.3 费用估算

内置模型定价表 (可配置):

```go
var modelPricing = map[string]PricePer1K{
    "gpt-4o":           {Input: 0.00250, Output: 0.01000},
    "gpt-4o-mini":      {Input: 0.00015, Output: 0.00060},
    "claude-3.5-sonnet": {Input: 0.00300, Output: 0.01500},
    "deepseek-chat":    {Input: 0.00014, Output: 0.00028},
}
```

纯 SQL 查询，不需要 LLM 参与。

### 7.4 API 端点

```
GET /api/system/agent/insights?days=30
```

---

## 8. 关键约束

| 约束 | 说明 |
|------|------|
| 不阻塞用户 | Nudge fork 在 goroutine 中运行 |
| 去重检查 | 写入前关键词匹配检查已有内容 |
| 路径安全 | Skill name 正则限制，file_path 预防穿越 |
| Agent 权限 | 只能修改 `created_by: "agent"` 的 skill |
| Token 预算 | Skill 描述截断 60 字符，正文按需加载 |

---

## 9. 实施顺序

```
Task 1: PostgreSQL 全文搜索  (独立, 无依赖)
Task 2: Skill 系统核心       (独立)
Task 3: Skill Manager Tool   (依赖 Task 2)
Task 4: Memory Nudge         (依赖 AgentService)
Task 5: Insights Engine      (依赖 DAO 增强)
Task 6: Wire DI 集成         (依赖 Task 1-5)
Task 7: API 端点             (依赖 Task 6)
```

---

## 10. 参考索引

- OpenClaw 代码索引: `/home/van/github/openclaw/CODEBASE_INDEX.md` (1739 行)
- Hermes 代码索引: `/home/van/github/hermes-agent/CODEBASE_INDEX.md` (892 行)
- 实施计划: `docs/plans/2026-06-12-phase1-self-evolution-infrastructure.md` (987 行)
