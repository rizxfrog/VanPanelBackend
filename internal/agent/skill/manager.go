package skill

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// SkillManageRequest skill_manage 工具参数
type SkillManageRequest struct {
	Action      string `json:"action"`       // list|view|create|edit|patch|delete|write_file|remove_file|pin|unpin
	Name        string `json:"name,omitempty"`
	Category    string `json:"category,omitempty"`
	Description string `json:"description,omitempty"`
	Content     string `json:"content,omitempty"`
	OldString   string `json:"old_string,omitempty"`
	NewString   string `json:"new_string,omitempty"`
	FilePath    string `json:"file_path,omitempty"`
	FileContent string `json:"file_content,omitempty"`
}

// SkillManagerTool skill_manage Eino 工具
type SkillManagerTool struct {
	store *SkillStore
}

func NewSkillManagerTool(store *SkillStore) tool.InvokableTool {
	return &SkillManagerTool{store: store}
}

func (t *SkillManagerTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "skill_manage",
		Desc: `管理 skill 库。Skills 是可复用的过程性知识，用来高效完成特定类型任务。

操作:
  list   — 列出所有 skills (只返回名称和描述)
  view   — 查看 skill 完整内容
  create — 创建新 skill (需要 name, category, description, content)
  edit   — 替换 SKILL.md 全文
  patch  — 定向替换 (old_string → new_string)
  delete — 删除 skill
  write_file  — 写入参考文件 (references/ 或 templates/)
  remove_file — 删除参考文件
  pin    — 固定 skill (防止自动归档)
  unpin  — 取消固定`,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"action": {
				Type:     "string",
				Desc:     "操作类型: list, view, create, edit, patch, delete, write_file, remove_file, pin, unpin",
				Required: true,
				Enum:     []string{"list", "view", "create", "edit", "patch", "delete", "write_file", "remove_file", "pin", "unpin"},
			},
			"name": {
				Type: "string",
				Desc: "skill 名称",
			},
			"category": {
				Type: "string",
				Desc: "分类 (create 时使用)",
			},
			"description": {
				Type: "string",
				Desc: "描述 (create 时使用, 最多1024字符)",
			},
			"content": {
				Type: "string",
				Desc: "SKILL.md 内容 (create/edit 时使用)",
			},
			"old_string": {
				Type: "string",
				Desc: "要替换的旧文本 (patch 时使用)",
			},
			"new_string": {
				Type: "string",
				Desc: "替换后的新文本 (patch 时使用)",
			},
			"file_path": {
				Type: "string",
				Desc: "文件路径 (write_file/remove_file 时使用)",
			},
			"file_content": {
				Type: "string",
				Desc: "文件内容 (write_file 时使用)",
			},
		}),
	}, nil
}

func (t *SkillManagerTool) InvokableRun(ctx context.Context, params string, opts ...tool.Option) (string, error) {
	var req SkillManageRequest
	if err := json.Unmarshal([]byte(params), &req); err != nil {
		return "", fmt.Errorf("解析 skill_manage 参数失败: %w", err)
	}

	switch req.Action {
	case "list":
		return t.handleList(ctx)
	case "view":
		return t.handleView(ctx, req)
	case "create":
		return t.handleCreate(ctx, req)
	case "edit":
		return t.handleEdit(ctx, req)
	case "patch":
		return t.handlePatch(ctx, req)
	case "delete":
		return t.handleDelete(ctx, req)
	case "write_file":
		return t.handleWriteFile(ctx, req)
	case "remove_file":
		return t.handleRemoveFile(ctx, req)
	case "pin":
		return t.handlePin(ctx, req)
	case "unpin":
		return t.handleUnpin(ctx, req)
	default:
		return "", fmt.Errorf("未知操作: %s", req.Action)
	}
}

// ==================== handler implementations ====================

func (t *SkillManagerTool) handleList(ctx context.Context) (string, error) {
	skills, err := t.store.ListSkills(ctx)
	if err != nil {
		return errorJSON(err.Error()), nil
	}

	type listItem struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Category    string `json:"category"`
		State       string `json:"state"`
		UseCount    int    `json:"use_count"`
	}

	items := make([]listItem, 0, len(skills))
	for _, s := range skills {
		items = append(items, listItem{
			Name:        s.Meta.Name,
			Description: s.Meta.Description,
			Category:    s.Meta.Category,
			State:       string(s.State),
			UseCount:    s.UseCount,
		})
	}

	result, _ := json.Marshal(map[string]interface{}{
		"success": true,
		"data":    items,
	})
	return string(result), nil
}

func (t *SkillManagerTool) handleView(ctx context.Context, req SkillManageRequest) (string, error) {
	if req.Name == "" {
		return errorJSON("name is required for view"), nil
	}

	if req.FilePath != "" {
		data, err := t.store.GetSkillFile(ctx, req.Name, req.FilePath)
		if err != nil {
			return errorJSON(err.Error()), nil
		}
		result, _ := json.Marshal(map[string]interface{}{
			"success": true,
			"data": map[string]interface{}{
				"name":      req.Name,
				"file_path": req.FilePath,
				"content":   string(data),
			},
		})
		return string(result), nil
	}

	skill, err := t.store.GetSkill(ctx, req.Name)
	if err != nil {
		return errorJSON(err.Error()), nil
	}

	// Reconstruct full SKILL.md content (frontmatter + body)
	metaYAML, _ := json.Marshal(skill.Meta)
	var fullContent strings.Builder
	fullContent.WriteString("---\n")
	fullContent.WriteString(string(metaYAML))
	fullContent.WriteString("\n---\n\n")
	fullContent.WriteString(skill.Content)

	result, _ := json.Marshal(map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"name":        skill.Meta.Name,
			"description": skill.Meta.Description,
			"category":    skill.Meta.Category,
			"content":     fullContent.String(),
			"state":       string(skill.State),
			"use_count":   skill.UseCount,
			"patch_count": skill.PatchCount,
			"path":        skill.Path,
		},
	})
	return string(result), nil
}

func (t *SkillManagerTool) handleCreate(ctx context.Context, req SkillManageRequest) (string, error) {
	if req.Name == "" {
		return errorJSON("name is required for create"), nil
	}
	if req.Content == "" {
		return errorJSON("content is required for create"), nil
	}

	meta := SkillMeta{
		Name:        req.Name,
		Description: req.Description,
		Category:    req.Category,
		CreatedBy:   "agent",
	}

	skill, err := t.store.CreateSkill(ctx, meta, req.Content, SkillSourceAgent)
	if err != nil {
		return errorJSON(err.Error()), nil
	}

	result, _ := json.Marshal(map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"name": skill.Meta.Name,
			"path": skill.Path,
		},
	})
	return string(result), nil
}

func (t *SkillManagerTool) handleEdit(ctx context.Context, req SkillManageRequest) (string, error) {
	if req.Name == "" {
		return errorJSON("name is required for edit"), nil
	}
	if req.Content == "" {
		return errorJSON("content is required for edit"), nil
	}

	// Get current skill to obtain old content for full replacement
	oldSkill, err := t.store.GetSkill(ctx, req.Name)
	if err != nil {
		return errorJSON(err.Error()), nil
	}

	// Use PatchSkill with full content replacement
	patched, err := t.store.PatchSkill(ctx, req.Name, oldSkill.Content, req.Content)
	if err != nil {
		return errorJSON(err.Error()), nil
	}

	result, _ := json.Marshal(map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"name": patched.Meta.Name,
			"path": patched.Path,
		},
	})
	return string(result), nil
}

func (t *SkillManagerTool) handlePatch(ctx context.Context, req SkillManageRequest) (string, error) {
	if req.Name == "" {
		return errorJSON("name is required for patch"), nil
	}
	if req.OldString == "" {
		return errorJSON("old_string is required for patch"), nil
	}
	if req.NewString == "" {
		return errorJSON("new_string is required for patch"), nil
	}

	patched, err := t.store.PatchSkill(ctx, req.Name, req.OldString, req.NewString)
	if err != nil {
		return errorJSON(err.Error()), nil
	}

	result, _ := json.Marshal(map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"name": patched.Meta.Name,
			"path": patched.Path,
		},
	})
	return string(result), nil
}

func (t *SkillManagerTool) handleDelete(ctx context.Context, req SkillManageRequest) (string, error) {
	if req.Name == "" {
		return errorJSON("name is required for delete"), nil
	}

	if err := t.store.DeleteSkill(ctx, req.Name); err != nil {
		return errorJSON(err.Error()), nil
	}

	result, _ := json.Marshal(map[string]interface{}{
		"success": true,
		"data":    map[string]interface{}{"name": req.Name},
	})
	return string(result), nil
}

func (t *SkillManagerTool) handleWriteFile(ctx context.Context, req SkillManageRequest) (string, error) {
	if req.Name == "" {
		return errorJSON("name is required for write_file"), nil
	}
	if req.FilePath == "" {
		return errorJSON("file_path is required for write_file"), nil
	}

	// Validate filePath: no .. traversal
	if strings.Contains(req.FilePath, "..") {
		return errorJSON("file_path contains invalid characters: .."), nil
	}

	// Get skill to validate existence and locate directory
	skill, err := t.store.GetSkill(ctx, req.Name)
	if err != nil {
		return errorJSON(err.Error()), nil
	}

	fullPath := filepath.Join(skill.Path, req.FilePath)

	// Ensure file stays within skill directory
	absSkillPath, err := filepath.Abs(skill.Path)
	if err != nil {
		return errorJSON(fmt.Sprintf("获取绝对路径失败: %v", err)), nil
	}
	absFullPath, err := filepath.Abs(fullPath)
	if err != nil {
		return errorJSON(fmt.Sprintf("获取绝对路径失败: %v", err)), nil
	}
	if !strings.HasPrefix(absFullPath, absSkillPath+string(filepath.Separator)) && absFullPath != absSkillPath {
		return errorJSON("file_path extends beyond skill directory"), nil
	}

	// Ensure parent directory exists
	parentDir := filepath.Dir(fullPath)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return errorJSON(fmt.Sprintf("创建目录失败: %v", err)), nil
	}

	if err := os.WriteFile(fullPath, []byte(req.FileContent), 0644); err != nil {
		return errorJSON(fmt.Sprintf("写入文件失败: %v", err)), nil
	}

	result, _ := json.Marshal(map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"name":      req.Name,
			"file_path": req.FilePath,
		},
	})
	return string(result), nil
}

func (t *SkillManagerTool) handleRemoveFile(ctx context.Context, req SkillManageRequest) (string, error) {
	if req.Name == "" {
		return errorJSON("name is required for remove_file"), nil
	}
	if req.FilePath == "" {
		return errorJSON("file_path is required for remove_file"), nil
	}

	// Validate filePath: no .. traversal
	if strings.Contains(req.FilePath, "..") {
		return errorJSON("file_path contains invalid characters: .."), nil
	}

	// Get skill to validate existence and locate directory
	skill, err := t.store.GetSkill(ctx, req.Name)
	if err != nil {
		return errorJSON(err.Error()), nil
	}

	fullPath := filepath.Join(skill.Path, req.FilePath)

	// Ensure file stays within skill directory
	absSkillPath, err := filepath.Abs(skill.Path)
	if err != nil {
		return errorJSON(fmt.Sprintf("获取绝对路径失败: %v", err)), nil
	}
	absFullPath, err := filepath.Abs(fullPath)
	if err != nil {
		return errorJSON(fmt.Sprintf("获取绝对路径失败: %v", err)), nil
	}
	if !strings.HasPrefix(absFullPath, absSkillPath+string(filepath.Separator)) && absFullPath != absSkillPath {
		return errorJSON("file_path extends beyond skill directory"), nil
	}

	if err := os.Remove(fullPath); err != nil {
		return errorJSON(fmt.Sprintf("删除文件失败: %v", err)), nil
	}

	result, _ := json.Marshal(map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"name":      req.Name,
			"file_path": req.FilePath,
		},
	})
	return string(result), nil
}

func (t *SkillManagerTool) handlePin(ctx context.Context, req SkillManageRequest) (string, error) {
	if req.Name == "" {
		return errorJSON("name is required for pin"), nil
	}

	if err := t.store.PinSkill(ctx, req.Name); err != nil {
		return errorJSON(err.Error()), nil
	}

	result, _ := json.Marshal(map[string]interface{}{
		"success": true,
		"data":    map[string]interface{}{"name": req.Name},
	})
	return string(result), nil
}

func (t *SkillManagerTool) handleUnpin(ctx context.Context, req SkillManageRequest) (string, error) {
	if req.Name == "" {
		return errorJSON("name is required for unpin"), nil
	}

	if err := t.store.UnpinSkill(ctx, req.Name); err != nil {
		return errorJSON(err.Error()), nil
	}

	result, _ := json.Marshal(map[string]interface{}{
		"success": true,
		"data":    map[string]interface{}{"name": req.Name},
	})
	return string(result), nil
}

// ==================== helpers ====================

func errorJSON(msg string) string {
	result, _ := json.Marshal(map[string]interface{}{
		"success": false,
		"error":   msg,
	})
	return string(result)
}
