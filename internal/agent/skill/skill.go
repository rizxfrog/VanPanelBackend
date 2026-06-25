package skill

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

const defaultBaseDir = "data/skills"

var nameRegex = regexp.MustCompile(`^[a-z0-9-]+$`)

// SkillStore 基于文件系统的 skill 管理器
type SkillStore struct {
	baseDir   string       // data/skills/
	usagePath string       // data/skills/.usage.json
	mu        sync.RWMutex // 保护 .usage.json 的并发访问
	logger    *zap.Logger
}

// NewSkillStore 创建 skill 管理器
// baseDir 为空时默认为 "data/skills"
func NewSkillStore(baseDir string, logger *zap.Logger) (*SkillStore, error) {
	if baseDir == "" {
		baseDir = defaultBaseDir
	}

	s := &SkillStore{
		baseDir:   baseDir,
		usagePath: filepath.Join(baseDir, defaultUsageFile),
		logger:    logger,
	}

	// 确保 baseDir 存在
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("创建 skill 目录失败: %w", err)
	}

	// 初始化 .usage.json（如果不存在）
	if _, err := os.Stat(s.usagePath); os.IsNotExist(err) {
		if err := s.saveUsage(make(map[string]*SkillUsage)); err != nil {
			return nil, fmt.Errorf("初始化使用统计文件失败: %w", err)
		}
	}

	return s, nil
}

// ListSkills 列出所有 skill（仅元数据，不含完整内容）
func (s *SkillStore) ListSkills(ctx context.Context) ([]*Skill, error) {
	var skills []*Skill

	s.mu.RLock()
	usage, err := s.loadUsage()
	s.mu.RUnlock()
	if err != nil {
		return nil, fmt.Errorf("加载使用统计失败: %w", err)
	}

	err = filepath.Walk(s.baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 跳过无法访问的路径
		}
		if info.IsDir() || filepath.Base(path) != "SKILL.md" {
			return nil
		}

		// 跳过 .usage.json 等隐藏文件所在的目录
		if strings.HasPrefix(filepath.Base(filepath.Dir(path)), ".") {
			return nil
		}

		// 读取并解析 YAML frontmatter（仅元数据）
		meta, err := parseFrontmatter(path)
		if err != nil {
			s.logger.Warn("解析 SKILL.md frontmatter 失败", zap.String("path", path), zap.Error(err))
			return nil
		}

		if meta.Name == "" {
			return nil
		}

		// 从 .usage.json 获取使用统计
		u := usage[meta.Name]
		now := time.Now()

		skill := &Skill{
			Meta:      *meta,
			Path:      filepath.Dir(path),
			Source:    SkillSourceBundled,
			State:     SkillStateActive,
			CreatedAt: info.ModTime(),
			UpdatedAt: info.ModTime(),
		}

		if u != nil {
			skill.UseCount = u.UseCount
			skill.ViewCount = u.ViewCount
			skill.PatchCount = u.PatchCount
			skill.State = u.State
			if u.Pinned {
				skill.State = SkillStatePinned
			}
		}

		_ = now // 保留用于后续扩展

		skills = append(skills, skill)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("扫描 skill 目录失败: %w", err)
	}

	return skills, nil
}

// GetSkill 按名称加载完整的 skill（含内容）
func (s *SkillStore) GetSkill(ctx context.Context, name string) (*Skill, error) {
	skillPath, err := s.findSkillPath(name)
	if err != nil {
		return nil, err
	}

	skillMD := filepath.Join(skillPath, "SKILL.md")

	// 读取完整文件
	data, err := os.ReadFile(skillMD)
	if err != nil {
		return nil, fmt.Errorf("读取 SKILL.md 失败: %w", err)
	}

	// 解析 YAML frontmatter
	meta, content, err := ParseFullSkillMD(data)
	if err != nil {
		return nil, fmt.Errorf("解析 SKILL.md 失败: %w", err)
	}

	if meta.Name == "" {
		return nil, fmt.Errorf("skill 名称为空: %s", skillMD)
	}

	info, err := os.Stat(skillMD)
	if err != nil {
		return nil, fmt.Errorf("获取文件信息失败: %w", err)
	}

	// 记录查看（先记录再加载，确保返回的计数是最新的）
	s.RecordView(ctx, name)

	// 获取使用统计
	s.mu.RLock()
	u := s.getUsage(name)
	s.mu.RUnlock()
	state := SkillStateActive
	if u != nil {
		state = u.State
		if u.Pinned {
			state = SkillStatePinned
		}
	}

	skill := &Skill{
		Meta:      *meta,
		Path:      skillPath,
		Content:   content,
		Source:    SkillSourceBundled,
		State:     state,
		CreatedAt: info.ModTime(),
		UpdatedAt: info.ModTime(),
	}

	if u != nil {
		skill.UseCount = u.UseCount
		skill.ViewCount = u.ViewCount
		skill.PatchCount = u.PatchCount
	}

	return skill, nil
}

// GetSkillFile 加载 skill 的辅助文件（references/templates 中的文件）
func (s *SkillStore) GetSkillFile(ctx context.Context, name string, filePath string) ([]byte, error) {
	// 路径安全检查：禁止 .. 穿越
	if strings.Contains(filePath, "..") {
		return nil, fmt.Errorf("文件路径包含非法字符: ..")
	}

	skillPath, err := s.findSkillPath(name)
	if err != nil {
		return nil, err
	}

	fullPath := filepath.Join(skillPath, filePath)

	// 确保路径在 skill 目录内
	absSkillPath, err := filepath.Abs(skillPath)
	if err != nil {
		return nil, fmt.Errorf("获取绝对路径失败: %w", err)
	}
	absFullPath, err := filepath.Abs(fullPath)
	if err != nil {
		return nil, fmt.Errorf("获取绝对路径失败: %w", err)
	}
	if !strings.HasPrefix(absFullPath, absSkillPath+string(filepath.Separator)) && absFullPath != absSkillPath {
		return nil, fmt.Errorf("文件路径超出 skill 目录范围")
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}

	return data, nil
}

// CreateSkill 创建新 skill
func (s *SkillStore) CreateSkill(ctx context.Context, meta SkillMeta, content string, source SkillSource) (*Skill, error) {
	// 验证名称格式
	if !nameRegex.MatchString(meta.Name) {
		return nil, fmt.Errorf("skill 名称格式无效，必须匹配 [a-z0-9-]+: %s", meta.Name)
	}

	if len(meta.Name) > 64 {
		return nil, fmt.Errorf("skill 名称不能超过64个字符: %s", meta.Name)
	}
	if len(meta.Description) > 1024 {
		return nil, fmt.Errorf("skill 描述不能超过1024个字符")
	}

	category := meta.Category
	if category == "" {
		category = "general"
	}

	// 创建目录: data/skills/<category>/<name>/
	skillDir := filepath.Join(s.baseDir, category, meta.Name)
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		return nil, fmt.Errorf("创建 skill 目录失败: %w", err)
	}

	// 创建 references 和 templates 子目录
	for _, sub := range []string{"references", "templates"} {
		subDir := filepath.Join(skillDir, sub)
		if err := os.MkdirAll(subDir, 0755); err != nil {
			return nil, fmt.Errorf("创建 %s 目录失败: %w", sub, err)
		}
	}

	// 写入 SKILL.md
	skillMD := filepath.Join(skillDir, "SKILL.md")
	if err := WriteSkillMD(skillMD, meta, content); err != nil {
		return nil, fmt.Errorf("写入 SKILL.md 失败: %w", err)
	}

	// 更新使用统计
	now := time.Now()
	s.updateUsage(meta.Name, func(u *SkillUsage) {
		u.State = SkillStateActive
		u.LastActivityAt = now
	})

	skill := &Skill{
		Meta:      meta,
		Path:      skillDir,
		Content:   content,
		Source:    source,
		State:     SkillStateActive,
		CreatedAt: now,
		UpdatedAt: now,
	}

	s.logger.Info("skill 创建成功", zap.String("name", meta.Name), zap.String("category", category))
	return skill, nil
}

// PatchSkill 修补 skill 内容（find-and-replace）
func (s *SkillStore) PatchSkill(ctx context.Context, name string, oldStr string, newStr string) (*Skill, error) {
	skillPath, err := s.findSkillPath(name)
	if err != nil {
		return nil, err
	}

	skillMD := filepath.Join(skillPath, "SKILL.md")

	// 读取现有内容
	data, err := os.ReadFile(skillMD)
	if err != nil {
		return nil, fmt.Errorf("读取 SKILL.md 失败: %w", err)
	}

	// 执行替换
	newData := strings.ReplaceAll(string(data), oldStr, newStr)

	// 写回
	if err := os.WriteFile(skillMD, []byte(newData), 0644); err != nil {
		return nil, fmt.Errorf("写入 SKILL.md 失败: %w", err)
	}

	// 记录修补
	s.updateUsage(name, func(u *SkillUsage) {
		u.PatchCount++
	})

	// 重新加载返回
	skill, err := s.GetSkill(ctx, name)
	if err != nil {
		return nil, err
	}

	s.logger.Info("skill 修补成功", zap.String("name", name))
	return skill, nil
}

// DeleteSkill 删除 skill
func (s *SkillStore) DeleteSkill(ctx context.Context, name string) error {
	skillPath, err := s.findSkillPath(name)
	if err != nil {
		return err
	}

	// 删除整个 skill 目录
	if err := os.RemoveAll(skillPath); err != nil {
		return fmt.Errorf("删除 skill 目录失败: %w", err)
	}

	// 从使用统计中移除
	s.removeUsage(name)

	s.logger.Info("skill 删除成功", zap.String("name", name))
	return nil
}

// PinSkill 置顶 skill
func (s *SkillStore) PinSkill(ctx context.Context, name string) error {
	_, err := s.findSkillPath(name)
	if err != nil {
		return err
	}

	s.updateUsage(name, func(u *SkillUsage) {
		u.Pinned = true
	})

	return nil
}

// UnpinSkill 取消置顶 skill
func (s *SkillStore) UnpinSkill(ctx context.Context, name string) error {
	_, err := s.findSkillPath(name)
	if err != nil {
		return err
	}

	s.updateUsage(name, func(u *SkillUsage) {
		u.Pinned = false
	})

	return nil
}

// RecordUse 记录 skill 使用
func (s *SkillStore) RecordUse(ctx context.Context, name string) {
	s.updateUsage(name, func(u *SkillUsage) {
		u.UseCount++
	})
}

// RecordView 记录 skill 查看
func (s *SkillStore) RecordView(ctx context.Context, name string) {
	s.updateUsage(name, func(u *SkillUsage) {
		u.ViewCount++
	})
}

// UpdateState 更新 skill 生命周期状态
func (s *SkillStore) UpdateState(ctx context.Context, name string, state SkillState) error {
	_, err := s.findSkillPath(name)
	if err != nil {
		return err
	}

	s.updateUsage(name, func(u *SkillUsage) {
		u.State = state
	})

	return nil
}

// FormatSkillsForPrompt 生成 Hermes 风格的 skill 提示注入
func (s *SkillStore) FormatSkillsForPrompt(ctx context.Context) string {
	skills, err := s.ListSkills(ctx)
	if err != nil {
		s.logger.Warn("生成 skill 提示失败", zap.Error(err))
		return ""
	}

	if len(skills) == 0 {
		return ""
	}

	// 按分类分组
	categories := make(map[string][]*Skill)
	var catOrder []string

	for _, skill := range skills {
		cat := skill.Meta.Category
		if cat == "" {
			cat = "general"
		}
		if _, ok := categories[cat]; !ok {
			catOrder = append(catOrder, cat)
		}
		categories[cat] = append(categories[cat], skill)
	}

	sort.Strings(catOrder)

	var sb strings.Builder
	sb.WriteString("## Skills (mandatory)\n")
	sb.WriteString("在回复前扫描以下 skills。如果某个 skill 匹配或部分相关，必须用\n")
	sb.WriteString("skill_view(name) 加载并遵循其指令。宁可多加载。\n")
	sb.WriteString("\n")
	sb.WriteString("当你完成一个复杂任务 (5次以上工具调用)、修复了一个棘手错误、\n")
	sb.WriteString("或发现了一个非平凡的工作流后，用 skill_manage(action=\"create\")\n")
	sb.WriteString("将方法保存为 skill，供未来复用。\n")
	sb.WriteString("\n")
	sb.WriteString("使用某个 skill 时如果发现它过时、不完整或错误，立即用\n")
	sb.WriteString("skill_manage(action=\"patch\") 修补 — 不要等待。\n")
	sb.WriteString("\n")
	sb.WriteString("<available_skills>\n")

	for _, cat := range catOrder {
		catSkills := categories[cat]
		if len(catSkills) == 0 {
			continue
		}

		// 生成分类描述
		catDesc := s.categoryDescription(cat, catSkills)
		sb.WriteString(fmt.Sprintf("  %s: %s\n", cat, catDesc))

		for _, skill := range catSkills {
			desc := truncateDesc(skill.Meta.Description, 60)
			sb.WriteString(fmt.Sprintf("    - %s: %s\n", skill.Meta.Name, desc))
		}
	}

	sb.WriteString("</available_skills>\n")

	return sb.String()
}

// categoryDescription 根据分类中的 skill 生成分类描述
func (s *SkillStore) categoryDescription(cat string, skills []*Skill) string {
	if len(skills) == 1 {
		return truncateDesc(skills[0].Meta.Description, 60)
	}
	// 多个 skill：根据分类名推断"X管理"或"X工具"
	return cat + "相关技能"
}

// findSkillPath 按名称查找 skill 目录
func (s *SkillStore) findSkillPath(name string) (string, error) {
	var found string

	err := filepath.Walk(s.baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() || filepath.Base(path) != "SKILL.md" {
			return nil
		}
		if strings.HasPrefix(filepath.Base(filepath.Dir(path)), ".") {
			return nil
		}

		meta, err := parseFrontmatter(path)
		if err != nil || meta.Name != name {
			return nil
		}

		found = filepath.Dir(path)
		return filepath.SkipAll // 找到即停止
	})

	if err != nil {
		return "", fmt.Errorf("查找 skill 失败: %w", err)
	}
	if found == "" {
		return "", fmt.Errorf("skill 不存在: %s", name)
	}

	return found, nil
}

// parseFrontmatter 仅解析 SKILL.md 的 YAML frontmatter（不解析正文）
func parseFrontmatter(filePath string) (*SkillMeta, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	content := string(data)
	parts := strings.SplitN(content, "---\n", 3)
	if len(parts) < 3 {
		return nil, fmt.Errorf("无效的 SKILL.md 格式: 缺少 frontmatter")
	}

	var meta SkillMeta
	if err := yaml.Unmarshal([]byte(parts[1]), &meta); err != nil {
		return nil, fmt.Errorf("解析 YAML frontmatter 失败: %w", err)
	}

	return &meta, nil
}

// ParseFullSkillMD 完整解析 SKILL.md（frontmatter + 正文）
func ParseFullSkillMD(data []byte) (*SkillMeta, string, error) {
	content := string(data)
	parts := strings.SplitN(content, "---\n", 3)
	if len(parts) < 3 {
		return nil, "", fmt.Errorf("无效的 SKILL.md 格式: 缺少 frontmatter")
	}

	var meta SkillMeta
	if err := yaml.Unmarshal([]byte(parts[1]), &meta); err != nil {
		return nil, "", fmt.Errorf("解析 YAML frontmatter 失败: %w", err)
	}

	// parts[2] 是 YAML 结束标记后的所有内容
	body := strings.TrimSpace(parts[2])

	return &meta, body, nil
}

// WriteSkillMD 写入 SKILL.md 文件（frontmatter + 正文）
func WriteSkillMD(filePath string, meta SkillMeta, content string) error {
	metaYAML, err := yaml.Marshal(meta)
	if err != nil {
		return fmt.Errorf("序列化 frontmatter 失败: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString(string(metaYAML))
	sb.WriteString("---\n\n")
	sb.WriteString(content)
	sb.WriteString("\n")

	if err := os.WriteFile(filePath, []byte(sb.String()), 0644); err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}

	return nil
}

// truncateDesc 截断描述到指定长度
func truncateDesc(desc string, maxLen int) string {
	if len(desc) <= maxLen {
		return desc
	}
	target := maxLen - 3 // reserve space for "..."
	var lastBytePos int
	for bytePos := range desc {
		if bytePos >= target {
			return desc[:lastBytePos] + "..."
		}
		lastBytePos = bytePos
	}
	return desc[:lastBytePos] + "..."
}
