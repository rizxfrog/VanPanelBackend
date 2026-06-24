package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rizxfrog/VanPanelBackend/internal/gateway"
)

func init() {
	gateway.RegisterMethod("tools.catalog", string(gateway.ScopeRead), handleToolsCatalog)
	gateway.RegisterMethod("tools.effective", string(gateway.ScopeRead), handleToolsEffective)
	gateway.RegisterMethod("tools.invoke", string(gateway.ScopeWrite), handleToolsInvoke)
}

// toolGroupInfo maps tool name prefixes to frontend group metadata.
var toolGroupInfo = map[string]struct {
	id    string
	label string
}{
	"net":        {id: "network", label: "Network"},
	"log":        {id: "log", label: "Log"},
	"proc":       {id: "process", label: "Process"},
	"disk":       {id: "disk", label: "Disk"},
	"sys":        {id: "system", label: "System"},
	"svc":        {id: "service", label: "Service"},
	"shell":      {id: "shell", label: "Shell"},
	"file":       {id: "file", label: "File"},
	"container":  {id: "container", label: "Container"},
	"prometheus": {id: "monitor", label: "Monitor"},
}

type toolGroup struct {
	ID     string      `json:"id"`
	Label  string      `json:"label"`
	Source string      `json:"source,omitempty"`
	Tools  []toolEntry `json:"tools"`
}

type toolEntry struct {
	ID              string   `json:"id"`
	Label           string   `json:"label"`
	Description     string   `json:"description"`
	Source          string   `json:"source,omitempty"`
	Optional        bool     `json:"optional,omitempty"`
	DefaultProfiles []string `json:"defaultProfiles,omitempty"`
}

func buildToolGroups(tools []map[string]interface{}) []toolGroup {
	groups := make(map[string]*toolGroup)
	fallback := &toolGroup{ID: "other", Label: "Other"}

	for _, t := range tools {
		name, _ := t["name"].(string)
		if name == "" {
			continue
		}
		desc, _ := t["description"].(string)

		prefix := strings.SplitN(name, ".", 2)[0]
		meta, ok := toolGroupInfo[prefix]
		if !ok {
			fallback.Tools = append(fallback.Tools, toolEntry{
				ID:          name,
				Label:       name,
				Description: desc,
				Source:      "core",
			})
			continue
		}

		g, exists := groups[meta.id]
		if !exists {
			g = &toolGroup{
				ID:     meta.id,
				Label:  meta.label,
				Source: "core",
			}
			groups[meta.id] = g
		}
		g.Tools = append(g.Tools, toolEntry{
			ID:              name,
			Label:           name,
			Description:     desc,
			Source:          "core",
			DefaultProfiles: []string{"default"},
		})
	}

	result := make([]toolGroup, 0, len(groups)+1)
	order := []string{"shell", "system", "service", "process", "network", "log", "disk", "file", "container", "monitor", "other"}
	for _, id := range order {
		if g, ok := groups[id]; ok {
			result = append(result, *g)
		}
	}
	if len(fallback.Tools) > 0 {
		result = append(result, *fallback)
	}
	return result
}

func handleToolsCatalog(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	if agentSvc == nil {
		return map[string]interface{}{
			"groups": []toolGroup{},
		}, nil
	}

	tools, err := agentSvc.ListTools(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取工具目录失败: %w", err)
	}

	return map[string]interface{}{
		"groups": buildToolGroups(tools),
	}, nil
}

func handleToolsEffective(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	if agentSvc == nil {
		return map[string]interface{}{
			"tools":        []toolEntry{},
			"defaultTools": []interface{}{},
			"sessionTools": map[string]interface{}{},
		}, nil
	}

	tools, err := agentSvc.ListTools(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取有效工具失败: %w", err)
	}

	entries := make([]toolEntry, 0, len(tools))
	for _, t := range tools {
		name, _ := t["name"].(string)
		desc, _ := t["description"].(string)
		if name == "" {
			continue
		}
		entries = append(entries, toolEntry{
			ID:          name,
			Label:       name,
			Description: desc,
			Source:      "core",
		})
	}

	return map[string]interface{}{
		"tools":        entries,
		"defaultTools": []interface{}{},
		"sessionTools": map[string]interface{}{},
	}, nil
}

func handleToolsInvoke(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	if agentSvc == nil {
		return map[string]interface{}{
			"result": "",
			"error":  "AgentService 未初始化",
		}, nil
	}

	var req struct {
		ToolName string                 `json:"toolName"`
		Name     string                 `json:"name"`
		Args     map[string]interface{} `json:"args"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, fmt.Errorf("解析参数失败: %w", err)
	}

	toolName := req.ToolName
	if toolName == "" {
		toolName = req.Name
	}
	if toolName == "" {
		return nil, fmt.Errorf("toolName 不能为空")
	}

	argsJSON := "{}"
	if req.Args != nil {
		b, err := json.Marshal(req.Args)
		if err != nil {
			return nil, fmt.Errorf("序列化参数失败: %w", err)
		}
		argsJSON = string(b)
	}

	result, err := agentSvc.InvokeTool(ctx, toolName, argsJSON)
	if err != nil {
		return map[string]interface{}{
			"result": "",
			"error":  err.Error(),
		}, nil
	}

	return map[string]interface{}{
		"result": result,
		"error":  nil,
	}, nil
}
