package skill

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"go.uber.org/zap"
)

const defaultUsageFile = ".usage.json"

// loadUsage 从 .usage.json 加载使用统计
func (s *SkillStore) loadUsage() (map[string]*SkillUsage, error) {
	data, err := os.ReadFile(s.usagePath)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]*SkillUsage), nil
		}
		return nil, fmt.Errorf("读取使用统计文件失败: %w", err)
	}

	var usage map[string]*SkillUsage
	if err := json.Unmarshal(data, &usage); err != nil {
		return nil, fmt.Errorf("解析使用统计失败: %w", err)
	}
	return usage, nil
}

// saveUsage 将使用统计写入 .usage.json
func (s *SkillStore) saveUsage(usage map[string]*SkillUsage) error {
	data, err := json.MarshalIndent(usage, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化使用统计失败: %w", err)
	}

	if err := os.WriteFile(s.usagePath, data, 0644); err != nil {
		return fmt.Errorf("写入使用统计文件失败: %w", err)
	}
	return nil
}

// getUsage 获取指定 skill 的使用统计，不存在则返回 nil
func (s *SkillStore) getUsage(name string) *SkillUsage {
	usage, err := s.loadUsage()
	if err != nil {
		s.logger.Warn("加载使用统计失败", zap.String("skill", name), zap.Error(err))
		return nil
	}
	return usage[name]
}

// updateUsage 原子更新指定 skill 的使用统计
func (s *SkillStore) updateUsage(name string, fn func(*SkillUsage)) {
	s.mu.Lock()
	defer s.mu.Unlock()

	usage, err := s.loadUsage()
	if err != nil {
		s.logger.Warn("加载使用统计失败", zap.String("skill", name), zap.Error(err))
		return
	}

	u, ok := usage[name]
	if !ok {
		u = &SkillUsage{
			Name:           name,
			LastActivityAt: time.Now(),
			State:          SkillStateActive,
		}
		usage[name] = u
	}
	fn(u)
	u.LastActivityAt = time.Now()

	if err := s.saveUsage(usage); err != nil {
		s.logger.Warn("保存使用统计失败", zap.String("skill", name), zap.Error(err))
	}
}

// removeUsage 从 .usage.json 中移除指定 skill
func (s *SkillStore) removeUsage(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	usage, err := s.loadUsage()
	if err != nil {
		s.logger.Warn("加载使用统计失败", zap.String("skill", name), zap.Error(err))
		return
	}

	delete(usage, name)

	if err := s.saveUsage(usage); err != nil {
		s.logger.Warn("保存使用统计失败", zap.String("skill", name), zap.Error(err))
	}
}
