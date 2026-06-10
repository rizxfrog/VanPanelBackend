package service

import (
	"context"
	"encoding/json"
	"regexp"

	agentDao "github.com/rizxfrog/VanPanelBackend/internal/agent/dao"
)

// InjectionRule 单条注入检测规则
type InjectionRule struct {
	ID      int            `json:"id"`
	Pattern string         `json:"pattern"`
	Desc    string         `json:"desc"`
	Enabled bool           `json:"enabled"`
	Re      *regexp.Regexp `json:"-"` // compiled regex, not serialized
}

// InjectionRulesConfig injection_rules 配置的 JSON 结构
type InjectionRulesConfig struct {
	Rules []InjectionRule `json:"rules"`
}

// LLMAuditPromptConfig llm_audit_prompt 配置的 JSON 结构
type LLMAuditPromptConfig struct {
	Enabled      bool    `json:"enabled"`
	Model        string  `json:"model"`
	Temperature  float64 `json:"temperature"`
	MaxTokens    int     `json:"max_tokens"`
	TimeoutSec   int     `json:"timeout_sec"`
	MaxRetries   int     `json:"max_retries"`
	SystemPrompt string  `json:"system_prompt"`
}

type ConfigService struct {
	dao *agentDao.AgentConfigDAO
}

func NewConfigService(dao *agentDao.AgentConfigDAO) *ConfigService {
	return &ConfigService{dao: dao}
}

// GetConfig returns the raw config value JSON string for a key
func (s *ConfigService) GetConfig(ctx context.Context, key string) (string, error) {
	cfg, err := s.dao.GetByKey(ctx, key)
	if err != nil {
		return "", err
	}
	return cfg.ConfigValue, nil
}

// ListConfigs returns all config keys with descriptions (no values)
func (s *ConfigService) ListConfigs(ctx context.Context) ([]map[string]string, error) {
	cfgs, err := s.dao.List(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]map[string]string, len(cfgs))
	for i, c := range cfgs {
		result[i] = map[string]string{
			"config_key":  c.ConfigKey,
			"description": c.Description,
		}
	}
	return result, nil
}

// UpsertConfig updates or creates a config entry
func (s *ConfigService) UpsertConfig(ctx context.Context, key string, value string) error {
	return s.dao.Upsert(ctx, key, value, "")
}

// GetInjectionRules loads injection regex rules from DB and compiles them
func (s *ConfigService) GetInjectionRules(ctx context.Context) ([]InjectionRule, error) {
	cfgJSON, err := s.GetConfig(ctx, "injection_rules")
	if err != nil {
		return nil, err
	}
	var rulesCfg InjectionRulesConfig
	if err := json.Unmarshal([]byte(cfgJSON), &rulesCfg); err != nil {
		return nil, err
	}
	for i := range rulesCfg.Rules {
		if rulesCfg.Rules[i].Enabled {
			re, err := regexp.Compile(rulesCfg.Rules[i].Pattern)
			if err != nil {
				continue
			}
			rulesCfg.Rules[i].Re = re
		}
	}
	return rulesCfg.Rules, nil
}

// GetLLMAuditPrompt loads the LLM audit prompt config from DB
func (s *ConfigService) GetLLMAuditPrompt(ctx context.Context) (*LLMAuditPromptConfig, error) {
	cfgJSON, err := s.GetConfig(ctx, "llm_audit_prompt")
	if err != nil {
		return nil, err
	}
	var promptCfg LLMAuditPromptConfig
	if err := json.Unmarshal([]byte(cfgJSON), &promptCfg); err != nil {
		return nil, err
	}
	return &promptCfg, nil
}
