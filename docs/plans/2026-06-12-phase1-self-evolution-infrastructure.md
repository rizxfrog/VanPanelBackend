# Phase 1: 自进化基础设施实施计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 构建 VanPanelBackend 的自进化基础设施 — FTS5 会话搜索、Memory Nudge、Skill 系统、Skill Manager 工具、Insights Engine

**Architecture:** 5 个独立模块，全部基于现有 PostgreSQL 数据库。全文搜索使用 PostgreSQL 原生 `tsvector`/`tsquery` + GIN 索引，无需额外数据库。所有模块遵循现有 `api → service → dao → model` 分层架构，通过 Wire DI 注入。Skill 系统使用 agentskills.io 兼容的 SKILL.md 格式，文件存储在 `data/skills/`。

**Tech Stack:** Go 1.25, GORM, Eino, PostgreSQL tsvector (全文搜索), Asynq (cron), Gin, Wire, zap

**设计决策 (Brainstorming 产出):**
- Skill 存储: 纯文件系统 (Hermes 风格), `data/skills/category/name/SKILL.md`
- Skill 来源: 用户 + Agent 均可创建, `created_by` 字段区分
- Skill Prompt 注入: Hermes 风格 — 分类分组 + 60 字符截断描述 + 强制语气
- 全文搜索: PostgreSQL tsvector (非 SQLite), 零额外依赖

**参考仓库:**
- OpenClaw: `/home/van/github/openclaw/CODEBASE_INDEX.md`
- Hermes: `/home/van/github/hermes-agent/CODEBASE_INDEX.md`

---

## 模块架构概览

```
internal/agent/
├── search/          # [NEW] PostgreSQL 全文搜索
│   ├── engine.go         tsvector/tsquery 搜索核心
│   ├── engine_test.go
│   └── doc.go
├── nudge/           # [NEW] Memory Nudge 系统
│   ├── nudge.go          后台审查 fork + 自动写入记忆
│   ├── nudge_test.go
│   └── doc.go
├── skill/           # [NEW] Skill 系统
│   ├── skill.go          Skill CRUD (文件系统 + DB 元数据)
│   ├── manager.go        SkillManager — Agent 操作的 skill 工具
│   ├── usage.go          Skill 使用追踪
│   ├── types.go          Skill 类型定义
│   ├── skill_test.go
│   └── doc.go
├── insight/         # [NEW] Insights Engine
│   ├── engine.go         Token/费用/工具分析
│   ├── engine_test.go
│   └── doc.go
├── dao/
│   └── agent_dao.go      [MODIFY] 新增 DAO 方法
├── service/
│   └── service.go         [MODIFY] 集成 Nudge + Search
└── ...existing files...

pkg/di/
├── agent.go               [MODIFY] 新增 Wire provider
└── wire.go                [MODIFY] 注入新模块
```

---

## Task 1: PostgreSQL 全文搜索引擎

**目标:** 提供全文本搜索历史对话 + LLM 摘要功能，支持 CJK。

**Files:**
- Create: `internal/agent/search/doc.go`
- Create: `internal/agent/search/engine.go`
- Create: `internal/agent/search/engine_test.go`
- Modify: `internal/agent/dao/agent_dao.go` — 新增 SearchMessages, GetMessageContext
- Modify: `internal/model/agent.go` — AgentMessage 新增 SearchVector 字段

### 设计要点

- 使用 PostgreSQL 原生 `tsvector`/`tsquery` + GIN 索引，**无需额外数据库**
- `cl_agent_messages` 新增 `search_vector` 列 (GENERATED ALWAYS AS `to_tsvector('simple', content)` STORED)
- 使用 `simple` dictionary (不做词干化, CJK 兼容)
- 使用 `plainto_tsquery()` 将用户输入转为搜索查询 (自动分词 + AND)
- 使用 `ts_rank()` 排序, `ts_headline()` 生成高亮 snippet
- 三个搜索模式:
  1. **DISCOVERY** — `ts_query` 全库搜索，返回 snippets + context windows
  2. **SCROLL** — 在特定会话内浏览，按 message_id 偏移
  3. **BROWSE** — 列出最近会话及标题/预览/时间戳
- AutoMigrate 自动创建 GIN 索引
- 无需数据同步 — 消息写入时自动更新 search_vector

### Step 1: Model 层增强

```go
// internal/model/agent.go — AgentMessage 新增字段

type AgentMessage struct {
    // ... 现有字段 ...
    SearchVector string `json:"-" gorm:"type:tsvector;index:,type:gin,expression:gin_trgm_ops;comment:全文搜索向量"`
    // GORM 不支持 generated column, 需要 raw SQL migration:
    // ALTER TABLE cl_agent_messages ADD COLUMN search_vector tsvector
    //   GENERATED ALWAYS AS (to_tsvector('simple', coalesce(content, ''))) STORED;
    // CREATE INDEX idx_messages_search ON cl_agent_messages USING GIN(search_vector);
}
```

### Step 1b: Database Migration

```go
// pkg/di/db.go — InitDB() 中添加 raw SQL migration

func createSearchVector(db *gorm.DB) error {
    // 检查列是否已存在
    var exists bool
    db.Raw(`SELECT EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_name = 'cl_agent_messages' AND column_name = 'search_vector'
    )`).Scan(&exists)
    if exists {
        return nil
    }

    return db.Exec(`
        ALTER TABLE cl_agent_messages 
        ADD COLUMN search_vector tsvector 
        GENERATED ALWAYS AS (to_tsvector('simple', coalesce(content, ''))) STORED;

        CREATE INDEX IF NOT EXISTS idx_messages_search 
        ON cl_agent_messages USING GIN(search_vector);
    `).Error
}
```

### Step 2: 搜索引擎实现

```go
// internal/agent/search/engine.go
package search

import (
    "context"
    "time"
    "gorm.io/gorm"
    "go.uber.org/zap"
)

// SearchMode 搜索模式
type SearchMode string
const (
    SearchModeDiscovery SearchMode = "discovery"
    SearchModeScroll    SearchMode = "scroll"
    SearchModeBrowse    SearchMode = "browse"
)

// SearchResult 搜索结果
type SearchResult struct {
    SessionID    string    `json:"session_id"`
    MessageID    int64     `json:"message_id"`
    Role         string    `json:"role"`
    Headline     string    `json:"headline"`     // ts_headline 高亮 snippet
    Content      string    `json:"content"`      // 完整内容 (前500字符)
    Rank         float64   `json:"rank"`         // ts_rank
    CreatedAt    time.Time `json:"created_at"`
    SessionTitle string    `json:"session_title,omitempty"`
}

// SearchEngine PostgreSQL 全文搜索引擎
type SearchEngine struct {
    db     *gorm.DB
    logger *zap.Logger
}

func NewSearchEngine(db *gorm.DB, logger *zap.Logger) *SearchEngine {
    return &SearchEngine{db: db, logger: logger}
}

// Search 全文本搜索 (discovery 模式)
// 使用 plainto_tsquery 将用户输入转为 tsquery, 自动 AND 连接
func (e *SearchEngine) Search(ctx context.Context, query string, limit int) ([]*SearchResult, error) {
    if limit <= 0 {
        limit = 20
    }

    var results []*SearchResult
    err := e.db.WithContext(ctx).Raw(`
        SELECT 
            m.session_id,
            m.id as message_id,
            m.role,
            ts_headline('simple', m.content, plainto_tsquery('simple', ?), 
                'MaxWords=40 MinWords=15 ShortWord=3 MaxFragments=3 StartSel=<b> StopSel=</b>') as headline,
            substring(m.content, 1, 500) as content,
            ts_rank(m.search_vector, plainto_tsquery('simple', ?)) as rank,
            m.created_at,
            s.title as session_title
        FROM cl_agent_messages m
        LEFT JOIN cl_agent_sessions s ON s.id::text = m.session_id
        WHERE m.search_vector @@ plainto_tsquery('simple', ?)
        ORDER BY rank DESC
        LIMIT ?
    `, query, query, query, limit).Scan(&results).Error

    return results, err
}

// Scroll 会话内滚动浏览 (按 message_id 偏移)
func (e *SearchEngine) Scroll(ctx context.Context, sessionID string, cursor int64, limit int) ([]*SearchResult, error) {
    if limit <= 0 {
        limit = 20
    }
    
    var results []*SearchResult
    err := e.db.WithContext(ctx).Raw(`
        SELECT 
            m.session_id, m.id as message_id, m.role,
            substring(m.content, 1, 500) as content,
            m.created_at
        FROM cl_agent_messages m
        WHERE m.session_id = ? AND m.id > ?
        ORDER BY m.id ASC
        LIMIT ?
    `, sessionID, cursor, limit).Scan(&results).Error

    return results, err
}

// Browse 浏览最近会话
func (e *SearchEngine) Browse(ctx context.Context, limit int) ([]*BrowseResult, error) {
    if limit <= 0 {
        limit = 20
    }
    
    var results []*BrowseResult
    err := e.db.WithContext(ctx).Raw(`
        SELECT 
            m.session_id,
            s.title,
            substring(m.content, 1, 100) as preview,
            s.message_count,
            MAX(m.created_at) as last_active_at
        FROM cl_agent_messages m
        LEFT JOIN cl_agent_sessions s ON s.id::text = m.session_id
        WHERE m.role = 'user'
        GROUP BY m.session_id, s.title, s.message_count
        ORDER BY last_active_at DESC
        LIMIT ?
    `, limit).Scan(&results).Error

    return results, err
}

// BrowseResult 浏览结果
type BrowseResult struct {
    SessionID    string    `json:"session_id"`
    Title        string    `json:"title"`
    Preview      string    `json:"preview"`
    MessageCount int       `json:"message_count"`
    LastActiveAt time.Time `json:"last_active_at"`
}
```

### Step 3: DAO 层方法

```go
// internal/agent/dao/agent_dao.go — AgentDAO interface 新增

// SearchMessages 全文搜索消息
SearchMessages(ctx context.Context, query string, limit int) ([]*search.SearchResult, error)

// GetMessageContext 获取消息上下文 (前后 N 条)
GetMessageContext(ctx context.Context, sessionID string, messageID int64, contextSize int) ([]*model.AgentMessage, error)

// ListRecentSessions 获取最近活跃会话
ListRecentSessions(ctx context.Context, limit int) ([]*search.BrowseResult, error)
```

### 对比: PostgreSQL tsvector vs SQLite FTS5

| 方面 | PostgreSQL tsvector | SQLite FTS5 |
|------|---------------------|-------------|
| 部署 | 零额外组件 | 需单独 SQLite 文件 |
| CJK 支持 | `simple` dict 天然支持 | `unicode61` tokenizer |
| 数据同步 | GENERATED 列自动更新 | 手动 INSERT/UPDATE |
| 索引 | GIN 索引, 高性能 | FTS5 内置索引 |
| 查询 | `plainto_tsquery()` 自动分词 | `MATCH` 语句 |
| 高亮 | `ts_headline()` 内置 | `snippet()` 函数 |
| 打分 | `ts_rank()` 内置 | `rank` 列 |

### Step 4: 配置 (可选)

PostgreSQL 搜索无需额外配置路径，使用数据库内置能力。可选调优参数:

```yaml
agent:
  search:
    max_results: 20
```

### Step 5: Wire 集成

```go
// pkg/di/agent.go
func ProvideAgentSearchEngine(db *gorm.DB, l *zap.Logger) *search.SearchEngine {
    return search.NewSearchEngine(db, l)
}
```

---

## Task 2: Memory Nudge 系统

**目标:** 每 N 轮对话后，后台 fork Agent 审查对话，自动写入记忆。

**Files:**
- Create: `internal/agent/nudge/doc.go`
- Create: `internal/agent/nudge/nudge.go`
- Create: `internal/agent/nudge/nudge_test.go`
- Modify: `internal/agent/service/service.go` — 在 Query/QueryStream 末尾调用 nudge

### 设计要点

- 两个 nudge 间隔 (可配置):
  - `memory.nudge_interval` — 触发记忆审查 (默认 10 轮)
  - `skills.creation_nudge_interval` — 触发 skill 审查 (默认 10 次工具调用)
- 后台线程 fork Agent (使用同一 LLM 配置) 审查当前对话
- 记忆审查 prompt 参考 Hermes 设计
- 智能去重: 不写入已存在的相似记忆
- 文件写入: `data/memory/MEMORY.md` 和 `data/memory/USER.md`

### Step 1: Nudge 系统实现

```go
// internal/agent/nudge/nudge.go
package nudge

import (
    "context"
    "sync"
    "go.uber.org/zap"
)

// NudgeConfig Nudge 配置
type NudgeConfig struct {
    MemoryInterval   int `json:"memory_interval"`    // 记忆审查间隔 (轮数)
    SkillInterval    int `json:"skill_interval"`     // skill 审查间隔 (工具调用数)
}

// NudgeState 当前 nudge 状态
type NudgeState struct {
    mu                sync.Mutex
    TurnCount         int  // 自上次 nudge 以来的轮数
    ToolCallCount     int  // 自上次 nudge 以来的工具调用数
    LastMemoryNudgeAt time.Time
    LastSkillNudgeAt  time.Time
}

// MemoryNudgeReviewer 记忆审查器
// 使用独立的 LLM 调用来审查对话并提取记忆
type MemoryNudgeReviewer struct {
    state  *NudgeState
    config NudgeConfig
    memoryDir string // data/memory/
    logger *zap.Logger
}

// ReviewMemoryPrompt Hermes 风格记忆审查 prompt
const reviewMemoryPrompt = `Review the conversation above and consider saving to memory if appropriate.

Focus on:
1. Has the user revealed things about themselves — persona, desires, 
   preferences, personal details?
2. Has the user expressed expectations about how you should behave, 
   work style, ways they want you to operate?
3. Has the user mentioned important facts, decisions, or constraints?

If nothing significant, reply with {"action": "skip"}.
If you find something worth remembering, reply with:
{"action": "add", "file": "MEMORY.md"|"USER.md", "content": "..."}`

// Review 执行记忆审查 (后台 goroutine)
func (r *MemoryNudgeReviewer) Review(ctx context.Context, messages []model.AgentMessage, llmConfig LLMConfig)

// ShouldNudge 检查是否应该触发 nudge
func (r *MemoryNudgeReviewer) ShouldNudge(toolCallsThisTurn int) (memoryNudge, skillNudge bool)

// RecordTurn 记录一轮完成 (增加计数)
func (r *MemoryNudgeReviewer) RecordTurn(toolCalls int)

// ResetMemory 重置记忆 nudge 计数
func (r *MemoryNudgeReviewer) ResetMemory()

// ResetSkill 重置 skill nudge 计数
func (r *MemoryNudgeReviewer) ResetSkill()
```

### Step 2: 集成到 AgentService

在 `internal/agent/service/service.go` 的 `Query`/`QueryStream` 方法末尾:

```go
// 在 agent.Generate() 完成后
if s.nudgeReviewer != nil {
    s.nudgeReviewer.RecordTurn(toolCallCount)
    if shouldMemoryNudge, shouldSkillNudge := s.nudgeReviewer.ShouldNudge(toolCallCount); shouldMemoryNudge || shouldSkillNudge {
        go func() {
            ctx := context.Background()
            if shouldMemoryNudge {
                s.nudgeReviewer.Review(ctx, messages, llmConfig)
            }
            // Skill nudge 在 Phase 2 skill 系统完成后启用
        }()
    }
}
```

### Step 3: 配置

```yaml
agent:
  nudge:
    memory_interval: 10    # 每10轮审查一次记忆
    skill_interval: 10      # 每10次工具调用审查一次 skill
```

---

## Task 3: Skill 系统核心

**目标:** agentskills.io 兼容的 Skill 系统 — Agent 可创建/修补/删除 skills，使用追踪。

**Files:**
- Create: `internal/agent/skill/types.go`
- Create: `internal/agent/skill/doc.go`
- Create: `internal/agent/skill/skill.go`
- Create: `internal/agent/skill/manager.go`
- Create: `internal/agent/skill/usage.go`
- Create: `internal/agent/skill/skill_test.go`
- Create: `data/skills/` (运行时目录)
- Create: `internal/model/skill.go` — DB 模型
- Modify: `internal/model/general.go` — 如需要

### 设计要点

- Skill 文件结构 (agentskills.io 兼容):
  ```
  data/skills/
  ├── network/
  │   └── diagnostics/
  │       ├── SKILL.md
  │       ├── references/
  │       └── templates/
  ├── kubernetes/
  │   └── troubleshooting/
  │       └── SKILL.md
  └── .usage.json   # 使用追踪 sidecar
  ```
- SKILL.md 格式: YAML frontmatter + Markdown 内容
- 三个来源: bundled (内置), user (用户创建), agent (Agent 创建)
- 生命周期: active → stale (30天未用) → archived (90天未用)
- pinned skills 豁免所有自动过渡
- 使用追踪: `use_count`, `view_count`, `patch_count`, `last_activity_at`

### Step 1: Skill 类型定义

```go
// internal/agent/skill/types.go
package skill

// SkillSource skill 来源
type SkillSource string
const (
    SkillSourceBundled SkillSource = "bundled"  // 内置
    SkillSourceUser    SkillSource = "user"     // 用户创建
    SkillSourceAgent   SkillSource = "agent"    // Agent 创建
)

// SkillState 生命周期状态
type SkillState string
const (
    SkillStateActive   SkillState = "active"
    SkillStateStale    SkillState = "stale"     // 30天未用
    SkillStateArchived SkillState = "archived"  // 90天未用
    SkillStatePinned   SkillState = "pinned"    // 已固定
)

// SkillMeta SKILL.md 的 YAML frontmatter
type SkillMeta struct {
    Name         string   `yaml:"name" json:"name"`                  // 唯一标识, ≤64 chars
    Description  string   `yaml:"description" json:"description"`    // ≤1024 chars
    Version      string   `yaml:"version,omitempty" json:"version,omitempty"`
    License      string   `yaml:"license,omitempty" json:"license,omitempty"`
    Tags         []string `yaml:"tags,omitempty" json:"tags,omitempty"`
    Platforms    []string `yaml:"platforms,omitempty" json:"platforms,omitempty"`
    CreatedBy    string   `yaml:"created_by,omitempty" json:"created_by,omitempty"` // "agent" | "user"
    Category     string   `yaml:"category,omitempty" json:"category,omitempty"`
}

// Skill 完整的 skill 对象
type Skill struct {
    Meta       SkillMeta    `json:"meta"`
    Path       string       `json:"path"`        // 文件系统路径
    Content    string       `json:"content"`     // SKILL.md 正文 (不含 frontmatter)
    Source     SkillSource  `json:"source"`
    State      SkillState   `json:"state"`
    UseCount   int          `json:"use_count"`
    ViewCount  int          `json:"view_count"`
    PatchCount int          `json:"patch_count"`
    CreatedAt  time.Time    `json:"created_at"`
    UpdatedAt  time.Time    `json:"updated_at"`
    PinnedAt   *time.Time   `json:"pinned_at,omitempty"`
}

// SkillUsage 使用追踪条目 (.usage.json)
type SkillUsage struct {
    Name           string     `json:"name"`
    UseCount       int        `json:"use_count"`
    ViewCount      int        `json:"view_count"`
    PatchCount     int        `json:"patch_count"`
    LastActivityAt time.Time  `json:"last_activity_at"`
    State          SkillState `json:"state"`
    Pinned         bool       `json:"pinned"`
}
```

### Step 2: Skill 管理器

```go
// internal/agent/skill/skill.go
package skill

// SkillStore Skill 持久化存储 (文件系统 + DB 元数据)
type SkillStore struct {
    baseDir string          // data/skills/
    usagePath string        // data/skills/.usage.json
    logger *zap.Logger
}

func NewSkillStore(baseDir string, logger *zap.Logger) (*SkillStore, error)

// ListSkills 列出所有 skills (progressive disclosure tier 1 — 只返回 metadata)
func (s *SkillStore) ListSkills(filterBySource ...SkillSource) ([]*Skill, error)

// GetSkill 获取完整 skill 内容 (tier 2 — 加载完整 SKILL.md)
func (s *SkillStore) GetSkill(name string) (*Skill, error)

// GetSkillFile 获取 skill 下的引用文件 (tier 3)
func (s *SkillStore) GetSkillFile(name string, filePath string) (string, error)

// CreateSkill 创建新 skill (Agent 或用户)
func (s *SkillStore) CreateSkill(name string, meta SkillMeta, content string) error

// PatchSkill 修补 skill (find-and-replace)
func (s *SkillStore) PatchSkill(name string, oldStr, newStr string) error

// WriteSkillFile 写入 skill 下的支持文件
func (s *SkillStore) WriteSkillFile(name string, filePath string, content string) error

// DeleteSkill 删除 skill
func (s *SkillStore) DeleteSkill(name string) error

// PinSkill 固定 skill
func (s *SkillStore) PinSkill(name string) error

// UnpinSkill 取消固定
func (s *SkillStore) UnpinSkill(name string) error

// RecordUse 记录使用 (增加 use_count)
func (s *SkillStore) RecordUse(name string) error

// RecordView 记录查看
func (s *SkillStore) RecordView(name string) error

// UpdateState 更新生命周期状态
func (s *SkillStore) UpdateState(name string, state SkillState) error

// FormatForPrompt 将 skills 格式化为 agent system prompt 注入文本
// 采用 Hermes 风格: 分类分组 + 60字符截断描述 + 强制语气
//
// 输出示例:
//
//	## Skills (mandatory)
//	Before replying, scan the skills below. If a skill matches,
//	you MUST load it with skill_view(name) and follow its instructions.
//
//	<available_skills>
//	  kubernetes: Kubernetes 集群管理
//	    - k8s-pod-debugging: 诊断和修复 Kubernetes Pod 启动失败、CrashLoopBac...
//	    - k8s-deployment: 管理 Deployment 的创建、更新和回滚...
//	  network: 网络诊断工具
//	    - network-diagnostics: 网络连通性诊断、DNS、路由、防火墙规则检查...
//	</available_skills>
func FormatSkillsForPrompt(skills []*Skill) string
```

### Step 3: DB 模型 (可选, 暂用文件系统)

如果后续需要跨节点同步或 Web UI 展示，可添加:

```go
// internal/model/skill.go
type AgentSkill struct {
    Model
    Name        string    `json:"name" gorm:"type:varchar(64);uniqueIndex;not null;comment:Skill标识"`
    Description string    `json:"description" gorm:"type:varchar(1024);comment:Skill描述"`
    Source      string    `json:"source" gorm:"type:varchar(20);not null;default:'agent';comment:来源 agent/user/bundled"`
    State       string    `json:"state" gorm:"type:varchar(20);not null;default:'active';comment:状态"`
    Category    string    `json:"category" gorm:"type:varchar(50);index;comment:分类"`
    Tags        StringList `json:"tags" gorm:"type:json;comment:标签"`
    UseCount    int       `json:"use_count" gorm:"default:0;comment:使用次数"`
    Pinned      bool      `json:"pinned" gorm:"default:false;comment:是否固定"`
}

func (AgentSkill) TableName() string { return "cl_agent_skills" }
```

---

## Task 4: Skill Manager Tool (Agent 工具)

**目标:** Agent 可以使用 `skill_manage` 工具来创建/修补/删除 skills。

**Files:**
- Create: `internal/agent/skill/manager.go` — SkillManageTool (Eino InvokableTool)
- Modify: `internal/agent/tool/builtin/registry.go` — 注册 skill_manage 工具

### 设计

工具名: `skill_manage`
参数:
```json
{
  "action": "create|edit|patch|delete|write_file|remove_file|pin|unpin|list",
  "name": "skill-name",
  "content": "...",       // create/edit 时
  "old_string": "...",    // patch 时
  "new_string": "...",    // patch 时
  "file_path": "...",     // write_file/remove_file 时
  "file_content": "..."   // write_file 时
}
```

### Step 1: Skill Manager Tool 实现

```go
// internal/agent/skill/manager.go
package skill

import (
    "context"
    "encoding/json"
    "github.com/cloudwego/eino/components/tool"
    "github.com/cloudwego/eino/schema"
)

// SkillManagerTool skill_manage 工具实现
// 实现 Eino 的 InvokableTool 接口
type SkillManagerTool struct {
    store *SkillStore
}

func NewSkillManagerTool(store *SkillStore) tool.InvokableTool {
    return &SkillManagerTool{store: store}
}

func (t *SkillManagerTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
    return &schema.ToolInfo{
        Name: "skill_manage",
        Desc: `Manage the agent's skill library. Skills are reusable procedural knowledge 
that capture how to perform specific types of tasks based on proven experience.

Actions:
  create  — Create a new skill (generates SKILL.md + directory structure)
  edit    — Replace the SKILL.md content of a skill (full rewrite)
  patch   — Targeted find-and-replace within SKILL.md
  delete  — Remove a skill entirely
  write_file — Add/overwrite a supporting file (reference, template, script)
  remove_file — Remove a supporting file
  pin     — Pin a skill (prevents auto-archival)
  unpin   — Unpin a skill
  list    — List all available skills with metadata`,
        ParamsOneOf: ...,
    }, nil
}

func (t *SkillManagerTool) InvokableRun(ctx context.Context, params string, opts ...tool.Option) (string, error) {
    var req SkillManageRequest
    if err := json.Unmarshal([]byte(params), &req); err != nil {
        return "", fmt.Errorf("解析skill_manage参数失败: %w", err)
    }
    
    switch req.Action {
    case "create":
        return t.handleCreate(ctx, req)
    case "edit":
        return t.handleEdit(ctx, req)
    case "patch":
        return t.handlePatch(ctx, req)
    case "delete":
        return t.handleDelete(ctx, req)
    case "write_file":
        return t.handleWriteFile(ctx, req)
    case "remove_file":
        return t.handleRemoveFile(ctx, req)
    case "pin":
        return t.handlePin(ctx, req)
    case "unpin":
        return t.handleUnpin(ctx, req)
    case "list":
        return t.handleList(ctx)
    default:
        return "", fmt.Errorf("未知操作: %s", req.Action)
    }
}
```

### Step 2: 注册到工具注册表

```go
// internal/agent/tool/builtin/registry.go 修改

func NewBuiltinTools(skillStore *skill.SkillStore) []tool.InvokableTool {
    tools := []tool.InvokableTool{
        // ... 现有 23 个工具 ...
        skill.NewSkillManagerTool(skillStore),  // 新增
    }
    return tools
}
```

---

## Task 5: Insights Engine

**目标:** 分析会话数据, 产生 token 消耗、费用、工具使用、活动趋势的报告。

**Files:**
- Create: `internal/agent/insight/doc.go`
- Create: `internal/agent/insight/engine.go`
- Create: `internal/agent/insight/engine_test.go`
- Modify: `internal/agent/dao/agent_dao.go` — 新增 GetSessionStats, GetToolUsageStats

### 设计

- 分析源: `cl_agent_messages` (带 metadata/token 信息)
- 如果消息 metadata 中有 token 信息, 直接使用; 否则根据内容估算
- 报告内容:
  - 时间段内的总 token 消耗 (input/output)
  - 费用估算 (按模型)
  - 工具使用分布 (频率、成功率)
  - 活动趋势 (按天)
  - 模型/平台 breakdown
  - 会话指标 (数量、平均长度)

### Step 1: DAO 新增方法

```go
// AgentDAO interface 新增
GetSessionStats(ctx context.Context, days int, userID int) (*SessionStats, error)
GetToolUsageStats(ctx context.Context, days int, userID int) ([]*ToolUsageStat, error)
GetDailyActivity(ctx context.Context, days int, userID int) ([]*DailyActivity, error)
GetModelBreakdown(ctx context.Context, days int, userID int) ([]*ModelBreakdown, error)
```

### Step 2: Insights Engine 实现

```go
// internal/agent/insight/engine.go
package insight

// InsightsEngine 分析引擎
type InsightsEngine struct {
    dao    dao.AgentDAO
    logger *zap.Logger
}

// InsightsReport 完整分析报告
type InsightsReport struct {
    Period         string            `json:"period"`          // "Last 30 days"
    TotalSessions  int               `json:"total_sessions"`
    TotalTokens    TokenBreakdown    `json:"total_tokens"`
    EstimatedCost  CostBreakdown     `json:"estimated_cost"`
    ToolUsage      []*ToolUsageStat  `json:"tool_usage"`
    DailyActivity  []*DailyActivity  `json:"daily_activity"`
    ModelBreakdown []*ModelBreakdown `json:"model_breakdown"`
    AvgSessionLen  int               `json:"avg_session_len"`
}

type TokenBreakdown struct {
    Input  int64 `json:"input"`
    Output int64 `json:"output"`
    Cache  int64 `json:"cache,omitempty"`
    Total  int64 `json:"total"`
}

type CostBreakdown struct {
    TotalUSD float64           `json:"total_usd"`
    ByModel  map[string]float64 `json:"by_model"`
}

type ToolUsageStat struct {
    ToolName     string  `json:"tool_name"`
    Count        int     `json:"count"`
    SuccessRate  float64 `json:"success_rate"`
}

type DailyActivity struct {
    Date       string `json:"date"`
    Sessions   int    `json:"sessions"`
    Messages   int    `json:"messages"`
    TokensIn   int64  `json:"tokens_in"`
    TokensOut  int64  `json:"tokens_out"`
}

type ModelBreakdown struct {
    Model    string `json:"model"`
    Sessions int    `json:"sessions"`
    TokensIn int64  `json:"tokens_in"`
    TokensOut int64 `json:"tokens_out"`
}

func NewInsightsEngine(dao dao.AgentDAO, logger *zap.Logger) *InsightsEngine {
    return &InsightsEngine{dao: dao, logger: logger}
}

// Generate 生成指定天数的分析报告
func (e *InsightsEngine) Generate(ctx context.Context, days int, userID int) (*InsightsReport, error)

// FormatTerminal 格式化报告为终端可读文本
func (e *InsightsEngine) FormatTerminal(report *InsightsReport) string

// FormatJSON 格式化报告为 JSON
func (e *InsightsEngine) FormatJSON(report *InsightsReport) (string, error)
```

### Step 3: Tool Metadata 增强

修改 `internal/agent/service/service.go` 的 `Query`/`QueryStream`, 在保存 assistant message 时添加 token 使用信息到 metadata:

```go
// 在 agent.Generate() 返回后
metadata := map[string]interface{}{
    "tokens_input":  usage.PromptTokens,
    "tokens_output": usage.CompletionTokens,
    "model":         modelName,
    "tool_calls":    toolCallCount,
}
message.Metadata = metadata
```

---

## Task 6: Wire DI 集成

**Files:**
- Modify: `pkg/di/agent.go` — 新增 provider 函数
- Modify: `pkg/di/wire.go` — 注册 provider 到 AgentSet
- Modify: `pkg/di/config.go` — 新增配置结构

### Step 1: 配置结构

```go
// pkg/di/config.go 新增

type AgentSearchConfig struct {
    MaxResults int `mapstructure:"max_results" default:"20" env:"AGENT_SEARCH_MAX_RESULTS"`
}

type AgentNudgeConfig struct {
    MemoryInterval int `mapstructure:"memory_interval" default:"10" env:"AGENT_NUDGE_MEMORY_INTERVAL"`
    SkillInterval  int `mapstructure:"skill_interval" default:"10" env:"AGENT_NUDGE_SKILL_INTERVAL"`
}

type AgentSkillConfig struct {
    BaseDir string `mapstructure:"base_dir" default:"data/skills" env:"AGENT_SKILL_BASE_DIR"`
}

// 添加到 AgentConfig
type AgentConfig struct {
    // ... 现有字段 ...
    Search AgentSearchConfig `mapstructure:"search"`
    Nudge  AgentNudgeConfig  `mapstructure:"nudge"`
    Skill  AgentSkillConfig  `mapstructure:"skill"`
}
```

### Step 2: Provider 函数

```go
// pkg/di/agent.go 新增

func ProvideAgentSearchEngine(l *zap.Logger) (*search.SearchEngine, error) {
    dbPath := di.GlobalConfig.Agent.Search.FTSDBPath
    return search.NewSearchEngine(dbPath, l)
}

func ProvideAgentSkillStore(cfg *di.AgentConfig, l *zap.Logger) (*skill.SkillStore, error) {
    baseDir := cfg.Skill.BaseDir
    if baseDir == "" {
        baseDir = "data/skills"
    }
    return skill.NewSkillStore(baseDir, l)
}

func ProvideAgentSkillManager(store *skill.SkillStore) tool.InvokableTool {
    return skill.NewSkillManagerTool(store)
}

func ProvideAgentNudgeReviewer(cfg *di.AgentConfig, l *zap.Logger) *nudge.MemoryNudgeReviewer {
    nudgeCfg := nudge.NudgeConfig{
        MemoryInterval: cfg.Nudge.MemoryInterval,
        SkillInterval:  cfg.Nudge.SkillInterval,
    }
    return nudge.NewMemoryNudgeReviewer(nudgeCfg, "data/memory", l)
}

func ProvideAgentInsightsEngine(dao dao.AgentDAO, l *zap.Logger) *insight.InsightsEngine {
    return insight.NewInsightsEngine(dao, l)
}
```

### Step 3: Wire Set

```go
// pkg/di/wire.go — 添加到 AgentSet

var AgentSet = wire.NewSet(
    // ... 现有 ...
    ProvideAgentSearchEngine,
    ProvideAgentSkillStore,
    ProvideAgentSkillManager,
    ProvideAgentNudgeReviewer,
    ProvideAgentInsightsEngine,
)
```

### Step 4: 重新生成

```bash
make generate  # 重新生成 wire_gen.go
```

---

## Task 7: Agent API 端点

**Files:**
- Modify: `internal/agent/api/handler.go` — 新增 handler 方法
- Modify: `main.go` — 注册路由

### 新增端点

```
GET  /api/system/agent/insights       — 获取分析报告
GET  /api/system/agent/search          — 搜索历史对话 (keyword search)
GET  /api/system/agent/skills          — 列出所有 skills
GET  /api/system/agent/skills/:name    — 获取 skill 详情
POST /api/system/agent/skills/:name/pin    — 固定 skill
POST /api/system/agent/skills/:name/unpin  — 取消固定
GET  /api/system/agent/memory          — 查看当前记忆 (MEMORY.md, USER.md)
```

---

## 实施顺序建议

```
Task 1: FTS5 搜索引擎          (独立, 无依赖)
Task 2: Memory Nudge 系统       (依赖 AgentService)
Task 3: Skill 系统核心          (独立)
Task 4: Skill Manager Tool      (依赖 Task 3)
Task 5: Insights Engine         (依赖 DAO 增强)
Task 6: Wire DI 集成            (依赖 Task 1-5)
Task 7: API 端点                (依赖 Task 6)
```

每个 Task 内部遵循 TDD: 先写测试 → 确认失败 → 实现 → 确认通过 → 提交。

---

## 关键依赖

```
go get gopkg.in/yaml.v3                # SKILL.md frontmatter 解析
# PostgreSQL tsvector 零额外依赖 — 数据库内置
```

---

## 文件清单汇总

### 新建文件 (15 个)
```
internal/agent/search/doc.go
internal/agent/search/engine.go
internal/agent/search/engine_test.go
internal/agent/nudge/doc.go
internal/agent/nudge/nudge.go
internal/agent/nudge/nudge_test.go
internal/agent/skill/doc.go
internal/agent/skill/types.go
internal/agent/skill/skill.go
internal/agent/skill/manager.go
internal/agent/skill/usage.go
internal/agent/skill/skill_test.go
internal/agent/insight/doc.go
internal/agent/insight/engine.go
internal/agent/insight/engine_test.go
```

### 修改文件 (6 个)
```
internal/agent/dao/agent_dao.go       — 新增 SearchMessages, GetSessionStats 等方法
internal/agent/service/service.go      — 集成 Nudge, 增强 metadata
internal/agent/tool/builtin/registry.go — 注册 skill_manage 工具
internal/agent/api/handler.go          — 新增 API 端点
pkg/di/agent.go                        — 新增 Wire provider
pkg/di/config.go                       — 新增配置结构
pkg/di/wire.go                         — 注册 provider 到 AgentSet
```
