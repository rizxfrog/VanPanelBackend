// Package hub provides the MCP Hub service for plugin management
// and remote MCP configuration.
package hub

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/rizxfrog/VanPanelBackend/internal/agent/dao"
	"github.com/rizxfrog/VanPanelBackend/internal/agent/tool/mcp/manager"
	"github.com/rizxfrog/VanPanelBackend/internal/agent/tool/mcp/remote"
	"github.com/rizxfrog/VanPanelBackend/internal/model"
)

// AgentHubConfig Hub 配置，与 di 配置对齐，避免 import cycle。
type AgentHubConfig struct {
	PluginDir            string // 插件存放根目录
	MaxPluginSize        int    // 上传插件最大字节数
	MaxConcurrentPlugins int    // 最大并发插件数
}

// HubService MCP Hub 服务接口
type HubService interface {
	// 插件市场
	ListPlugins(ctx context.Context, req *model.ListMCPPluginsReq) (model.ListResp[*model.MCPPlugin], error)
	GetPlugin(ctx context.Context, id int) (*model.MCPPlugin, error)
	UploadPlugin(ctx context.Context, manifestJSON string, binary io.Reader, userID int) (*model.MCPPlugin, error)
	InstallPlugin(ctx context.Context, req *model.InstallMCPPluginReq, userID int) error
	UninstallPlugin(ctx context.Context, req *model.UninstallMCPPluginReq, userID int) error
	TogglePlugin(ctx context.Context, req *model.ToggleMCPPluginReq, userID int) error

	// 内置工具
	ListBuiltinTools(ctx context.Context) ([]*model.BuiltinTool, error)
	ToggleBuiltinTool(ctx context.Context, name string) error

	// 远程 MCP
	ListRemoteConfigs(ctx context.Context, req *model.ListRemoteMCPConfigsReq) (model.ListResp[*model.RemoteMCPConfig], error)
	CreateRemoteConfig(ctx context.Context, req *model.CreateRemoteMCPConfigReq, userID int) error
	UpdateRemoteConfig(ctx context.Context, req *model.UpdateRemoteMCPConfigReq) error
	DeleteRemoteConfig(ctx context.Context, id int) error
	ToggleRemoteConfig(ctx context.Context, id int) error
	TestRemoteConfig(ctx context.Context, id int) (*model.TestRemoteMCPResult, error)
}

type hubService struct {
	dao     dao.AgentDAO
	toolMgr *manager.ToolManager
	cfg     *AgentHubConfig
	logger  *zap.Logger
}

// NewHubService 创建 Hub 服务实例
func NewHubService(
	dao dao.AgentDAO,
	toolMgr *manager.ToolManager,
	cfg *AgentHubConfig,
	logger *zap.Logger,
) HubService {
	return &hubService{
		dao:     dao,
		toolMgr: toolMgr,
		cfg:     cfg,
		logger:  logger,
	}
}

// ==================== 插件市场 ====================

// ListPlugins 获取插件列表
func (s *hubService) ListPlugins(ctx context.Context, req *model.ListMCPPluginsReq) (model.ListResp[*model.MCPPlugin], error) {
	items, total, err := s.dao.ListPlugins(ctx, req)
	if err != nil {
		return model.ListResp[*model.MCPPlugin]{}, fmt.Errorf("查询插件列表失败: %w", err)
	}
	return model.ListResp[*model.MCPPlugin]{
		Items: items,
		Total: total,
	}, nil
}

// GetPlugin 获取插件详情
func (s *hubService) GetPlugin(ctx context.Context, id int) (*model.MCPPlugin, error) {
	plugin, err := s.dao.GetPlugin(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("获取插件详情失败: %w", err)
	}
	return plugin, nil
}

// UploadPlugin 上传插件：解析清单、写入二进制、计算哈希、入库
func (s *hubService) UploadPlugin(ctx context.Context, manifestJSON string, binary io.Reader, userID int) (*model.MCPPlugin, error) {
	// 解析 manifest
	var manifest map[string]interface{}
	if err := json.Unmarshal([]byte(manifestJSON), &manifest); err != nil {
		return nil, fmt.Errorf("解析插件清单失败: %w", err)
	}

	name, _ := manifest["name"].(string)
	if name == "" {
		return nil, errors.New("插件清单缺少 name 字段")
	}
	version, _ := manifest["version"].(string)
	if version == "" {
		version = "0.0.1"
	}
	displayName, _ := manifest["display_name"].(string)
	if displayName == "" {
		displayName = name
	}
	description, _ := manifest["description"].(string)
	author, _ := manifest["author"].(string)
	category, _ := manifest["category"].(string)
	homepage, _ := manifest["homepage"].(string)

	// 解析 tags
	var tags model.StringList
	if rawTags, ok := manifest["tags"].([]interface{}); ok {
		for _, t := range rawTags {
			if s, ok := t.(string); ok {
				tags = append(tags, s)
			}
		}
	}

	// 创建插件目录
	pluginDirName := fmt.Sprintf("%s-%s", name, version)
	pluginDir := filepath.Join(s.cfg.PluginDir, pluginDirName)
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		return nil, fmt.Errorf("创建插件目录失败: %w", err)
	}

	// 写入二进制文件
	binaryPath := filepath.Join(pluginDir, name)
	file, err := os.Create(binaryPath)
	if err != nil {
		return nil, fmt.Errorf("创建二进制文件失败: %w", err)
	}
	defer file.Close()

	hasher := sha256.New()
	writer := io.MultiWriter(file, hasher)

	// 如果设置了大小限制，使用 LimitReader
	reader := binary
	if s.cfg.MaxPluginSize > 0 {
		reader = io.LimitReader(binary, int64(s.cfg.MaxPluginSize))
	}

	written, err := io.Copy(writer, reader)
	if err != nil {
		_ = os.Remove(binaryPath)
		return nil, fmt.Errorf("写入二进制文件失败: %w", err)
	}
	if written == 0 {
		_ = os.Remove(binaryPath)
		return nil, errors.New("插件二进制文件为空")
	}

	hash := hex.EncodeToString(hasher.Sum(nil))

	// 设置可执行权限
	if err := os.Chmod(binaryPath, 0o755); err != nil {
		s.logger.Warn("设置可执行权限失败", zap.String("path", binaryPath), zap.Error(err))
	}

	// 写入清单文件
	manifestPath := filepath.Join(pluginDir, "manifest.json")
	manifestBytes, _ := json.MarshalIndent(manifest, "", "  ")
	if err := os.WriteFile(manifestPath, manifestBytes, 0o644); err != nil {
		s.logger.Warn("写入清单文件失败", zap.String("path", manifestPath), zap.Error(err))
	}

	// 构建插件记录
	plugin := &model.MCPPlugin{
		Name:        name,
		DisplayName: displayName,
		Description: description,
		Version:     version,
		Author:      author,
		Category:    category,
		Tags:        tags,
		Homepage:    homepage,
		Manifest:    model.JSONMap(manifest),
		BinaryPath:  binaryPath,
		BinaryHash:  hash,
		Status:      model.MCPPluginStatusActive,
	}

	if err := s.dao.CreatePlugin(ctx, plugin); err != nil {
		_ = os.Remove(binaryPath)
		return nil, fmt.Errorf("保存插件记录失败: %w", err)
	}

	s.logger.Info("插件上传成功",
		zap.String("name", name),
		zap.String("version", version),
		zap.Int64("size", written),
		zap.String("hash", hash))

	return plugin, nil
}

// InstallPlugin 安装插件
func (s *hubService) InstallPlugin(ctx context.Context, req *model.InstallMCPPluginReq, userID int) error {
	// 检查插件是否存在
	plugin, err := s.dao.GetPlugin(ctx, req.ID)
	if err != nil {
		return fmt.Errorf("获取插件失败: %w", err)
	}
	if plugin.Status != model.MCPPluginStatusActive {
		return fmt.Errorf("插件 [%s] 当前状态不可安装: %s", plugin.Name, plugin.Status)
	}

	// 检查是否已安装
	if _, err := s.dao.GetPluginInstall(ctx, req.ID, userID); err == nil {
		return fmt.Errorf("插件 [%s] 已安装", plugin.Name)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("查询安装记录失败: %w", err)
	}

	// 创建安装记录
	install := &model.MCPPluginInstall{
		PluginID: req.ID,
		UserID:   userID,
		Config:   req.Config,
		Enabled:  true,
	}
	if err := s.dao.CreatePluginInstall(ctx, install); err != nil {
		return fmt.Errorf("创建安装记录失败: %w", err)
	}

	// 增加下载计数
	if err := s.dao.IncrementPluginDownloads(ctx, req.ID); err != nil {
		s.logger.Warn("更新下载计数失败", zap.Int("plugin_id", req.ID), zap.Error(err))
	}

	// 通知工具管理器重载
	s.toolMgr.Reload(ctx)

	s.logger.Info("插件安装成功",
		zap.Int("plugin_id", req.ID),
		zap.Int("user_id", userID),
		zap.String("name", plugin.Name))

	return nil
}

// UninstallPlugin 卸载插件
func (s *hubService) UninstallPlugin(ctx context.Context, req *model.UninstallMCPPluginReq, userID int) error {
	if err := s.dao.DeletePluginInstall(ctx, req.ID, userID); err != nil {
		return fmt.Errorf("卸载插件失败: %w", err)
	}

	// 通知工具管理器重载
	s.toolMgr.Reload(ctx)

	s.logger.Info("插件卸载成功", zap.Int("plugin_id", req.ID), zap.Int("user_id", userID))
	return nil
}

// TogglePlugin 切换插件启用状态
func (s *hubService) TogglePlugin(ctx context.Context, req *model.ToggleMCPPluginReq, userID int) error {
	// 获取当前安装记录
	install, err := s.dao.GetPluginInstall(ctx, req.ID, userID)
	if err != nil {
		return fmt.Errorf("获取插件安装记录失败: %w", err)
	}

	// 切换状态
	if err := s.dao.TogglePluginInstall(ctx, req.ID, userID, !install.Enabled); err != nil {
		return fmt.Errorf("切换插件状态失败: %w", err)
	}

	// 通知工具管理器重载
	s.toolMgr.Reload(ctx)

	s.logger.Info("插件状态已切换",
		zap.Int("plugin_id", req.ID),
		zap.Bool("enabled", !install.Enabled))
	return nil
}

// ==================== 内置工具 ====================

// ListBuiltinTools 获取内置工具列表
func (s *hubService) ListBuiltinTools(ctx context.Context) ([]*model.BuiltinTool, error) {
	tools, err := s.dao.ListBuiltinTools(ctx)
	if err != nil {
		return nil, fmt.Errorf("查询内置工具列表失败: %w", err)
	}
	return tools, nil
}

// ToggleBuiltinTool 切换内置工具启用状态
func (s *hubService) ToggleBuiltinTool(ctx context.Context, name string) error {
	// 获取当前工具列表以找到目标工具
	tools, err := s.dao.ListBuiltinTools(ctx)
	if err != nil {
		return fmt.Errorf("查询内置工具失败: %w", err)
	}

	var current *model.BuiltinTool
	for _, t := range tools {
		if t.Name == name {
			current = t
			break
		}
	}
	if current == nil {
		return fmt.Errorf("内置工具 [%s] 不存在", name)
	}

	// 切换状态
	if err := s.dao.ToggleBuiltinTool(ctx, name, !current.Enabled); err != nil {
		return fmt.Errorf("切换内置工具状态失败: %w", err)
	}

	// 通知工具管理器重载
	s.toolMgr.Reload(ctx)

	s.logger.Info("内置工具状态已切换",
		zap.String("name", name),
		zap.Bool("enabled", !current.Enabled))
	return nil
}

// ==================== 远程 MCP 配置 ====================

// ListRemoteConfigs 获取远程 MCP 配置列表
func (s *hubService) ListRemoteConfigs(ctx context.Context, req *model.ListRemoteMCPConfigsReq) (model.ListResp[*model.RemoteMCPConfig], error) {
	items, total, err := s.dao.ListRemoteMCPConfigs(ctx, req)
	if err != nil {
		return model.ListResp[*model.RemoteMCPConfig]{}, fmt.Errorf("查询远程MCP配置列表失败: %w", err)
	}
	return model.ListResp[*model.RemoteMCPConfig]{
		Items: items,
		Total: total,
	}, nil
}

// CreateRemoteConfig 创建远程 MCP 配置
func (s *hubService) CreateRemoteConfig(ctx context.Context, req *model.CreateRemoteMCPConfigReq, userID int) error {
	config := &model.RemoteMCPConfig{
		UserID:      userID,
		Name:        req.Name,
		Description: req.Description,
		Transport:   req.Transport,
		URL:         req.URL,
		AuthType:    req.AuthType,
		AuthConfig:  req.AuthConfig,
		Enabled:     true,
	}
	// 默认认证方式
	if config.AuthType == "" {
		config.AuthType = model.RemoteMCPAuthTypeNone
	}

	if err := s.dao.CreateRemoteMCPConfig(ctx, config); err != nil {
		return fmt.Errorf("创建远程MCP配置失败: %w", err)
	}

	s.logger.Info("远程MCP配置创建成功",
		zap.Int("user_id", userID),
		zap.String("name", req.Name))
	return nil
}

// UpdateRemoteConfig 更新远程 MCP 配置
func (s *hubService) UpdateRemoteConfig(ctx context.Context, req *model.UpdateRemoteMCPConfigReq) error {
	config := &model.RemoteMCPConfig{
		Model:       model.Model{ID: req.ID},
		Name:        req.Name,
		Description: req.Description,
		Transport:   req.Transport,
		URL:         req.URL,
		AuthType:    req.AuthType,
		AuthConfig:  req.AuthConfig,
	}

	if err := s.dao.UpdateRemoteMCPConfig(ctx, config); err != nil {
		return fmt.Errorf("更新远程MCP配置失败: %w", err)
	}

	s.logger.Info("远程MCP配置更新成功", zap.Int("id", req.ID))
	return nil
}

// DeleteRemoteConfig 删除远程 MCP 配置
func (s *hubService) DeleteRemoteConfig(ctx context.Context, id int) error {
	if err := s.dao.DeleteRemoteMCPConfig(ctx, id); err != nil {
		return fmt.Errorf("删除远程MCP配置失败: %w", err)
	}

	s.logger.Info("远程MCP配置删除成功", zap.Int("id", id))
	return nil
}

// ToggleRemoteConfig 切换远程 MCP 配置启用状态
func (s *hubService) ToggleRemoteConfig(ctx context.Context, id int) error {
	// 获取当前配置
	config, err := s.dao.GetRemoteMCPConfig(ctx, id)
	if err != nil {
		return fmt.Errorf("获取远程MCP配置失败: %w", err)
	}

	if err := s.dao.ToggleRemoteMCPConfig(ctx, id, !config.Enabled); err != nil {
		return fmt.Errorf("切换远程MCP配置状态失败: %w", err)
	}

	// 通知工具管理器重载
	s.toolMgr.Reload(ctx)

	s.logger.Info("远程MCP配置状态已切换",
		zap.Int("id", id),
		zap.Bool("enabled", !config.Enabled))
	return nil
}

// TestRemoteConfig 测试远程 MCP 配置连通性
func (s *hubService) TestRemoteConfig(ctx context.Context, id int) (*model.TestRemoteMCPResult, error) {
	// 获取配置
	config, err := s.dao.GetRemoteMCPConfig(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("获取远程MCP配置失败: %w", err)
	}

	// 构建认证配置
	auth := remote.AuthConfig{Type: config.AuthType}
	if config.AuthConfig != nil {
		if v, ok := config.AuthConfig["token"].(string); ok {
			auth.Token = v
		}
		if v, ok := config.AuthConfig["username"].(string); ok {
			auth.Username = v
		}
		if v, ok := config.AuthConfig["password"].(string); ok {
			auth.Password = v
		}
	}

	// 创建临时客户端并尝试连接
	client := remote.NewClient(config.Name, config.URL, config.Transport, auth, s.logger)
	defer client.Close()

	result := &model.TestRemoteMCPResult{}

	if err := client.Connect(ctx); err != nil {
		result.Reachable = false
		result.Error = err.Error()
		return result, nil
	}

	// 连接成功，尝试获取工具列表
	result.Reachable = true
	mcpTools, err := client.ListTools(ctx)
	if err != nil {
		result.Error = fmt.Sprintf("连接成功但获取工具列表失败: %s", err.Error())
		return result, nil
	}

	// 提取工具名称
	result.Tools = make([]string, 0, len(mcpTools))
	for _, t := range mcpTools {
		result.Tools = append(result.Tools, t.Name)
	}

	return result, nil
}
