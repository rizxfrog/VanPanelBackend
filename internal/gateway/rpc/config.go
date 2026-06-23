package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/viper"
	"go.uber.org/zap"

	agentService "github.com/rizxfrog/VanPanelBackend/internal/agent/service"
	"github.com/rizxfrog/VanPanelBackend/internal/gateway"
)

// configSvc is set during server initialization to inject ConfigService into config handlers.
var configSvc *agentService.ConfigService

// SetConfigService sets the ConfigService for gateway config handlers.
func SetConfigService(svc *agentService.ConfigService) {
	configSvc = svc
}

func init() {
	gateway.RegisterMethod("config.get", string(gateway.ScopeRead), handleConfigGet)
	gateway.RegisterMethod("config.set", string(gateway.ScopeAdmin), handleConfigSet)
	gateway.RegisterMethod("config.apply", string(gateway.ScopeAdmin), handleConfigApply)
	gateway.RegisterMethod("config.patch", string(gateway.ScopeAdmin), handleConfigPatch)
	gateway.RegisterMethod("config.schema", string(gateway.ScopeAdmin), handleConfigSchema)
	gateway.RegisterMethod("config.schema.lookup", string(gateway.ScopeRead), handleConfigSchemaLookup)
	gateway.RegisterMethod("config.openFile", string(gateway.ScopeAdmin), handleConfigOK)
}

// configSchemaEntry 描述一个可配置项的元数据。
type configSchemaEntry struct {
	Key             string      `json:"key"`
	Type            string      `json:"type"`
	Default         interface{} `json:"default"`
	Description     string      `json:"description"`
	Writable        bool        `json:"writable"`
	RequiresRestart bool        `json:"requiresRestart"`
}

// knownConfigKeys 是 Gateway 暴露的运行时可配置项清单。
// YAML/环境变量作为默认值，DB 中的值作为运行时覆盖。
// Default 字段在运行时用 Viper 实际默认值填充，避免与部署配置不一致。
var knownConfigKeys = []configSchemaEntry{
	{Key: "agent.llm.provider", Type: "string", Description: "LLM 提供商", Writable: true, RequiresRestart: false},
	{Key: "agent.llm.base_url", Type: "string", Description: "LLM API 基础地址", Writable: true, RequiresRestart: false},
	{Key: "agent.llm.api_key", Type: "string", Description: "LLM API 密钥", Writable: true, RequiresRestart: false},
	{Key: "agent.llm.model", Type: "string", Description: "默认对话模型", Writable: true, RequiresRestart: false},
	{Key: "agent.llm.temperature", Type: "number", Description: "采样温度", Writable: true, RequiresRestart: false},
	{Key: "agent.llm.max_tokens", Type: "integer", Description: "最大生成 token 数", Writable: true, RequiresRestart: false},
	{Key: "agent.max_history", Type: "integer", Description: "会话历史消息保留条数", Writable: true, RequiresRestart: false},
}

var configKeyIndex = make(map[string]configSchemaEntry)

func init() {
	for _, e := range knownConfigKeys {
		configKeyIndex[e.Key] = e
	}
}

// maskSecret 对敏感配置值进行掩码处理，避免把完整 key 返回给前端。
// 规则：保留最后 4 位，其余用 * 替代；长度不足 4 位则全部隐藏。
func maskSecret(key string, value interface{}) interface{} {
	if !isSensitiveKey(key) {
		return value
	}
	s, ok := value.(string)
	if !ok {
		return "***"
	}
	if len(s) <= 4 {
		return strings.Repeat("*", len(s))
	}
	return strings.Repeat("*", len(s)-4) + s[len(s)-4:]
}

func isSensitiveKey(key string) bool {
	return strings.Contains(key, "api_key") || strings.Contains(key, "secret") || strings.Contains(key, "password")
}

// configValueResult 返回单个配置项的生效值、来源和默认值。
type configValueResult struct {
	Key          string      `json:"key"`
	Value        interface{} `json:"value"`
	Source       string      `json:"source"`
	DefaultValue interface{} `json:"defaultValue"`
	Description  string      `json:"description"`
}

// getDefaultForKey 返回当前部署环境下的默认值（YAML/env），并在 Viper 未初始化时返回硬编码兜底值。
func getDefaultForKey(key string) interface{} {
	if viper.IsSet(key) {
		return viper.Get(key)
	}
	switch key {
	case "agent.llm.provider":
		return "openai"
	case "agent.llm.base_url":
		return "https://api.openai.com/v1"
	case "agent.llm.api_key":
		return ""
	case "agent.llm.model":
		return "gpt-4o"
	case "agent.llm.temperature":
		return 0.7
	case "agent.llm.max_tokens":
		return 4096
	case "agent.max_history":
		return 20
	}
	return nil
}

func handleConfigGet(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	if configSvc == nil {
		return nil, fmt.Errorf("ConfigService 未初始化")
	}
	var req struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, fmt.Errorf("解析 config.get 参数失败: %w", err)
	}
	if req.Key == "" {
		return nil, fmt.Errorf("缺少 key 参数")
	}
	entry, ok := configKeyIndex[req.Key]
	if !ok {
		return nil, fmt.Errorf("未知配置项: %s", req.Key)
	}
	result := configValueResult{
		Key:          req.Key,
		DefaultValue: maskSecret(req.Key, getDefaultForKey(req.Key)),
		Description:  entry.Description,
	}

	// 优先读取 DB 运行时覆盖
	raw, err := configSvc.GetConfig(ctx, req.Key)
	if err == nil && raw != "" {
		var v interface{}
		if err := json.Unmarshal([]byte(raw), &v); err == nil {
			result.Value = maskSecret(req.Key, v)
			result.Source = "database"
			return result, nil
		}
	}

	// 回退到 YAML / 环境变量（Viper）
	result.Value = maskSecret(req.Key, getDefaultForKey(req.Key))
	result.Source = "default"
	return result, nil
}

func handleConfigSet(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	if configSvc == nil {
		return nil, fmt.Errorf("ConfigService 未初始化")
	}
	var req struct {
		Key   string      `json:"key"`
		Value interface{} `json:"value"`
		Apply bool        `json:"apply,omitempty"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, fmt.Errorf("解析 config.set 参数失败: %w", err)
	}
	if req.Key == "" {
		return nil, fmt.Errorf("缺少 key 参数")
	}
	if _, ok := configKeyIndex[req.Key]; !ok {
		return nil, fmt.Errorf("未知配置项: %s", req.Key)
	}

	// value 为空时删除 DB 覆盖，恢复默认值
	if req.Value == nil {
		if err := configSvc.DeleteConfig(ctx, req.Key); err != nil {
			return nil, fmt.Errorf("重置配置失败: %w", err)
		}
	} else {
		jsonVal, err := json.Marshal(req.Value)
		if err != nil {
			return nil, fmt.Errorf("序列化配置值失败: %w", err)
		}
		if err := configSvc.UpsertConfig(ctx, req.Key, string(jsonVal)); err != nil {
			return nil, fmt.Errorf("保存配置失败: %w", err)
		}
	}

	if req.Apply && agentSvc != nil {
		if err := applyRuntimeConfig(ctx); err != nil {
			zap.L().Warn("config.set 触发 apply 失败", zap.Error(err))
		}
	}

	return map[string]interface{}{"ok": true, "key": req.Key}, nil
}

func handleConfigApply(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	if configSvc == nil {
		return nil, fmt.Errorf("ConfigService 未初始化")
	}
	if agentSvc == nil {
		return nil, fmt.Errorf("AgentService 未初始化，无法应用配置")
	}
	if err := applyRuntimeConfig(ctx); err != nil {
		return nil, fmt.Errorf("应用配置失败: %w", err)
	}
	return map[string]interface{}{"ok": true}, nil
}

func handleConfigPatch(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	if configSvc == nil {
		return nil, fmt.Errorf("ConfigService 未初始化")
	}
	var req map[string]interface{}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, fmt.Errorf("解析 config.patch 参数失败: %w", err)
	}

	updated := make([]string, 0, len(req))
	for key, value := range req {
		if _, ok := configKeyIndex[key]; !ok {
			return nil, fmt.Errorf("未知配置项: %s", key)
		}
		jsonVal, err := json.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf("序列化 %s 失败: %w", key, err)
		}
		if err := configSvc.UpsertConfig(ctx, key, string(jsonVal)); err != nil {
			return nil, fmt.Errorf("保存 %s 失败: %w", key, err)
		}
		updated = append(updated, key)
	}

	if agentSvc != nil {
		if err := applyRuntimeConfig(ctx); err != nil {
			zap.L().Warn("config.patch 触发 apply 失败", zap.Error(err))
		}
	}

	return map[string]interface{}{"ok": true, "updated": updated}, nil
}

func handleConfigSchema(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	schema := make([]configSchemaEntry, len(knownConfigKeys))
	for i, e := range knownConfigKeys {
		e.Default = maskSecret(e.Key, getDefaultForKey(e.Key))
		schema[i] = e
	}
	return map[string]interface{}{
		"schema":      schema,
		"uiHints":     map[string]interface{}{},
		"version":     "1",
		"generatedAt": "",
	}, nil
}

func handleConfigSchemaLookup(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	var req struct {
		Keys []string `json:"keys"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, fmt.Errorf("解析 config.schema.lookup 参数失败: %w", err)
	}
	if len(req.Keys) == 0 {
		return []configSchemaEntry{}, nil
	}
	result := make([]configSchemaEntry, 0, len(req.Keys))
	for _, key := range req.Keys {
		if e, ok := configKeyIndex[key]; ok {
			e.Default = maskSecret(e.Key, getDefaultForKey(e.Key))
			result = append(result, e)
		}
	}
	return result, nil
}

func handleConfigOK(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]bool{"ok": true}, nil
}

// applyRuntimeConfig 从 DB 读取运行时配置覆盖并热重载到 AgentService。
func applyRuntimeConfig(ctx context.Context) error {
	if configSvc == nil || agentSvc == nil {
		return fmt.Errorf("ConfigService 或 AgentService 未初始化")
	}

	llm := agentService.LLMConfig{
		Provider:    viper.GetString("agent.llm.provider"),
		BaseURL:     viper.GetString("agent.llm.base_url"),
		APIKey:      viper.GetString("agent.llm.api_key"),
		Model:       viper.GetString("agent.llm.model"),
		Temperature: viper.GetFloat64("agent.llm.temperature"),
		MaxTokens:   viper.GetInt("agent.llm.max_tokens"),
	}
	maxHistory := viper.GetInt("agent.max_history")

	// 用 DB 值覆盖 YAML 默认值
	overrideString(ctx, "agent.llm.provider", &llm.Provider)
	overrideString(ctx, "agent.llm.base_url", &llm.BaseURL)
	overrideString(ctx, "agent.llm.api_key", &llm.APIKey)
	overrideString(ctx, "agent.llm.model", &llm.Model)
	overrideFloat64(ctx, "agent.llm.temperature", &llm.Temperature)
	overrideInt(ctx, "agent.llm.max_tokens", &llm.MaxTokens)
	overrideInt(ctx, "agent.max_history", &maxHistory)

	cfg := &agentService.Config{
		LLM:        llm,
		MaxHistory: maxHistory,
	}
	return agentSvc.ReloadConfig(ctx, cfg)
}

func overrideString(ctx context.Context, key string, target *string) {
	if configSvc == nil {
		return
	}
	raw, err := configSvc.GetConfig(ctx, key)
	if err != nil || raw == "" {
		return
	}
	var s string
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return
	}
	*target = s
}

func overrideFloat64(ctx context.Context, key string, target *float64) {
	if configSvc == nil {
		return
	}
	raw, err := configSvc.GetConfig(ctx, key)
	if err != nil || raw == "" {
		return
	}
	var f float64
	if err := json.Unmarshal([]byte(raw), &f); err != nil {
		return
	}
	*target = f
}

func overrideInt(ctx context.Context, key string, target *int) {
	if configSvc == nil {
		return
	}
	raw, err := configSvc.GetConfig(ctx, key)
	if err != nil || raw == "" {
		return
	}
	// JSON number 可能序列化为 float64，也可能为整数字符串
	var f float64
	if err := json.Unmarshal([]byte(raw), &f); err == nil {
		*target = int(f)
		return
	}
	var s string
	if err := json.Unmarshal([]byte(raw), &s); err == nil {
		if i, err := strconv.Atoi(s); err == nil {
			*target = i
		}
	}
}

// loadRuntimeConfig 在启动时从 DB 读取配置覆盖并应用到 AgentService。
// 供 main.go 调用，确保重启后使用的是 DB 中的运行时配置。
func LoadRuntimeConfig(ctx context.Context) error {
	return applyRuntimeConfig(ctx)
}
