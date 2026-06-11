package search

import (
	"context"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// SearchResult 全文搜索结果
type SearchResult struct {
	SessionID    string    `json:"session_id"`
	MessageID    int64     `json:"message_id"`
	Role         string    `json:"role"`
	Headline     string    `json:"headline"`     // ts_headline 生成的高亮片段
	Content      string    `json:"content"`      // 消息内容前500字符
	Rank         float64   `json:"rank"`         // ts_rank 评分
	CreatedAt    time.Time `json:"created_at"`
	SessionTitle string    `json:"session_title,omitempty"`
}

// BrowseResult 会话浏览结果
type BrowseResult struct {
	SessionID    string    `json:"session_id"`
	Title        string    `json:"title"`
	Preview      string    `json:"preview"`       // 首条用户消息前100字符
	MessageCount int       `json:"message_count"`
	LastActiveAt time.Time `json:"last_active_at"`
}

// SearchEngine PostgreSQL 全文搜索引擎
type SearchEngine struct {
	db     *gorm.DB
	logger *zap.Logger
}

// NewSearchEngine 创建全文搜索引擎实例
func NewSearchEngine(db *gorm.DB, logger *zap.Logger) *SearchEngine {
	return &SearchEngine{db: db, logger: logger}
}

// Search 使用 plainto_tsquery 进行全文搜索，返回 ts_rank 排序 + ts_headline 高亮的结果
func (e *SearchEngine) Search(ctx context.Context, query string, limit int) ([]*SearchResult, error) {
	if limit <= 0 {
		limit = 20
	}
	var results []*SearchResult
	err := e.db.WithContext(ctx).Raw(`
		SELECT m.session_id, m.id as message_id, m.role,
			ts_headline('simple', m.content, plainto_tsquery('simple', ?),
				'MaxWords=40 MinWords=15 ShortWord=3 MaxFragments=3 StartSel=<b> StopSel=</b>') as headline,
			substring(m.content, 1, 500) as content,
			ts_rank(m.search_vector, plainto_tsquery('simple', ?)) as rank,
			m.created_at,
			s.title as session_title
		FROM cl_agent_messages m
		LEFT JOIN cl_agent_sessions s ON s.id::text = m.session_id
		WHERE m.search_vector @@ plainto_tsquery('simple', ?)
		ORDER BY rank DESC LIMIT ?
	`, query, query, query, limit).Scan(&results).Error
	if err != nil {
		e.logger.Error("全文搜索失败", zap.String("query", query), zap.Error(err))
		return nil, err
	}
	return results, nil
}

// Scroll 按 message_id 游标浏览会话内的消息
func (e *SearchEngine) Scroll(ctx context.Context, sessionID string, cursor int64, limit int) ([]*SearchResult, error) {
	if limit <= 0 {
		limit = 20
	}
	var results []*SearchResult
	err := e.db.WithContext(ctx).Raw(`
		SELECT m.session_id, m.id as message_id, m.role,
			substring(m.content, 1, 500) as content, m.created_at
		FROM cl_agent_messages m
		WHERE m.session_id = ? AND m.id > ? ORDER BY m.id ASC LIMIT ?
	`, sessionID, cursor, limit).Scan(&results).Error
	if err != nil {
		e.logger.Error("消息滚动查询失败", zap.String("session_id", sessionID), zap.Int64("cursor", cursor), zap.Error(err))
		return nil, err
	}
	return results, nil
}

// Browse 列出最近活跃的会话
func (e *SearchEngine) Browse(ctx context.Context, limit int) ([]*BrowseResult, error) {
	if limit <= 0 {
		limit = 20
	}
	var results []*BrowseResult
	err := e.db.WithContext(ctx).Raw(`
		SELECT m.session_id, s.title,
			MIN(substring(m.content, 1, 100)) as preview,
			s.message_count, MAX(m.created_at) as last_active_at
		FROM cl_agent_messages m
		LEFT JOIN cl_agent_sessions s ON s.id::text = m.session_id
		WHERE m.role = 'user'
		GROUP BY m.session_id, s.title, s.message_count
		ORDER BY last_active_at DESC LIMIT ?
	`, limit).Scan(&results).Error
	if err != nil {
		e.logger.Error("浏览会话列表失败", zap.Error(err))
		return nil, err
	}
	return results, nil
}
