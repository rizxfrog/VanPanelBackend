package rpc

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/rizxfrog/VanPanelBackend/internal/gateway"
)

func init() {
	gateway.RegisterMethod("models.list", string(gateway.ScopeRead), handleModelsList)
	gateway.RegisterMethod("models.authStatus", string(gateway.ScopeRead), handleModelsAuthStatus)
	gateway.RegisterMethod("models.authLogout", string(gateway.ScopeAdmin), handleModelsAuthLogout)
}

// modelCatalogEntry 与前端 ModelCatalogEntry 类型对齐。
type modelCatalogEntry struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Provider      string   `json:"provider"`
	Alias         string   `json:"alias,omitempty"`
	ContextWindow int      `json:"contextWindow,omitempty"`
	Reasoning     bool     `json:"reasoning,omitempty"`
	Input         []string `json:"input,omitempty"`
}

func handleModelsList(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	if agentSvc == nil {
		return nil, fmt.Errorf("AgentService 未初始化")
	}

	catalog, err := agentSvc.GetModelCatalog(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取模型目录失败: %w", err)
	}

	models := make([]modelCatalogEntry, 0)
	for _, provider := range catalog.Providers {
		for _, model := range provider.Models {
			entry := modelCatalogEntry{
				ID:       model.ID,
				Name:     model.Name,
				Provider: provider.ID,
			}
			if model.ID == catalog.DefaultModel {
				entry.Alias = "default"
			}
			// 根据模型 ID 推断上下文窗口和能力
			entry.ContextWindow = inferContextWindow(model.ID)
			entry.Input = inferInputTypes(model.ID)
			models = append(models, entry)
		}
	}

	return map[string]interface{}{
		"models": models,
	}, nil
}

// inferContextWindow 根据常见模型 ID 推断上下文窗口。
func inferContextWindow(modelID string) int {
	switch {
	case contains(modelID, "gpt-4o") || contains(modelID, "gpt-4-turbo") || contains(modelID, "claude-3"):
		return 128000
	case contains(modelID, "gpt-4"):
		return 8192
	case contains(modelID, "gpt-3.5"):
		return 16384
	case contains(modelID, "mimo"):
		return 128000
	default:
		return 128000
	}
}

// inferInputTypes 根据常见模型 ID 推断支持的输入类型。
func inferInputTypes(modelID string) []string {
	if contains(modelID, "gpt-4o") || contains(modelID, "claude-3") || contains(modelID, "gemini") {
		return []string{"text", "image", "document"}
	}
	return []string{"text"}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsInternal(s, substr))
}

func containsInternal(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// modelAuthStatusProvider 与前端 ModelAuthStatusProvider 对齐。
type modelAuthStatusProvider struct {
	Name       string `json:"name"`
	Authorized bool   `json:"authorized"`
}

func handleModelsAuthStatus(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	if agentSvc == nil {
		return nil, fmt.Errorf("AgentService 未初始化")
	}

	catalog, err := agentSvc.GetModelCatalog(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取模型目录失败: %w", err)
	}

	providerName := "openai"
	if len(catalog.Providers) > 0 && catalog.Providers[0].ID != "" {
		providerName = catalog.Providers[0].ID
	}

	// 从当前 LLM 配置判断认证状态：存在 api_key 即认为已授权
	authorized := false
	if cfg := agentSvc.GetConfig(); cfg != nil {
		authorized = cfg.LLM.APIKey != ""
	}

	return map[string]interface{}{
		"providers": []modelAuthStatusProvider{
			{Name: providerName, Authorized: authorized},
		},
	}, nil
}

// handleModelsAuthLogout 清除 LLM API key 运行时覆盖，恢复为默认空值。
func handleModelsAuthLogout(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	if configSvc == nil {
		return nil, fmt.Errorf("ConfigService 未初始化")
	}
	if agentSvc == nil {
		return nil, fmt.Errorf("AgentService 未初始化")
	}

	if err := configSvc.DeleteConfig(ctx, "agent.llm.api_key"); err != nil {
		return nil, fmt.Errorf("登出模型授权失败: %w", err)
	}

	if err := applyRuntimeConfig(ctx); err != nil {
		return nil, fmt.Errorf("重载配置失败: %w", err)
	}

	return map[string]bool{"ok": true}, nil
}
