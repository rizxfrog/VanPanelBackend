package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/GoSimplicity/AI-CloudOps/internal/files/dao"
	"github.com/GoSimplicity/AI-CloudOps/internal/model"
	"go.uber.org/zap"
)

type FileShareService interface {
	CreateShare(ctx context.Context, creatorID int, req *model.CreateShareReq) (*model.CreateShareResp, error)
	MergeShares(ctx context.Context, req *model.MergeSharesReq) (*model.CreateShareResp, error)
	VerifyAccess(ctx context.Context, shareCode string, req *model.VerifyAccessReq) (*model.VerifyAccessResp, error)
	UpdateShare(ctx context.Context, shareID uint, req *model.UpdateShareReq) error
	GetShareFiles(ctx context.Context, shareCode string) ([]model.FileShareItem, error)
	ListShares(ctx context.Context, req *model.ShareListReq) ([]model.FileShare, int64, error)
	GetShareByID(ctx context.Context, shareID uint) (*model.ShareDetailResp, error)
	GetShareByCode(ctx context.Context, shareCode string) (*model.ShareInfoResp, error)
	DeleteShare(ctx context.Context, shareID uint, userID int, isAdmin bool) error
	DownloadFile(ctx context.Context, shareCode, accessCode, filePath string) (string, error)
}

type fileShareService struct {
	l      *zap.Logger
	dao    dao.FileShareDAO
	config ManagerConfig
}

func NewFileShareService(l *zap.Logger, dao dao.FileShareDAO, config ManagerConfig) FileShareService {
	return &fileShareService{
		l:      l,
		dao:    dao,
		config: config,
	}
}

// CreateShare 创建分享链接
func (s *fileShareService) CreateShare(ctx context.Context, creatorID int, req *model.CreateShareReq) (*model.CreateShareResp, error) {
	// 验证文件路径在允许的根目录下
	for _, item := range req.Items {
		if err := s.validateFilePath(item.FilePath); err != nil {
			return nil, fmt.Errorf("invalid file path %s: %w", item.FilePath, err)
		}
	}

	resp, err := s.dao.CreateShare(ctx, creatorID, req.AccessLevel, req.MaxDownloads, req.ExpireAt, req.Items)
	if err != nil {
		s.l.Error("创建分享失败", zap.Error(err))
		return nil, fmt.Errorf("创建分享失败: %w", err)
	}

	s.l.Info("创建分享成功",
		zap.Int("share_id", resp.ShareID),
		zap.String("share_code", resp.ShareCode),
		zap.Int("creator_id", creatorID))

	return resp, nil
}

// MergeShares 合并分享链接
func (s *fileShareService) MergeShares(ctx context.Context, req *model.MergeSharesReq) (*model.CreateShareResp, error) {
	if len(req.ShareIDs) < 2 {
		return nil, fmt.Errorf("至少需要2个分享链接才能合并")
	}

	resp, err := s.dao.MergeShares(ctx, req.ShareIDs)
	if err != nil {
		s.l.Error("合并分享失败", zap.Error(err))
		return nil, fmt.Errorf("合并分享失败: %w", err)
	}

	s.l.Info("合并分享成功",
		zap.Int("new_share_id", resp.ShareID),
		zap.Uints("merged_ids", req.ShareIDs))

	return resp, nil
}

// VerifyAccess 验证访问权限
func (s *fileShareService) VerifyAccess(ctx context.Context, shareCode string, req *model.VerifyAccessReq) (*model.VerifyAccessResp, error) {
	resp, err := s.dao.VerifyAccess(ctx, shareCode, req.AccessCode)
	if err != nil {
		s.l.Error("验证访问失败", zap.Error(err))
		return nil, fmt.Errorf("验证访问失败: %w", err)
	}

	return resp, nil
}

// UpdateShare 更新分享设置
func (s *fileShareService) UpdateShare(ctx context.Context, shareID uint, req *model.UpdateShareReq) error {
	// 检查分享是否存在
	_, err := s.dao.GetShareByID(ctx, shareID)
	if err != nil {
		return fmt.Errorf("分享不存在")
	}

	if err := s.dao.UpdateShare(ctx, shareID, req); err != nil {
		s.l.Error("更新分享失败", zap.Error(err))
		return fmt.Errorf("更新分享失败: %w", err)
	}

	s.l.Info("更新分享成功", zap.Uint("share_id", shareID))
	return nil
}

// GetShareFiles 获取分享的文件列表
func (s *fileShareService) GetShareFiles(ctx context.Context, shareCode string) ([]model.FileShareItem, error) {
	items, err := s.dao.GetShareFiles(ctx, shareCode)
	if err != nil {
		s.l.Error("获取分享文件失败", zap.Error(err))
		return nil, fmt.Errorf("获取分享文件失败: %w", err)
	}

	return items, nil
}

// ListShares 获取分享列表
func (s *fileShareService) ListShares(ctx context.Context, req *model.ShareListReq) ([]model.FileShare, int64, error) {
	// 设置默认分页
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.Size <= 0 {
		req.Size = 20
	}

	shares, total, err := s.dao.ListShares(ctx, req)
	if err != nil {
		s.l.Error("获取分享列表失败", zap.Error(err))
		return nil, 0, fmt.Errorf("获取分享列表失败: %w", err)
	}

	return shares, total, nil
}

// GetShareByID 获取分享详情
func (s *fileShareService) GetShareByID(ctx context.Context, shareID uint) (*model.ShareDetailResp, error) {
	share, err := s.dao.GetShareByID(ctx, shareID)
	if err != nil {
		return nil, fmt.Errorf("分享不存在")
	}

	items, err := s.dao.GetShareItems(ctx, shareID)
	if err != nil {
		s.l.Error("获取分享项目失败", zap.Error(err))
		return nil, fmt.Errorf("获取分享项目失败: %w", err)
	}

	return &model.ShareDetailResp{
		ShareInfoResp: model.ShareInfoResp{
			ShareCode:     share.ShareCode,
			AccessLevel:   share.AccessLevel,
			CreatorID:     share.CreatorID,
			FileCount:     len(items),
			DownloadCount: share.DownloadCount,
			MaxDownloads:  share.MaxDownloads,
			ExpireAt:      share.ExpireAt,
			Status:        share.Status,
			CreatedAt:     share.CreatedAt,
		},
		Items: items,
	}, nil
}

// GetShareByCode 根据分享码获取分享信息
func (s *fileShareService) GetShareByCode(ctx context.Context, shareCode string) (*model.ShareInfoResp, error) {
	share, err := s.dao.GetShareByCode(ctx, shareCode)
	if err != nil {
		return nil, fmt.Errorf("分享不存在或已失效")
	}

	items, err := s.dao.GetShareItems(ctx, uint(share.ID))
	if err != nil {
		s.l.Error("获取分享项目失败", zap.Error(err))
		return nil, fmt.Errorf("获取分享项目失败: %w", err)
	}

	return &model.ShareInfoResp{
		ShareCode:     share.ShareCode,
		AccessLevel:   share.AccessLevel,
		CreatorID:     share.CreatorID,
		FileCount:     len(items),
		DownloadCount: share.DownloadCount,
		MaxDownloads:  share.MaxDownloads,
		ExpireAt:      share.ExpireAt,
		Status:        share.Status,
		CreatedAt:     share.CreatedAt,
	}, nil
}

// DeleteShare 删除分享
func (s *fileShareService) DeleteShare(ctx context.Context, shareID uint, userID int, isAdmin bool) error {
	share, err := s.dao.GetShareByID(ctx, shareID)
	if err != nil {
		return fmt.Errorf("分享不存在")
	}

	// 非管理员只能删除自己的分享
	if !isAdmin && share.CreatorID != userID {
		return fmt.Errorf("无权删除此分享")
	}

	if err := s.dao.DeleteShare(ctx, shareID); err != nil {
		s.l.Error("删除分享失败", zap.Error(err))
		return fmt.Errorf("删除分享失败: %w", err)
	}

	s.l.Info("删除分享成功", zap.Uint("share_id", shareID))
	return nil
}

// DownloadFile 下载文件
func (s *fileShareService) DownloadFile(ctx context.Context, shareCode, accessCode, filePath string) (string, error) {
	// 验证访问权限
	verifyResp, err := s.dao.VerifyAccess(ctx, shareCode, accessCode)
	if err != nil {
		return "", fmt.Errorf("验证访问失败: %w", err)
	}

	if !verifyResp.Valid {
		return "", fmt.Errorf("访问验证失败: %s", verifyResp.ErrorMsg)
	}

	// 获取分享项目
	items, err := s.dao.GetShareFiles(ctx, shareCode)
	if err != nil {
		return "", fmt.Errorf("获取分享文件失败: %w", err)
	}

	// 检查请求的文件是否在分享列表中
	found := false
	for _, item := range items {
		if item.FilePath == filePath {
			found = true
			break
		}
	}

	if !found {
		return "", fmt.Errorf("文件不在分享列表中")
	}

	// 验证文件路径
	if err := s.validateFilePath(filePath); err != nil {
		return "", fmt.Errorf("无效的文件路径: %w", err)
	}

	// 检查文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return "", fmt.Errorf("文件不存在")
	}

	// 增加下载次数
	if err := s.dao.IncrementDownloadCount(ctx, verifyResp.ShareID); err != nil {
		s.l.Error("更新下载次数失败", zap.Error(err))
	}

	return filePath, nil
}

// validateFilePath 验证文件路径是否在允许的根目录下
func (s *fileShareService) validateFilePath(filePath string) error {
	// 获取绝对路径
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	// 检查是否在允许的根目录下
	for _, root := range s.config.Roots {
		rootPath, err := filepath.Abs(root.Path)
		if err != nil {
			continue
		}
		if filepath.HasPrefix(absPath, rootPath) {
			return nil
		}
	}

	return fmt.Errorf("path is not under allowed roots")
}
