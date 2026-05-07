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

package model

import "time"

// FileShare 文件分享模型
type FileShare struct {
	Model
	ShareCode     string     `json:"share_code" gorm:"type:varchar(32);not null;uniqueIndex;comment:分享码"`
	AccessCode    string     `json:"access_code" gorm:"type:varchar(6);not null;comment:提取码"`
	CreatorID     int        `json:"creator_id" gorm:"not null;index;comment:创建者ID"`
	AccessLevel   string     `json:"access_level" gorm:"type:varchar(20);not null;default:'public';comment:访问级别 public/login_required"`
	MaxDownloads  int        `json:"max_downloads" gorm:"not null;default:0;comment:最大下载次数 0表示无限制"`
	DownloadCount int        `json:"download_count" gorm:"not null;default:0;comment:已下载次数"`
	ExpireAt      *time.Time `json:"expire_at" gorm:"index;comment:过期时间 NULL表示永久"`
	Status        string     `json:"status" gorm:"type:varchar(20);not null;default:'active';index;comment:状态 active/cancelled/expired/merged"`
}

func (fs *FileShare) TableName() string {
	return "cl_file_shares"
}

// FileShareItem 文件分享项目模型
type FileShareItem struct {
	Model
	ShareID  uint   `json:"share_id" gorm:"not null;index;comment:分享ID"`
	FilePath string `json:"file_path" gorm:"type:varchar(500);not null;comment:文件路径"`
	FileType string `json:"file_type" gorm:"type:varchar(20);not null;comment:文件类型 file/directory"`
	FileName string `json:"file_name" gorm:"type:varchar(255);not null;comment:文件名"`
}

func (fsi *FileShareItem) TableName() string {
	return "cl_file_share_items"
}

// CreateShareReq 创建分享请求
type CreateShareReq struct {
	AccessLevel  string         `json:"access_level" binding:"required,oneof=public login_required"` // 访问级别
	MaxDownloads int            `json:"max_downloads" binding:"omitempty,min=0"`                     // 最大下载次数
	ExpireAt     *time.Time     `json:"expire_at"`                                                   // 过期时间
	Items        []ShareItemReq `json:"items" binding:"required,min=1"`                              // 分享项目列表
}

// ShareItemReq 分享项目请求
type ShareItemReq struct {
	FilePath string `json:"file_path" binding:"required"`                      // 文件路径
	FileType string `json:"file_type" binding:"required,oneof=file directory"` // 文件类型
	FileName string `json:"file_name" binding:"required"`                      // 文件名
}

// UpdateShareReq 更新分享请求
type UpdateShareReq struct {
	AccessLevel  *string    `json:"access_level" binding:"omitempty,oneof=public login_required"` // 访问级别
	MaxDownloads *int       `json:"max_downloads" binding:"omitempty,min=0"`                      // 最大下载次数
	ExpireAt     *time.Time `json:"expire_at"`                                                    // 过期时间
	Status       *string    `json:"status" binding:"omitempty,oneof=active cancelled"`            // 状态
}

// MergeSharesReq 合并分享请求
type MergeSharesReq struct {
	ShareIDs []uint `json:"share_ids" binding:"required,min=2"` // 分享ID列表
}

// VerifyAccessReq 验证访问请求
type VerifyAccessReq struct {
	AccessCode string `json:"access_code" binding:"required,len=6"` // 提取码
}

// ShareListReq 分享列表请求
type ShareListReq struct {
	Page        int    `json:"page" form:"page" binding:"omitempty,min=1"`
	Size        int    `json:"size" form:"size" binding:"omitempty,min=10,max=100"`
	Search      string `json:"search" form:"search" binding:"omitempty"`
	Status      string `json:"status" form:"status" binding:"omitempty,oneof=active cancelled expired merged"`
	CreatorID   *int   `json:"creator_id" form:"creator_id" binding:"omitempty"`
	AccessLevel string `json:"access_level" form:"access_level" binding:"omitempty,oneof=public login_required"`
}

// ShareInfoResp 分享信息响应
type ShareInfoResp struct {
	ShareCode     string     `json:"share_code"`
	AccessLevel   string     `json:"access_level"`
	CreatorID     int        `json:"creator_id"`
	FileCount     int        `json:"file_count"`
	DownloadCount int        `json:"download_count"`
	MaxDownloads  int        `json:"max_downloads"`
	ExpireAt      *time.Time `json:"expire_at"`
	Status        string     `json:"status"`
	CreatedAt     time.Time  `json:"created_at"`
}

// ShareDetailResp 分享详情响应
type ShareDetailResp struct {
	ShareInfoResp
	Items []FileShareItem `json:"items"`
}

// VerifyAccessResp 验证访问响应
// 字段顺序必须与 PostgreSQL 函数 verify_share_access 返回列顺序一致 (GORM Scan 按位置映射)
type VerifyAccessResp struct {
	ShareID     uint   `json:"share_id" gorm:"column:share_id"`
	Valid       bool   `json:"valid" gorm:"column:is_valid"`
	AccessLevel string `json:"access_level" gorm:"column:access_level"`
	ErrorMsg    string `json:"error_msg" gorm:"column:error_message"`
}

// CreateShareResp 创建分享响应 (存储过程返回)
type CreateShareResp struct {
	ShareID    int    `json:"share_id" gorm:"column:share_id"`
	ShareCode  string `json:"share_code" gorm:"column:share_code"`
	AccessCode string `json:"access_code" gorm:"column:access_code"`
}
