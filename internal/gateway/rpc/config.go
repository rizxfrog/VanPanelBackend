package rpc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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

// configSnapshot 与前端 ConfigSnapshot 类型对齐。
type configSnapshot struct {
	Config       map[string]interface{} `json:"config"`
	Resolved     map[string]interface{} `json:"resolved"`
	SourceConfig map[string]interface{} `json:"sourceConfig"`
	Raw          string                 `json:"raw"`
	Valid        bool                   `json:"valid"`
	Issues       []configIssue          `json:"issues"`
	Hash         string                 `json:"hash"`
}

type configIssue struct {
	Path    string `json:"path"`
	Message string `json:"message"`
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

// getEffectiveValue 返回某个 key 的生效值（DB 优先，否则 Viper 默认值），并对敏感值掩码。
func getEffectiveValue(ctx context.Context, key string) interface{} {
	if configSvc == nil {
		return maskSecret(key, getDefaultForKey(key))
	}
	raw, err := configSvc.GetConfig(ctx, key)
	if err == nil && raw != "" {
		var v interface{}
		if err := json.Unmarshal([]byte(raw), &v); err == nil {
			return maskSecret(key, v)
		}
	}
	return maskSecret(key, getDefaultForKey(key))
}

// isDBOverridden 判断某个 key 是否在 DB 中有运行时覆盖。
func isDBOverridden(ctx context.Context, key string) bool {
	if configSvc == nil {
		return false
	}
	raw, err := configSvc.GetConfig(ctx, key)
	return err == nil && raw != ""
}

// buildConfigObject 构造完整的嵌套配置对象。
// includeDefaults 为 true 时包含默认值，为 false 时只包含 DB 覆盖值。
func buildConfigObject(ctx context.Context, includeDefaults bool) map[string]interface{} {
	llm := map[string]interface{}{}
	if includeDefaults || isDBOverridden(ctx, "agent.llm.provider") {
		llm["provider"] = getEffectiveValue(ctx, "agent.llm.provider")
	}
	if includeDefaults || isDBOverridden(ctx, "agent.llm.base_url") {
		llm["base_url"] = getEffectiveValue(ctx, "agent.llm.base_url")
	}
	if includeDefaults || isDBOverridden(ctx, "agent.llm.api_key") {
		llm["api_key"] = getEffectiveValue(ctx, "agent.llm.api_key")
	}
	if includeDefaults || isDBOverridden(ctx, "agent.llm.model") {
		llm["model"] = getEffectiveValue(ctx, "agent.llm.model")
	}
	if includeDefaults || isDBOverridden(ctx, "agent.llm.temperature") {
		llm["temperature"] = getEffectiveValue(ctx, "agent.llm.temperature")
	}
	if includeDefaults || isDBOverridden(ctx, "agent.llm.max_tokens") {
		llm["max_tokens"] = getEffectiveValue(ctx, "agent.llm.max_tokens")
	}

	agent := map[string]interface{}{"llm": llm}
	if includeDefaults || isDBOverridden(ctx, "agent.max_history") {
		agent["max_history"] = getEffectiveValue(ctx, "agent.max_history")
	}

	return map[string]interface{}{"agent": agent}
}

// buildSnapshot 构造前端 ConfigSnapshot。
func buildSnapshot(ctx context.Context) (*configSnapshot, error) {
	config := buildConfigObject(ctx, true)
	sourceConfig := buildConfigObject(ctx, false)

	rawBytes, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("序列化配置失败: %w", err)
	}
	raw := string(rawBytes) + "\n"
	hash := sha256.Sum256([]byte(raw))

	return &configSnapshot{
		Config:       config,
		Resolved:     config,
		SourceConfig: sourceConfig,
		Raw:          raw,
		Valid:        true,
		Issues:       []configIssue{},
		Hash:         hex.EncodeToString(hash[:]),
	}, nil
}

// setPathValue 在嵌套 map 中按点分路径设置值。
func setPathValue(target map[string]interface{}, path string, value interface{}) {
	parts := strings.Split(path, ".")
	current := target
	for i := 0; i < len(parts)-1; i++ {
		key := parts[i]
		next, ok := current[key].(map[string]interface{})
		if !ok {
			next = make(map[string]interface{})
			current[key] = next
		}
		current = next
	}
	current[parts[len(parts)-1]] = value
}

// flattenConfigObject 将嵌套配置对象展开为平铺的 key-value 映射。
func flattenConfigObject(prefix string, obj map[string]interface{}, result map[string]interface{}) {
	for k, v := range obj {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		switch vv := v.(type) {
		case map[string]interface{}:
			flattenConfigObject(key, vv, result)
		default:
			result[key] = v
		}
	}
}

// applyRawConfig 解析前端提交的 raw JSON 配置并更新 DB。
func applyRawConfig(ctx context.Context, raw string) error {
	if configSvc == nil {
		return fmt.Errorf("ConfigService 未初始化")
	}
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &obj); err != nil {
		return fmt.Errorf("解析 raw 配置失败: %w", err)
	}

	flat := make(map[string]interface{})
	flattenConfigObject("", obj, flat)

	for key, value := range flat {
		if _, ok := configKeyIndex[key]; !ok {
			// 忽略未知 key
			continue
		}
		jsonVal, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("序列化 %s 失败: %w", key, err)
		}
		if err := configSvc.UpsertConfig(ctx, key, string(jsonVal)); err != nil {
			return fmt.Errorf("保存 %s 失败: %w", key, err)
		}
	}
	return nil
}

// buildJsonSchema 构造前端需要的 JSON Schema 根对象。
func buildJsonSchema() map[string]interface{} {
	root := map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
	for _, e := range knownConfigKeys {
		parts := strings.Split(e.Key, ".")
		attachSchemaProperty(root, parts, e)
	}
	return root
}

// attachSchemaProperty 把叶子 schema 按点分路径挂到嵌套 properties 中。
func attachSchemaProperty(parent map[string]interface{}, parts []string, entry configSchemaEntry) {
	props := parent["properties"].(map[string]interface{})
	for i := 0; i < len(parts)-1; i++ {
		part := parts[i]
		node, ok := props[part].(map[string]interface{})
		if !ok {
			node = map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			}
			props[part] = node
		}
		props = node["properties"].(map[string]interface{})
	}
	leaf := map[string]interface{}{
		"type":        jsonSchemaType(entry.Type),
		"title":       entry.Description,
		"description": entry.Description,
		"default":     maskSecret(entry.Key, getDefaultForKey(entry.Key)),
	}
	props[parts[len(parts)-1]] = leaf
}

// jsonSchemaType 把内部类型名转换为 JSON Schema 类型。
func jsonSchemaType(t string) string {
	switch t {
	case "integer":
		return "integer"
	case "number":
		return "number"
	case "boolean":
		return "boolean"
	default:
		return "string"
	}
}

// buildJsonSchemaForPath 根据点分路径返回对应的子 schema；未知路径返回空 object。
func buildJsonSchemaForPath(path string) map[string]interface{} {
	parts := strings.Split(path, ".")
	root := buildJsonSchema()
	current := root
	for _, part := range parts {
		props, ok := current["properties"].(map[string]interface{})
		if !ok {
			return map[string]interface{}{}
		}
		next, ok := props[part].(map[string]interface{})
		if !ok {
			return map[string]interface{}{}
		}
		current = next
	}
	return current
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

	// 前端默认调用 config.get({}) 获取完整 ConfigSnapshot
	if req.Key == "" {
		return buildSnapshot(ctx)
	}

	entry, ok := configKeyIndex[req.Key]
	if !ok {
		return nil, fmt.Errorf("未知配置项: %s", req.Key)
	}
	result := configValueResult{
		Key:          req.Key,
		DefaultValue: maskSecret(req.Key, getDefaultForKey(req.Key)),
		Description:  entry.Description,
		Value:        getEffectiveValue(ctx, req.Key),
		Source: func() string {
			if isDBOverridden(ctx, req.Key) {
				return "database"
			}
			return "default"
		}(),
	}
	return result, nil
}

func handleConfigSet(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	if configSvc == nil {
		return nil, fmt.Errorf("ConfigService 未初始化")
	}
	var req struct {
		Key      string      `json:"key"`
		Value    interface{} `json:"value"`
		Apply    bool        `json:"apply,omitempty"`
		Raw      string      `json:"raw"`
		BaseHash string      `json:"baseHash"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, fmt.Errorf("解析 config.set 参数失败: %w", err)
	}

	// 前端高级编辑器调用 config.set({ raw: "...", baseHash: "..." })
	if req.Raw != "" {
		if err := applyRawConfig(ctx, req.Raw); err != nil {
			return nil, err
		}
		if agentSvc != nil {
			if err := applyRuntimeConfig(ctx); err != nil {
				zap.L().Warn("config.set raw 触发 apply 失败", zap.Error(err))
			}
		}
		return map[string]interface{}{"ok": true, "status": "ok"}, nil
	}

	// 单 key 模式（工具/测试脚本使用）
	if req.Key == "" {
		return nil, fmt.Errorf("缺少 key 或 raw 参数")
	}
	if _, ok := configKeyIndex[req.Key]; !ok {
		return nil, fmt.Errorf("未知配置项: %s", req.Key)
	}

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

	var req struct {
		Raw      string `json:"raw"`
		BaseHash string `json:"baseHash"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, fmt.Errorf("解析 config.apply 参数失败: %w", err)
	}
	if req.Raw != "" {
		if err := applyRawConfig(ctx, req.Raw); err != nil {
			return nil, err
		}
	}
	if err := applyRuntimeConfig(ctx); err != nil {
		return nil, fmt.Errorf("应用配置失败: %w", err)
	}
	return map[string]interface{}{"ok": true, "status": "ok"}, nil
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
	uiHints := map[string]interface{}{
		"agent.llm.api_key": map[string]interface{}{"sensitive": true},
	}
	return map[string]interface{}{
		"schema":      buildJsonSchema(),
		"uiHints":     uiHints,
		"version":     "1",
		"generatedAt": "",
	}, nil
}

func handleConfigSchemaLookup(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	var req struct {
		Keys []string `json:"keys"`
		Path string   `json:"path"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, fmt.Errorf("解析 config.schema.lookup 参数失败: %w", err)
	}

	// 前端 dreaming 等使用 path 查询，返回该路径对应的 JSON Schema
	if req.Path != "" {
		return buildJsonSchemaForPath(req.Path), nil
	}

	// keys 模式（向后兼容）返回平铺 schema 列表
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
