package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rizxfrog/VanPanelBackend/internal/gateway"
	"github.com/rizxfrog/VanPanelBackend/internal/model"
	"go.uber.org/zap"
)

func init() {
	gateway.RegisterMethod("agents.list", string(gateway.ScopeRead), handleAgentsList)
	gateway.RegisterMethod("agents.create", string(gateway.ScopeAdmin), handleAgentsCreate)
	gateway.RegisterMethod("agents.update", string(gateway.ScopeAdmin), handleAgentsUpdate)
	gateway.RegisterMethod("agents.delete", string(gateway.ScopeAdmin), handleAgentsDelete)
	gateway.RegisterMethod("agents.files.list", string(gateway.ScopeRead), handleAgentsFilesList)
	gateway.RegisterMethod("agents.files.get", string(gateway.ScopeRead), handleAgentsFilesGet)
	gateway.RegisterMethod("agents.files.set", string(gateway.ScopeAdmin), handleAgentsFilesSet)
}

func handleAgentsList(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	if agentSvc == nil {
		// 返回默认 agent
		return map[string]interface{}{
			"agents": []map[string]interface{}{
				{"id": "main", "name": "VanPanel Agent", "model": "gpt-4o", "status": "ready"},
			},
			"defaultId": "main",
			"mainKey":   "agent:main:global",
			"scope":     "global",
		}, nil
	}

	agents, err := agentSvc.ListAgents(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取Agent列表失败: %w", err)
	}

	if len(agents) == 0 {
		// 自动创建默认 agent
		defaultAgent := &model.GatewayAgent{
			AgentID: "main",
			Name:    "VanPanel Agent",
			Model:   "gpt-4o",
			Status:  "ready",
		}
		if createErr := agentSvc.CreateAgent(ctx, defaultAgent); createErr != nil {
			// 创建失败也返回硬编码的默认值
			zap.L().Warn("创建默认Agent失败，返回默认值", zap.Error(createErr))
		}
		return map[string]interface{}{
			"agents": []map[string]interface{}{
				{"id": "main", "name": "VanPanel Agent", "model": "gpt-4o", "status": "ready"},
			},
			"defaultId": "main",
			"mainKey":   "agent:main:global",
			"scope":     "global",
		}, nil
	}

	rows := make([]map[string]interface{}, 0, len(agents))
	defaultID := agents[0].AgentID
	for _, a := range agents {
		row := map[string]interface{}{
			"id":     a.AgentID,
			"name":   a.Name,
			"model":  a.Model,
			"status": a.Status,
		}
		if a.ID == 0 {
			defaultID = a.AgentID
		}
		rows = append(rows, row)
	}

	return map[string]interface{}{
		"agents":    rows,
		"defaultId": defaultID,
		"mainKey":   fmt.Sprintf("agent:%s:global", defaultID),
		"scope":     "global",
	}, nil
}

func handleAgentsCreate(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	var req struct {
		AgentID      string `json:"id"`
		Name         string `json:"name"`
		Model        string `json:"model,omitempty"`
		SystemPrompt string `json:"systemPrompt,omitempty"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, fmt.Errorf("解析参数失败: %w", err)
	}

	if req.AgentID == "" || req.Name == "" {
		return nil, fmt.Errorf("id 和 name 不能为空")
	}

	if err := requireAgentSvc(); err != nil {
		return nil, err
	}

	// 检查是否已存在
	existing, _ := agentSvc.GetAgent(ctx, req.AgentID)
	if existing != nil {
		return nil, fmt.Errorf("Agent %s 已存在", req.AgentID)
	}

	modelName := req.Model
	if modelName == "" {
		modelName = "gpt-4o"
	}

	agent := &model.GatewayAgent{
		AgentID:      req.AgentID,
		Name:         req.Name,
		Model:        modelName,
		SystemPrompt: req.SystemPrompt,
		Status:       "ready",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := agentSvc.CreateAgent(ctx, agent); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"ok":   true,
		"id":   req.AgentID,
		"name": req.Name,
	}, nil
}

func handleAgentsUpdate(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	var req struct {
		AgentID      string  `json:"id"`
		Name         *string `json:"name,omitempty"`
		Model        *string `json:"model,omitempty"`
		SystemPrompt *string `json:"systemPrompt,omitempty"`
		Status       *string `json:"status,omitempty"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, fmt.Errorf("解析参数失败: %w", err)
	}

	if req.AgentID == "" {
		return nil, fmt.Errorf("id 不能为空")
	}

	if err := requireAgentSvc(); err != nil {
		return nil, err
	}

	agent, err := agentSvc.GetAgent(ctx, req.AgentID)
	if err != nil {
		return nil, err
	}
	if agent == nil {
		return nil, fmt.Errorf("Agent %s 不存在", req.AgentID)
	}

	if req.Name != nil {
		agent.Name = *req.Name
	}
	if req.Model != nil {
		agent.Model = *req.Model
	}
	if req.SystemPrompt != nil {
		agent.SystemPrompt = *req.SystemPrompt
	}
	if req.Status != nil {
		agent.Status = *req.Status
	}
	agent.UpdatedAt = time.Now()

	if err := agentSvc.UpdateAgent(ctx, agent); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"ok":   true,
		"id":   req.AgentID,
		"name": agent.Name,
	}, nil
}

func handleAgentsDelete(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	var req struct {
		AgentID string `json:"id"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, fmt.Errorf("解析参数失败: %w", err)
	}

	if req.AgentID == "" {
		return nil, fmt.Errorf("id 不能为空")
	}

	if err := requireAgentSvc(); err != nil {
		return nil, err
	}

	if err := agentSvc.DeleteAgent(ctx, req.AgentID); err != nil {
		return nil, err
	}

	return map[string]interface{}{"ok": true}, nil
}

// --- Agent Files ---

func agentFilesDir(agentID string) string {
	// Default agents dir
	return filepath.Join("data", "agents", agentID)
}

func handleAgentsFilesList(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	var req struct {
		AgentID string `json:"agentId"`
	}
	json.Unmarshal(params, &req)
	agentID := req.AgentID
	if agentID == "" {
		agentID = "main"
	}

	dir := agentFilesDir(agentID)
	files := make([]map[string]interface{}, 0)

	entries, err := os.ReadDir(dir)
	if err != nil {
		// Directory doesn't exist yet, return empty
		return map[string]interface{}{
			"agentId":   agentID,
			"workspace": dir,
			"files":     files,
		}, nil
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		filePath := filepath.Join(dir, entry.Name())
		contentBytes, readErr := os.ReadFile(filePath)
		content := ""
		if readErr == nil {
			content = string(contentBytes)
		}
		files = append(files, map[string]interface{}{
			"name":       entry.Name(),
			"path":       filePath,
			"missing":    false,
			"size":       info.Size(),
			"updatedAtMs": info.ModTime().UnixMilli(),
			"content":    content,
		})
	}

	return map[string]interface{}{
		"agentId":   agentID,
		"workspace": dir,
		"files":     files,
	}, nil
}

func handleAgentsFilesGet(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	var req struct {
		AgentID string `json:"agentId"`
		Name    string `json:"name"`
	}
	json.Unmarshal(params, &req)
	agentID := req.AgentID
	if agentID == "" {
		agentID = "main"
	}

	dir := agentFilesDir(agentID)
	filePath := filepath.Join(dir, req.Name)

	contentBytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, nil
	}

	info, _ := os.Stat(filePath)
	content := string(contentBytes)

	return map[string]interface{}{
		"agentId":   agentID,
		"workspace": dir,
		"file": map[string]interface{}{
			"name":       req.Name,
			"path":       filePath,
			"missing":    false,
			"size":       info.Size(),
			"updatedAtMs": info.ModTime().UnixMilli(),
			"content":    content,
		},
	}, nil
}

func handleAgentsFilesSet(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	var req struct {
		AgentID string `json:"agentId"`
		Name    string `json:"name"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, fmt.Errorf("解析参数失败: %w", err)
	}

	agentID := req.AgentID
	if agentID == "" {
		agentID = "main"
	}
	if req.Name == "" {
		return nil, fmt.Errorf("文件名不能为空")
	}

	dir := agentFilesDir(agentID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("创建Agent目录失败: %w", err)
	}

	// 安全检查：防止路径遍历
	name := strings.ReplaceAll(req.Name, "..", "")
	name = strings.TrimPrefix(name, "/")
	filePath := filepath.Join(dir, name)

	if err := os.WriteFile(filePath, []byte(req.Content), 0644); err != nil {
		return nil, fmt.Errorf("写入文件失败: %w", err)
	}

	info, _ := os.Stat(filePath)

	return map[string]interface{}{
		"ok":        true,
		"agentId":   agentID,
		"workspace": dir,
		"file": map[string]interface{}{
			"name":       name,
			"path":       filePath,
			"missing":    false,
			"size":       info.Size(),
			"updatedAtMs": info.ModTime().UnixMilli(),
			"content":    req.Content,
		},
	}, nil
}
