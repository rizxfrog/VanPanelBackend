package skill

import "time"

// SkillSource skill 来源
type SkillSource string

const (
	SkillSourceBundled  SkillSource = "bundled"
	SkillSourceUser     SkillSource = "user"
	SkillSourceAgent    SkillSource = "agent"
	SkillSourceUploaded SkillSource = "uploaded"
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

	// 以下字段为 OpenClaw Control UI 前端兼容而增加，均为可选。
	Emoji        string             `yaml:"emoji,omitempty" json:"emoji,omitempty"`
	Homepage     string             `yaml:"homepage,omitempty" json:"homepage,omitempty"`
	Always       bool               `yaml:"always,omitempty" json:"always,omitempty"`
	Bundled      bool               `yaml:"bundled,omitempty" json:"bundled,omitempty"`
	PrimaryEnv   string             `yaml:"primary_env,omitempty" json:"primary_env,omitempty"`
	Source       string             `yaml:"source,omitempty" json:"source,omitempty"`
	Requirements SkillRequirements  `yaml:"requirements,omitempty" json:"requirements,omitempty"`
	Install      []SkillInstallSpec `yaml:"install,omitempty" json:"install,omitempty"`
	ClawHub      *SkillClawHubMeta  `yaml:"clawhub,omitempty" json:"clawhub,omitempty"`
	SkillCard    *SkillCardMeta     `yaml:"skill_card,omitempty" json:"skill_card,omitempty"`
}

// SkillRequirements 技能运行所需的环境/二进制/配置/操作系统
type SkillRequirements struct {
	AnyBins []string `yaml:"any_bins,omitempty" json:"any_bins,omitempty"`
	Bins    []string `yaml:"bins,omitempty" json:"bins,omitempty"`
	Env     []string `yaml:"env,omitempty" json:"env,omitempty"`
	Config  []string `yaml:"config,omitempty" json:"config,omitempty"`
	OS      []string `yaml:"os,omitempty" json:"os,omitempty"`
}

// SkillInstallSpec 单个安装选项（对应前端 SkillInstallOption）
type SkillInstallSpec struct {
	ID      string   `yaml:"id" json:"id"`
	Kind    string   `yaml:"kind" json:"kind"`
	Label   string   `yaml:"label" json:"label"`
	Bins    []string `yaml:"bins,omitempty" json:"bins,omitempty"`
	Command string   `yaml:"command,omitempty" json:"command,omitempty"`
}

// SkillClawHubMeta 技能与 ClawHub 注册表的关联信息
type SkillClawHubMeta struct {
	Registry         string `yaml:"registry" json:"registry"`
	Slug             string `yaml:"slug" json:"slug"`
	Version          string `yaml:"version,omitempty" json:"version,omitempty"`
	InstalledVersion string `yaml:"installed_version,omitempty" json:"installed_version,omitempty"`
	InstalledAt      int64  `yaml:"installed_at,omitempty" json:"installed_at,omitempty"`
}

// SkillCardMeta 技能卡片文件元信息
type SkillCardMeta struct {
	Path string `yaml:"path" json:"path"`
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
