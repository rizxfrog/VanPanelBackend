package dao

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rizxfrog/VanPanelBackend/internal/agent/search"
	"github.com/rizxfrog/VanPanelBackend/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// UserMemory 用户长期记忆
type UserMemory struct {
	ID         uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID     uint      `gorm:"index;not null" json:"userId"`
	Username   string    `gorm:"size:64" json:"username"`
	SessionID  string    `gorm:"index;size:64" json:"sessionId"`
	Content    string    `gorm:"type:text;not null" json:"content"`
	MemoryType string    `gorm:"size:32;default:'general'" json:"memoryType"`
	Importance int       `gorm:"default:0" json:"importance"`
	CreatedAt  time.Time `json:"createdAt"`
}

// AgentDAO 智能体数据访问层接口
type AgentDAO interface {
	// 会话
	CreateSession(ctx context.Context, session *model.AgentSession) error
	GetSession(ctx context.Context, id int) (*model.AgentSession, error)
	ListSessions(ctx context.Context, req *model.ListAgentSessionsReq) ([]*model.AgentSession, int64, error)
	UpdateSession(ctx context.Context, session *model.AgentSession) error
	DeleteSession(ctx context.Context, id int) error
	IncrementSessionCounts(ctx context.Context, id int, msgDelta, toolDelta int) error

	// 消息
	CreateMessage(ctx context.Context, msg *model.AgentMessage) error
	ListMessages(ctx context.Context, req *model.ListAgentMessagesReq) ([]*model.AgentMessage, int64, error)

	// 内置工具
	ListBuiltinTools(ctx context.Context) ([]*model.BuiltinTool, error)
	ToggleBuiltinTool(ctx context.Context, name string, enabled bool) error
	CreateBuiltinTool(ctx context.Context, tool *model.BuiltinTool) error
	SeedBuiltinTools(ctx context.Context, tools []*model.BuiltinTool) error

	// 插件
	ListPlugins(ctx context.Context, req *model.ListMCPPluginsReq) ([]*model.MCPPlugin, int64, error)
	GetPlugin(ctx context.Context, id int) (*model.MCPPlugin, error)
	CreatePlugin(ctx context.Context, plugin *model.MCPPlugin) error
	UpdatePlugin(ctx context.Context, plugin *model.MCPPlugin) error
	DeletePlugin(ctx context.Context, id int) error
	IncrementPluginDownloads(ctx context.Context, id int) error

	// 插件安装
	CreatePluginInstall(ctx context.Context, install *model.MCPPluginInstall) error
	DeletePluginInstall(ctx context.Context, pluginID, userID int) error
	GetPluginInstall(ctx context.Context, pluginID, userID int) (*model.MCPPluginInstall, error)
	ListEnabledPluginInstalls(ctx context.Context) ([]*model.MCPPluginInstall, error)
	TogglePluginInstall(ctx context.Context, pluginID, userID int, enabled bool) error

	// 远程 MCP
	ListRemoteMCPConfigs(ctx context.Context, req *model.ListRemoteMCPConfigsReq) ([]*model.RemoteMCPConfig, int64, error)
	GetRemoteMCPConfig(ctx context.Context, id int) (*model.RemoteMCPConfig, error)
	CreateRemoteMCPConfig(ctx context.Context, config *model.RemoteMCPConfig) error
	UpdateRemoteMCPConfig(ctx context.Context, config *model.RemoteMCPConfig) error
	DeleteRemoteMCPConfig(ctx context.Context, id int) error
	ToggleRemoteMCPConfig(ctx context.Context, id int, enabled bool) error
	ListEnabledRemoteMCPConfigs(ctx context.Context) ([]*model.RemoteMCPConfig, error)

	// 审计
	CreateAuditEvent(ctx context.Context, event *model.AgentAuditEvent) error
	ListAuditEvents(ctx context.Context, sessionID string, limit int) ([]*model.AgentAuditEvent, error)

	// 长期记忆
	CreateUserMemory(ctx context.Context, memory *UserMemory) error
	ListUserMemoriesByUser(ctx context.Context, userID uint, limit int) ([]*UserMemory, error)

	// 全文搜索
	SearchMessages(ctx context.Context, query string, limit int) ([]*search.SearchResult, error)
	GetMessageContext(ctx context.Context, sessionID string, messageID int64, contextSize int) ([]*model.AgentMessage, error)
	ListRecentSessions(ctx context.Context, limit int) ([]*search.BrowseResult, error)
}

type agentDAO struct {
	db           *gorm.DB
	l            *zap.Logger
	searchEngine *search.SearchEngine
}

func NewAgentDAO(db *gorm.DB, l *zap.Logger, searchEngine *search.SearchEngine) AgentDAO {
	return &agentDAO{db: db, l: l, searchEngine: searchEngine}
}

// ==================== 会话 ====================

func (d *agentDAO) CreateSession(ctx context.Context, session *model.AgentSession) error {
	if session == nil {
		d.l.Error("CreateSession: 会话不能为空")
		return errors.New("会话不能为空")
	}
	if err := d.db.WithContext(ctx).Create(session).Error; err != nil {
		d.l.Error("CreateSession: 创建会话失败", zap.Error(err))
		return fmt.Errorf("创建会话失败: %w", err)
	}
	return nil
}

func (d *agentDAO) GetSession(ctx context.Context, id int) (*model.AgentSession, error) {
	if id <= 0 {
		d.l.Error("GetSession: 无效的ID", zap.Int("id", id))
		return nil, errors.New("无效的ID")
	}
	var session model.AgentSession
	if err := d.db.WithContext(ctx).Where("id = ?", id).First(&session).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			d.l.Debug("GetSession: 会话不存在", zap.Int("id", id))
			return nil, gorm.ErrRecordNotFound
		}
		d.l.Error("GetSession: 查询会话失败", zap.Int("id", id), zap.Error(err))
		return nil, fmt.Errorf("查询会话失败: %w", err)
	}
	return &session, nil
}

func (d *agentDAO) ListSessions(ctx context.Context, req *model.ListAgentSessionsReq) ([]*model.AgentSession, int64, error) {
	if req == nil {
		d.l.Error("ListSessions: 请求不能为空")
		return nil, 0, errors.New("请求不能为空")
	}
	var sessions []*model.AgentSession
	var total int64

	query := d.db.WithContext(ctx).Model(&model.AgentSession{})

	if req.Search != "" {
		query = query.Where("title LIKE ?", "%"+req.Search+"%")
	}

	if err := query.Count(&total).Error; err != nil {
		d.l.Error("ListSessions: 统计总数失败", zap.Error(err))
		return nil, 0, fmt.Errorf("统计总数失败: %w", err)
	}
	if total == 0 {
		return []*model.AgentSession{}, 0, nil
	}

	offset := (req.Page - 1) * req.Size
	if err := query.Order("created_at DESC, id DESC").Offset(offset).Limit(req.Size).Find(&sessions).Error; err != nil {
		d.l.Error("ListSessions: 查询列表失败", zap.Error(err))
		return nil, 0, fmt.Errorf("查询列表失败: %w", err)
	}
	return sessions, total, nil
}

func (d *agentDAO) UpdateSession(ctx context.Context, session *model.AgentSession) error {
	if session == nil || session.ID == 0 {
		d.l.Error("UpdateSession: 无效的会话")
		return errors.New("无效的会话")
	}
	result := d.db.WithContext(ctx).Model(session).Where("id = ?", session.ID).Updates(session)
	if result.Error != nil {
		d.l.Error("UpdateSession: 更新会话失败", zap.Int("id", session.ID), zap.Error(result.Error))
		return fmt.Errorf("更新会话失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		d.l.Debug("UpdateSession: 会话不存在", zap.Int("id", session.ID))
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (d *agentDAO) DeleteSession(ctx context.Context, id int) error {
	if id <= 0 {
		d.l.Error("DeleteSession: 无效的ID", zap.Int("id", id))
		return errors.New("无效的ID")
	}
	result := d.db.WithContext(ctx).Where("id = ?", id).Delete(&model.AgentSession{})
	if result.Error != nil {
		d.l.Error("DeleteSession: 删除会话失败", zap.Int("id", id), zap.Error(result.Error))
		return fmt.Errorf("删除会话失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		d.l.Debug("DeleteSession: 会话不存在", zap.Int("id", id))
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (d *agentDAO) IncrementSessionCounts(ctx context.Context, id int, msgDelta, toolDelta int) error {
	return d.db.WithContext(ctx).Model(&model.AgentSession{}).Where("id = ?", id).
		UpdateColumns(map[string]interface{}{
			"message_count": gorm.Expr("message_count + ?", msgDelta),
			"tool_count":    gorm.Expr("tool_count + ?", toolDelta),
		}).Error
}

// ==================== 消息 ====================

func (d *agentDAO) CreateMessage(ctx context.Context, msg *model.AgentMessage) error {
	if msg == nil {
		d.l.Error("CreateMessage: 消息不能为空")
		return errors.New("消息不能为空")
	}
	if err := d.db.WithContext(ctx).Create(msg).Error; err != nil {
		d.l.Error("CreateMessage: 创建消息失败", zap.Error(err))
		return fmt.Errorf("创建消息失败: %w", err)
	}
	return nil
}

func (d *agentDAO) ListMessages(ctx context.Context, req *model.ListAgentMessagesReq) ([]*model.AgentMessage, int64, error) {
	if req == nil {
		d.l.Error("ListMessages: 请求不能为空")
		return nil, 0, errors.New("请求不能为空")
	}
	if req.SessionID == "" {
		d.l.Error("ListMessages: 会话ID不能为空")
		return nil, 0, errors.New("会话ID不能为空")
	}

	var messages []*model.AgentMessage
	var total int64

	query := d.db.WithContext(ctx).Model(&model.AgentMessage{}).Where("session_id = ?", req.SessionID)

	if req.Search != "" {
		query = query.Where("content LIKE ?", "%"+req.Search+"%")
	}

	if err := query.Count(&total).Error; err != nil {
		d.l.Error("ListMessages: 统计总数失败", zap.String("session_id", req.SessionID), zap.Error(err))
		return nil, 0, fmt.Errorf("统计总数失败: %w", err)
	}
	if total == 0 {
		return []*model.AgentMessage{}, 0, nil
	}

	offset := (req.Page - 1) * req.Size
	if err := query.Order("created_at ASC, id ASC").Offset(offset).Limit(req.Size).Find(&messages).Error; err != nil {
		d.l.Error("ListMessages: 查询列表失败", zap.String("session_id", req.SessionID), zap.Error(err))
		return nil, 0, fmt.Errorf("查询列表失败: %w", err)
	}
	return messages, total, nil
}

// ==================== 内置工具 ====================

func (d *agentDAO) ListBuiltinTools(ctx context.Context) ([]*model.BuiltinTool, error) {
	var tools []*model.BuiltinTool
	if err := d.db.WithContext(ctx).Order("category ASC, name ASC").Find(&tools).Error; err != nil {
		d.l.Error("ListBuiltinTools: 查询内置工具列表失败", zap.Error(err))
		return nil, fmt.Errorf("查询内置工具列表失败: %w", err)
	}
	return tools, nil
}

func (d *agentDAO) ToggleBuiltinTool(ctx context.Context, name string, enabled bool) error {
	if name == "" {
		d.l.Error("ToggleBuiltinTool: 工具名称不能为空")
		return errors.New("工具名称不能为空")
	}
	result := d.db.WithContext(ctx).Model(&model.BuiltinTool{}).Where("name = ?", name).Update("enabled", enabled)
	if result.Error != nil {
		d.l.Error("ToggleBuiltinTool: 更新失败", zap.String("name", name), zap.Error(result.Error))
		return fmt.Errorf("更新工具状态失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (d *agentDAO) CreateBuiltinTool(ctx context.Context, tool *model.BuiltinTool) error {
	if tool == nil {
		d.l.Error("CreateBuiltinTool: 工具不能为空")
		return errors.New("工具不能为空")
	}
	if tool.Name == "" {
		d.l.Error("CreateBuiltinTool: 工具名称不能为空")
		return errors.New("工具名称不能为空")
	}
	if err := d.db.WithContext(ctx).Create(tool).Error; err != nil {
		d.l.Error("CreateBuiltinTool: 创建工具失败", zap.String("name", tool.Name), zap.Error(err))
		return fmt.Errorf("创建工具失败: %w", err)
	}
	return nil
}

func (d *agentDAO) SeedBuiltinTools(ctx context.Context, tools []*model.BuiltinTool) error {
	if len(tools) == 0 {
		return nil
	}
	for _, tool := range tools {
		if err := d.db.WithContext(ctx).
			Where("name = ?", tool.Name).
			FirstOrCreate(tool).Error; err != nil {
			d.l.Error("SeedBuiltinTools: seed 失败", zap.String("name", tool.Name), zap.Error(err))
			return fmt.Errorf("seed 内置工具 %s 失败: %w", tool.Name, err)
		}
	}
	d.l.Info("SeedBuiltinTools: 内置工具 seed 完成", zap.Int("count", len(tools)))
	return nil
}

// ==================== 插件 ====================

func (d *agentDAO) ListPlugins(ctx context.Context, req *model.ListMCPPluginsReq) ([]*model.MCPPlugin, int64, error) {
	if req == nil {
		d.l.Error("ListPlugins: 请求不能为空")
		return nil, 0, errors.New("请求不能为空")
	}
	var plugins []*model.MCPPlugin
	var total int64

	query := d.db.WithContext(ctx).Model(&model.MCPPlugin{})

	if req.Search != "" {
		query = query.Where("name LIKE ? OR display_name LIKE ?", "%"+req.Search+"%", "%"+req.Search+"%")
	}
	if req.Category != "" {
		query = query.Where("category = ?", req.Category)
	}

	if err := query.Count(&total).Error; err != nil {
		d.l.Error("ListPlugins: 统计总数失败", zap.Error(err))
		return nil, 0, fmt.Errorf("统计总数失败: %w", err)
	}
	if total == 0 {
		return []*model.MCPPlugin{}, 0, nil
	}

	offset := (req.Page - 1) * req.Size
	if err := query.Order("created_at DESC, id DESC").Offset(offset).Limit(req.Size).Find(&plugins).Error; err != nil {
		d.l.Error("ListPlugins: 查询列表失败", zap.Error(err))
		return nil, 0, fmt.Errorf("查询列表失败: %w", err)
	}
	return plugins, total, nil
}

func (d *agentDAO) GetPlugin(ctx context.Context, id int) (*model.MCPPlugin, error) {
	if id <= 0 {
		d.l.Error("GetPlugin: 无效的ID", zap.Int("id", id))
		return nil, errors.New("无效的ID")
	}
	var plugin model.MCPPlugin
	if err := d.db.WithContext(ctx).Where("id = ?", id).First(&plugin).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			d.l.Debug("GetPlugin: 插件不存在", zap.Int("id", id))
			return nil, gorm.ErrRecordNotFound
		}
		d.l.Error("GetPlugin: 查询插件失败", zap.Int("id", id), zap.Error(err))
		return nil, fmt.Errorf("查询插件失败: %w", err)
	}
	return &plugin, nil
}

func (d *agentDAO) CreatePlugin(ctx context.Context, plugin *model.MCPPlugin) error {
	if plugin == nil {
		d.l.Error("CreatePlugin: 插件不能为空")
		return errors.New("插件不能为空")
	}
	if err := d.db.WithContext(ctx).Create(plugin).Error; err != nil {
		d.l.Error("CreatePlugin: 创建插件失败", zap.String("name", plugin.Name), zap.Error(err))
		return fmt.Errorf("创建插件失败: %w", err)
	}
	return nil
}

func (d *agentDAO) UpdatePlugin(ctx context.Context, plugin *model.MCPPlugin) error {
	if plugin == nil || plugin.ID == 0 {
		d.l.Error("UpdatePlugin: 无效的插件")
		return errors.New("无效的插件")
	}
	result := d.db.WithContext(ctx).Model(plugin).Where("id = ?", plugin.ID).Updates(plugin)
	if result.Error != nil {
		d.l.Error("UpdatePlugin: 更新插件失败", zap.Int("id", plugin.ID), zap.Error(result.Error))
		return fmt.Errorf("更新插件失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		d.l.Debug("UpdatePlugin: 插件不存在", zap.Int("id", plugin.ID))
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (d *agentDAO) DeletePlugin(ctx context.Context, id int) error {
	if id <= 0 {
		d.l.Error("DeletePlugin: 无效的ID", zap.Int("id", id))
		return errors.New("无效的ID")
	}
	result := d.db.WithContext(ctx).Where("id = ?", id).Delete(&model.MCPPlugin{})
	if result.Error != nil {
		d.l.Error("DeletePlugin: 删除插件失败", zap.Int("id", id), zap.Error(result.Error))
		return fmt.Errorf("删除插件失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		d.l.Debug("DeletePlugin: 插件不存在", zap.Int("id", id))
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (d *agentDAO) IncrementPluginDownloads(ctx context.Context, id int) error {
	return d.db.WithContext(ctx).Model(&model.MCPPlugin{}).Where("id = ?", id).
		UpdateColumn("downloads", gorm.Expr("downloads + 1")).Error
}

// ==================== 插件安装 ====================

func (d *agentDAO) CreatePluginInstall(ctx context.Context, install *model.MCPPluginInstall) error {
	if install == nil {
		d.l.Error("CreatePluginInstall: 安装记录不能为空")
		return errors.New("安装记录不能为空")
	}
	if err := d.db.WithContext(ctx).Create(install).Error; err != nil {
		d.l.Error("CreatePluginInstall: 创建安装记录失败", zap.Int("plugin_id", install.PluginID), zap.Int("user_id", install.UserID), zap.Error(err))
		return fmt.Errorf("创建安装记录失败: %w", err)
	}
	return nil
}

func (d *agentDAO) DeletePluginInstall(ctx context.Context, pluginID, userID int) error {
	if pluginID <= 0 || userID <= 0 {
		d.l.Error("DeletePluginInstall: 无效的参数", zap.Int("plugin_id", pluginID), zap.Int("user_id", userID))
		return errors.New("无效的参数")
	}
	result := d.db.WithContext(ctx).Where("plugin_id = ? AND user_id = ?", pluginID, userID).Delete(&model.MCPPluginInstall{})
	if result.Error != nil {
		d.l.Error("DeletePluginInstall: 删除安装记录失败", zap.Int("plugin_id", pluginID), zap.Int("user_id", userID), zap.Error(result.Error))
		return fmt.Errorf("删除安装记录失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		d.l.Debug("DeletePluginInstall: 安装记录不存在", zap.Int("plugin_id", pluginID), zap.Int("user_id", userID))
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (d *agentDAO) GetPluginInstall(ctx context.Context, pluginID, userID int) (*model.MCPPluginInstall, error) {
	if pluginID <= 0 || userID <= 0 {
		d.l.Error("GetPluginInstall: 无效的参数", zap.Int("plugin_id", pluginID), zap.Int("user_id", userID))
		return nil, errors.New("无效的参数")
	}
	var install model.MCPPluginInstall
	if err := d.db.WithContext(ctx).Where("plugin_id = ? AND user_id = ?", pluginID, userID).First(&install).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			d.l.Debug("GetPluginInstall: 安装记录不存在", zap.Int("plugin_id", pluginID), zap.Int("user_id", userID))
			return nil, gorm.ErrRecordNotFound
		}
		d.l.Error("GetPluginInstall: 查询安装记录失败", zap.Int("plugin_id", pluginID), zap.Int("user_id", userID), zap.Error(err))
		return nil, fmt.Errorf("查询安装记录失败: %w", err)
	}
	return &install, nil
}

func (d *agentDAO) ListEnabledPluginInstalls(ctx context.Context) ([]*model.MCPPluginInstall, error) {
	var installs []*model.MCPPluginInstall
	if err := d.db.WithContext(ctx).Where("enabled = ?", true).Find(&installs).Error; err != nil {
		d.l.Error("ListEnabledPluginInstalls: 查询已启用安装记录失败", zap.Error(err))
		return nil, fmt.Errorf("查询已启用安装记录失败: %w", err)
	}
	return installs, nil
}

func (d *agentDAO) TogglePluginInstall(ctx context.Context, pluginID, userID int, enabled bool) error {
	if pluginID <= 0 || userID <= 0 {
		d.l.Error("TogglePluginInstall: 无效的参数", zap.Int("plugin_id", pluginID), zap.Int("user_id", userID))
		return errors.New("无效的参数")
	}
	result := d.db.WithContext(ctx).Model(&model.MCPPluginInstall{}).
		Where("plugin_id = ? AND user_id = ?", pluginID, userID).
		Update("enabled", enabled)
	if result.Error != nil {
		d.l.Error("TogglePluginInstall: 更新失败", zap.Int("plugin_id", pluginID), zap.Int("user_id", userID), zap.Error(result.Error))
		return fmt.Errorf("更新插件安装状态失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// ==================== 远程 MCP ====================

func (d *agentDAO) ListRemoteMCPConfigs(ctx context.Context, req *model.ListRemoteMCPConfigsReq) ([]*model.RemoteMCPConfig, int64, error) {
	if req == nil {
		d.l.Error("ListRemoteMCPConfigs: 请求不能为空")
		return nil, 0, errors.New("请求不能为空")
	}
	var configs []*model.RemoteMCPConfig
	var total int64

	query := d.db.WithContext(ctx).Model(&model.RemoteMCPConfig{})

	if req.Search != "" {
		query = query.Where("name LIKE ?", "%"+req.Search+"%")
	}

	if err := query.Count(&total).Error; err != nil {
		d.l.Error("ListRemoteMCPConfigs: 统计总数失败", zap.Error(err))
		return nil, 0, fmt.Errorf("统计总数失败: %w", err)
	}
	if total == 0 {
		return []*model.RemoteMCPConfig{}, 0, nil
	}

	offset := (req.Page - 1) * req.Size
	if err := query.Order("created_at DESC, id DESC").Offset(offset).Limit(req.Size).Find(&configs).Error; err != nil {
		d.l.Error("ListRemoteMCPConfigs: 查询列表失败", zap.Error(err))
		return nil, 0, fmt.Errorf("查询列表失败: %w", err)
	}
	return configs, total, nil
}

func (d *agentDAO) GetRemoteMCPConfig(ctx context.Context, id int) (*model.RemoteMCPConfig, error) {
	if id <= 0 {
		d.l.Error("GetRemoteMCPConfig: 无效的ID", zap.Int("id", id))
		return nil, errors.New("无效的ID")
	}
	var config model.RemoteMCPConfig
	if err := d.db.WithContext(ctx).Where("id = ?", id).First(&config).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			d.l.Debug("GetRemoteMCPConfig: 配置不存在", zap.Int("id", id))
			return nil, gorm.ErrRecordNotFound
		}
		d.l.Error("GetRemoteMCPConfig: 查询配置失败", zap.Int("id", id), zap.Error(err))
		return nil, fmt.Errorf("查询配置失败: %w", err)
	}
	return &config, nil
}

func (d *agentDAO) CreateRemoteMCPConfig(ctx context.Context, config *model.RemoteMCPConfig) error {
	if config == nil {
		d.l.Error("CreateRemoteMCPConfig: 配置不能为空")
		return errors.New("配置不能为空")
	}
	if err := d.db.WithContext(ctx).Create(config).Error; err != nil {
		d.l.Error("CreateRemoteMCPConfig: 创建配置失败", zap.String("name", config.Name), zap.Error(err))
		return fmt.Errorf("创建配置失败: %w", err)
	}
	return nil
}

func (d *agentDAO) UpdateRemoteMCPConfig(ctx context.Context, config *model.RemoteMCPConfig) error {
	if config == nil || config.ID == 0 {
		d.l.Error("UpdateRemoteMCPConfig: 无效的配置")
		return errors.New("无效的配置")
	}
	result := d.db.WithContext(ctx).Model(config).Where("id = ?", config.ID).Updates(config)
	if result.Error != nil {
		d.l.Error("UpdateRemoteMCPConfig: 更新配置失败", zap.Int("id", config.ID), zap.Error(result.Error))
		return fmt.Errorf("更新配置失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		d.l.Debug("UpdateRemoteMCPConfig: 配置不存在", zap.Int("id", config.ID))
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (d *agentDAO) DeleteRemoteMCPConfig(ctx context.Context, id int) error {
	if id <= 0 {
		d.l.Error("DeleteRemoteMCPConfig: 无效的ID", zap.Int("id", id))
		return errors.New("无效的ID")
	}
	result := d.db.WithContext(ctx).Where("id = ?", id).Delete(&model.RemoteMCPConfig{})
	if result.Error != nil {
		d.l.Error("DeleteRemoteMCPConfig: 删除配置失败", zap.Int("id", id), zap.Error(result.Error))
		return fmt.Errorf("删除配置失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		d.l.Debug("DeleteRemoteMCPConfig: 配置不存在", zap.Int("id", id))
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (d *agentDAO) ToggleRemoteMCPConfig(ctx context.Context, id int, enabled bool) error {
	if id <= 0 {
		d.l.Error("ToggleRemoteMCPConfig: 无效的ID", zap.Int("id", id))
		return errors.New("无效的ID")
	}
	result := d.db.WithContext(ctx).Model(&model.RemoteMCPConfig{}).Where("id = ?", id).Update("enabled", enabled)
	if result.Error != nil {
		d.l.Error("ToggleRemoteMCPConfig: 更新失败", zap.Int("id", id), zap.Error(result.Error))
		return fmt.Errorf("更新远程MCP配置状态失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (d *agentDAO) ListEnabledRemoteMCPConfigs(ctx context.Context) ([]*model.RemoteMCPConfig, error) {
	var configs []*model.RemoteMCPConfig
	if err := d.db.WithContext(ctx).Where("enabled = ?", true).Find(&configs).Error; err != nil {
		d.l.Error("ListEnabledRemoteMCPConfigs: 查询已启用配置失败", zap.Error(err))
		return nil, fmt.Errorf("查询已启用配置失败: %w", err)
	}
	return configs, nil
}

// ==================== 审计 ====================

func (d *agentDAO) CreateAuditEvent(ctx context.Context, event *model.AgentAuditEvent) error {
	if event == nil {
		d.l.Error("CreateAuditEvent: 审计事件不能为空")
		return errors.New("审计事件不能为空")
	}
	if err := d.db.WithContext(ctx).Create(event).Error; err != nil {
		d.l.Error("CreateAuditEvent: 创建审计事件失败", zap.Error(err))
		return fmt.Errorf("创建审计事件失败: %w", err)
	}
	return nil
}

func (d *agentDAO) ListAuditEvents(ctx context.Context, sessionID string, limit int) ([]*model.AgentAuditEvent, error) {
	if sessionID == "" {
		d.l.Error("ListAuditEvents: 会话ID不能为空")
		return nil, errors.New("会话ID不能为空")
	}
	if limit <= 0 {
		limit = 50
	}
	var events []*model.AgentAuditEvent
	if err := d.db.WithContext(ctx).Where("session_id = ?", sessionID).
		Order("created_at DESC, id DESC").Limit(limit).Find(&events).Error; err != nil {
		d.l.Error("ListAuditEvents: 查询审计事件失败", zap.String("session_id", sessionID), zap.Error(err))
		return nil, fmt.Errorf("查询审计事件失败: %w", err)
	}
	return events, nil
}

// ==================== 长期记忆 ====================

func (d *agentDAO) CreateUserMemory(ctx context.Context, memory *UserMemory) error {
	if memory == nil {
		d.l.Error("CreateUserMemory: 记忆不能为空")
		return errors.New("记忆不能为空")
	}
	if err := d.db.WithContext(ctx).Create(memory).Error; err != nil {
		d.l.Error("CreateUserMemory: 创建记忆失败", zap.Error(err))
		return fmt.Errorf("创建记忆失败: %w", err)
	}
	return nil
}

func (d *agentDAO) ListUserMemoriesByUser(ctx context.Context, userID uint, limit int) ([]*UserMemory, error) {
	if limit <= 0 {
		limit = 20
	}
	var memories []*UserMemory
	if err := d.db.WithContext(ctx).
		Order("created_at DESC, id DESC").
		Limit(limit).
		Find(&memories).Error; err != nil {
		d.l.Error("ListUserMemoriesByUser: 查询记忆列表失败", zap.Error(err))
		return nil, fmt.Errorf("查询记忆列表失败: %w", err)
	}
	return memories, nil
}

// ==================== 全文搜索 ====================

// SearchMessages 全文检索消息，委托给搜索引擎
func (d *agentDAO) SearchMessages(ctx context.Context, query string, limit int) ([]*search.SearchResult, error) {
	if d.searchEngine == nil {
		d.l.Error("SearchMessages: 搜索引擎未初始化")
		return nil, fmt.Errorf("搜索引擎未初始化")
	}
	if query == "" {
		d.l.Error("SearchMessages: 搜索关键词不能为空")
		return nil, fmt.Errorf("搜索关键词不能为空")
	}
	return d.searchEngine.Search(ctx, query, limit)
}

// GetMessageContext 获取指定消息的上下文（前后各 contextSize 条消息）
func (d *agentDAO) GetMessageContext(ctx context.Context, sessionID string, messageID int64, contextSize int) ([]*model.AgentMessage, error) {
	if sessionID == "" {
		d.l.Error("GetMessageContext: 会话ID不能为空")
		return nil, errors.New("会话ID不能为空")
	}
	if messageID <= 0 {
		d.l.Error("GetMessageContext: 无效的消息ID", zap.Int64("message_id", messageID))
		return nil, errors.New("无效的消息ID")
	}
	if contextSize <= 0 {
		contextSize = 5
	}

	var messages []*model.AgentMessage

	// 查询指定消息前后的上下文，合并为一条查询
	if err := d.db.WithContext(ctx).Raw(`
		SELECT id, session_id, role, content, tool_calls, tool_call_id, metadata, created_at FROM cl_agent_messages
		WHERE session_id = ? AND id >= ? - ? AND id <= ? + ? AND id != ?
		ORDER BY id ASC
	`, sessionID, messageID, contextSize, messageID, contextSize, messageID).
		Scan(&messages).Error; err != nil {
		d.l.Error("GetMessageContext: 查询消息上下文失败",
			zap.String("session_id", sessionID),
			zap.Int64("message_id", messageID),
			zap.Error(err))
		return nil, fmt.Errorf("查询消息上下文失败: %w", err)
	}
	return messages, nil
}

// ListRecentSessions 列出最近活跃的会话，委托给搜索引擎
func (d *agentDAO) ListRecentSessions(ctx context.Context, limit int) ([]*search.BrowseResult, error) {
	if d.searchEngine == nil {
		d.l.Error("ListRecentSessions: 搜索引擎未初始化")
		return nil, fmt.Errorf("搜索引擎未初始化")
	}
	return d.searchEngine.Browse(ctx, limit)
}
