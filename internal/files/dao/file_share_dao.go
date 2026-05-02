/*
 * MIT License
 *
 * Copyright (c) 2024 Bamboo
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in
 * all copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
 * THE SOFTWARE.
 *
 */

package dao

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/GoSimplicity/AI-CloudOps/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type FileShareDAO interface {
	CreateShare(ctx context.Context, creatorID int, accessLevel string, maxDownloads int, expireAt interface{}, items []model.ShareItemReq) (*model.CreateShareResp, error)
	MergeShares(ctx context.Context, shareIDs []uint) (*model.CreateShareResp, error)
	VerifyAccess(ctx context.Context, shareCode, accessCode string) (*model.VerifyAccessResp, error)
	UpdateShare(ctx context.Context, shareID uint, req *model.UpdateShareReq) error
	GetShareFiles(ctx context.Context, shareCode string) ([]model.FileShareItem, error)
	ListShares(ctx context.Context, req *model.ShareListReq) ([]model.FileShare, int64, error)
	GetShareByID(ctx context.Context, shareID uint) (*model.FileShare, error)
	GetShareByCode(ctx context.Context, shareCode string) (*model.FileShare, error)
	DeleteShare(ctx context.Context, shareID uint) error
	IncrementDownloadCount(ctx context.Context, shareID uint) error
	GetShareItems(ctx context.Context, shareID uint) ([]model.FileShareItem, error)
}

type fileShareDAO struct {
	db *gorm.DB
	l  *zap.Logger
}

func NewFileShareDAO(db *gorm.DB, l *zap.Logger) FileShareDAO {
	return &fileShareDAO{
		db: db,
		l:  l,
	}
}

func (d *fileShareDAO) dbWithContext(ctx context.Context) (*gorm.DB, error) {
	if d == nil {
		return nil, fmt.Errorf("file share dao is nil")
	}
	if d.db == nil {
		if d.l != nil {
			d.l.Error("database connection is not initialized")
		}
		return nil, fmt.Errorf("database connection is not initialized")
	}
	return d.db.WithContext(ctx), nil
}

// CreateShare 创建分享链接
func (d *fileShareDAO) CreateShare(ctx context.Context, creatorID int, accessLevel string, maxDownloads int, expireAt interface{}, items []model.ShareItemReq) (*model.CreateShareResp, error) {
	db, err := d.dbWithContext(ctx)
	if err != nil {
		return nil, err
	}

	// 将 items 转换为 JSON
	itemsJSON, err := json.Marshal(items)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal items: %w", err)
	}

	var resp model.CreateShareResp
	err = db.Raw("SELECT * FROM create_file_share(?, ?, ?, ?, ?)",
		creatorID, accessLevel, maxDownloads, expireAt, string(itemsJSON)).
		Scan(&resp).Error
	if err != nil {
		return nil, fmt.Errorf("failed to create share: %w", err)
	}

	return &resp, nil
}

// MergeShares 合并分享链接
func (d *fileShareDAO) MergeShares(ctx context.Context, shareIDs []uint) (*model.CreateShareResp, error) {
	db, err := d.dbWithContext(ctx)
	if err != nil {
		return nil, err
	}

	var resp model.CreateShareResp
	err = db.Raw("SELECT * FROM merge_file_shares(?)", shareIDs).Scan(&resp).Error
	if err != nil {
		return nil, fmt.Errorf("failed to merge shares: %w", err)
	}

	return &resp, nil
}

// VerifyAccess 验证访问权限
func (d *fileShareDAO) VerifyAccess(ctx context.Context, shareCode, accessCode string) (*model.VerifyAccessResp, error) {
	db, err := d.dbWithContext(ctx)
	if err != nil {
		return nil, err
	}

	var resp model.VerifyAccessResp
	err = db.Raw("SELECT * FROM verify_share_access(?, ?)", shareCode, accessCode).Scan(&resp).Error
	if err != nil {
		return nil, fmt.Errorf("failed to verify access: %w", err)
	}

	return &resp, nil
}

// UpdateShare 更新分享设置
func (d *fileShareDAO) UpdateShare(ctx context.Context, shareID uint, req *model.UpdateShareReq) error {
	db, err := d.dbWithContext(ctx)
	if err != nil {
		return err
	}

	err = db.Raw("SELECT update_file_share(?, ?, ?, ?, ?)",
		shareID, req.AccessLevel, req.MaxDownloads, req.ExpireAt, req.Status).Error
	if err != nil {
		return fmt.Errorf("failed to update share: %w", err)
	}

	return nil
}

// GetShareFiles 获取分享的文件列表
func (d *fileShareDAO) GetShareFiles(ctx context.Context, shareCode string) ([]model.FileShareItem, error) {
	db, err := d.dbWithContext(ctx)
	if err != nil {
		return nil, err
	}

	var items []model.FileShareItem
	err = db.Raw("SELECT * FROM get_share_files(?)", shareCode).Scan(&items).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get share files: %w", err)
	}

	return items, nil
}

// ListShares 获取分享列表
func (d *fileShareDAO) ListShares(ctx context.Context, req *model.ShareListReq) ([]model.FileShare, int64, error) {
	db, err := d.dbWithContext(ctx)
	if err != nil {
		return nil, 0, err
	}

	query := db.Model(&model.FileShare{})

	// 筛选条件
	if req.Status != "" {
		query = query.Where("status = ?", req.Status)
	}
	if req.CreatorID != nil {
		query = query.Where("creator_id = ?", *req.CreatorID)
	}
	if req.AccessLevel != "" {
		query = query.Where("access_level = ?", req.AccessLevel)
	}
	if req.Search != "" {
		query = query.Where("share_code LIKE ?", "%"+req.Search+"%")
	}

	// 计算总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count shares: %w", err)
	}

	// 分页查询
	var shares []model.FileShare
	offset := (req.Page - 1) * req.Size
	if err := query.Offset(offset).Limit(req.Size).Order("created_at DESC").Find(&shares).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list shares: %w", err)
	}

	return shares, total, nil
}

// GetShareByID 根据 ID 获取分享
func (d *fileShareDAO) GetShareByID(ctx context.Context, shareID uint) (*model.FileShare, error) {
	db, err := d.dbWithContext(ctx)
	if err != nil {
		return nil, err
	}

	var share model.FileShare
	if err := db.Where("id = ?", shareID).First(&share).Error; err != nil {
		return nil, fmt.Errorf("failed to get share: %w", err)
	}

	return &share, nil
}

// GetShareByCode 根据分享码获取分享
func (d *fileShareDAO) GetShareByCode(ctx context.Context, shareCode string) (*model.FileShare, error) {
	db, err := d.dbWithContext(ctx)
	if err != nil {
		return nil, err
	}

	var share model.FileShare
	if err := db.Where("share_code = ? AND status = ?", shareCode, "active").First(&share).Error; err != nil {
		return nil, fmt.Errorf("failed to get share: %w", err)
	}

	return &share, nil
}

// DeleteShare 删除分享
func (d *fileShareDAO) DeleteShare(ctx context.Context, shareID uint) error {
	db, err := d.dbWithContext(ctx)
	if err != nil {
		return err
	}

	if err := db.Model(&model.FileShare{}).Where("id = ?", shareID).Update("status", "cancelled").Error; err != nil {
		return fmt.Errorf("failed to delete share: %w", err)
	}

	return nil
}

// IncrementDownloadCount 增加下载次数
func (d *fileShareDAO) IncrementDownloadCount(ctx context.Context, shareID uint) error {
	db, err := d.dbWithContext(ctx)
	if err != nil {
		return err
	}

	err = db.Raw("SELECT increment_download_count(?)", shareID).Error
	if err != nil {
		return fmt.Errorf("failed to increment download count: %w", err)
	}

	return nil
}

// GetShareItems 获取分享项目
func (d *fileShareDAO) GetShareItems(ctx context.Context, shareID uint) ([]model.FileShareItem, error) {
	db, err := d.dbWithContext(ctx)
	if err != nil {
		return nil, err
	}

	var items []model.FileShareItem
	if err := db.Where("share_id = ?", shareID).Find(&items).Error; err != nil {
		return nil, fmt.Errorf("failed to get share items: %w", err)
	}

	return items, nil
}
