package skill

import "time"

// SkillSource skill 来源
type SkillSource string

const (
	SkillSourceBundled SkillSource = "bundled"
	SkillSourceUser    SkillSource = "user"
	SkillSourceAgent   SkillSource = "agent"
)

// SkillState 生命周期状态
type SkillState string

const (
	SkillStateActive   SkillState = "active"
	SkillStateStale    SkillState = "stale"
	SkillStateArchived SkillState = "archived"
	SkillStatePinned   SkillState = "pinned"
)

// SkillMeta SKILL.md YAML frontmatter (agentskills.io 兼容)
type SkillMeta struct {
	Name        string   `yaml:"name" json:"name"`
	Description string   `yaml:"description" json:"description"`
	Version     string   `yaml:"version,omitempty" json:"version,omitempty"`
	License     string   `yaml:"license,omitempty" json:"license,omitempty"`
	Tags        []string `yaml:"tags,omitempty" json:"tags,omitempty"`
	Platforms   []string `yaml:"platforms,omitempty" json:"platforms,omitempty"`
	CreatedBy   string   `yaml:"created_by,omitempty" json:"created_by,omitempty"`
	Category    string   `yaml:"category,omitempty" json:"category,omitempty"`
}

// Skill 完整 skill 对象
type Skill struct {
	Meta       SkillMeta   `json:"meta"`
	Path       string      `json:"path"`
	Content    string      `json:"content"`
	Source     SkillSource `json:"source"`
	State      SkillState  `json:"state"`
	UseCount   int         `json:"use_count"`
	ViewCount  int         `json:"view_count"`
	PatchCount int         `json:"patch_count"`
	CreatedAt  time.Time   `json:"created_at"`
	UpdatedAt  time.Time   `json:"updated_at"`
	PinnedAt   *time.Time  `json:"pinned_at,omitempty"`
}

// SkillUsage .usage.json 条目
type SkillUsage struct {
	Name           string     `json:"name"`
	UseCount       int        `json:"use_count"`
	ViewCount      int        `json:"view_count"`
	PatchCount     int        `json:"patch_count"`
	LastActivityAt time.Time  `json:"last_activity_at"`
	State          SkillState `json:"state"`
	Pinned         bool       `json:"pinned"`
}
