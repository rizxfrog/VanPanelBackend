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
 */

package dao

import (
	"context"
	"fmt"
	"time"

	"github.com/rizxfrog/VanPanelBackend/internal/model"
	"gorm.io/gorm"
)

// CronDAO 定时任务数据访问层
type CronDAO struct {
	db *gorm.DB
}

// NewCronDAO 创建 CronDAO
func NewCronDAO(db *gorm.DB) *CronDAO {
	return &CronDAO{db: db}
}

// ListJobsReq 任务列表查询条件
type ListJobsReq struct {
	IncludeDisabled bool
	Enabled         *bool
	Query           string
	Limit           int
	Offset          int
}

// ListRunsReq 运行日志列表查询条件
type ListRunsReq struct {
	Scope            string // "all" | "job"
	JobID            string
	Statuses         []string
	DeliveryStatuses []string
	Query            string
	Limit            int
	Offset           int
}

// CreateJob 创建任务
func (d *CronDAO) CreateJob(ctx context.Context, job *model.CronJob) error {
	return d.db.WithContext(ctx).Create(job).Error
}

// UpdateJob 更新任务
func (d *CronDAO) UpdateJob(ctx context.Context, job *model.CronJob) error {
	return d.db.WithContext(ctx).Save(job).Error
}

// DeleteJob 软删除任务
func (d *CronDAO) DeleteJob(ctx context.Context, jobID string) error {
	return d.db.WithContext(ctx).Where("job_id = ?", jobID).Delete(&model.CronJob{}).Error
}

// GetJob 按 jobID 获取任务
func (d *CronDAO) GetJob(ctx context.Context, jobID string) (*model.CronJob, error) {
	var job model.CronJob
	if err := d.db.WithContext(ctx).Where("job_id = ?", jobID).First(&job).Error; err != nil {
		return nil, err
	}
	return &job, nil
}

// ListJobs 查询任务列表
// 对 enabled、query 做 SQL 过滤；scheduleKind/lastRunStatus 由 service 层过滤。
func (d *CronDAO) ListJobs(ctx context.Context, req ListJobsReq) ([]*model.CronJob, int64, error) {
	db := d.db.WithContext(ctx).Model(&model.CronJob{})
	if req.Enabled != nil {
		db = db.Where("enabled = ?", *req.Enabled)
	}
	if req.Query != "" {
		pattern := "%" + req.Query + "%"
		db = db.Where("name LIKE ? OR description LIKE ?", pattern, pattern)
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count cron jobs failed: %w", err)
	}

	var jobs []*model.CronJob
	query := db.Order("next_run_at ASC, updated_at DESC")
	if req.Limit > 0 {
		query = query.Limit(req.Limit)
	}
	if req.Offset > 0 {
		query = query.Offset(req.Offset)
	}
	if err := query.Find(&jobs).Error; err != nil {
		return nil, 0, fmt.Errorf("list cron jobs failed: %w", err)
	}
	return jobs, total, nil
}

// CreateRun 创建运行日志
func (d *CronDAO) CreateRun(ctx context.Context, run *model.CronRun) error {
	return d.db.WithContext(ctx).Create(run).Error
}

// ListRuns 查询运行日志
func (d *CronDAO) ListRuns(ctx context.Context, req ListRunsReq) ([]*model.CronRun, int64, error) {
	db := d.db.WithContext(ctx).Model(&model.CronRun{})
	if req.Scope == "job" && req.JobID != "" {
		db = db.Where("job_id = ?", req.JobID)
	}
	if len(req.Statuses) > 0 {
		db = db.Where("status IN ?", req.Statuses)
	}
	if len(req.DeliveryStatuses) > 0 {
		db = db.Where("delivery_status IN ?", req.DeliveryStatuses)
	}
	if req.Query != "" {
		pattern := "%" + req.Query + "%"
		db = db.Where("job_name LIKE ? OR summary LIKE ? OR error LIKE ?", pattern, pattern, pattern)
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count cron runs failed: %w", err)
	}

	var runs []*model.CronRun
	if err := db.Order("created_at DESC, id DESC").
		Limit(req.Limit).
		Offset(req.Offset).
		Find(&runs).Error; err != nil {
		return nil, 0, fmt.Errorf("list cron runs failed: %w", err)
	}
	return runs, total, nil
}

// UpdateRun 更新运行日志
func (d *CronDAO) UpdateRun(ctx context.Context, run *model.CronRun) error {
	return d.db.WithContext(ctx).Save(run).Error
}

// UpdateJobState 更新任务状态字段
func (d *CronDAO) UpdateJobState(ctx context.Context, jobID string, stateJSON string) error {
	return d.db.WithContext(ctx).
		Model(&model.CronJob{}).
		Where("job_id = ?", jobID).
		Update("state_json", stateJSON).Error
}

// UpdateNextRun 更新下次运行时间
func (d *CronDAO) UpdateNextRun(ctx context.Context, jobID string, nextRunAt *time.Time) error {
	return d.db.WithContext(ctx).
		Model(&model.CronJob{}).
		Where("job_id = ?", jobID).
		Update("next_run_at", nextRunAt).Error
}

// CountJobs 统计任务总数
func (d *CronDAO) CountJobs(ctx context.Context) (int64, error) {
	var total int64
	if err := d.db.WithContext(ctx).Model(&model.CronJob{}).Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

// LoadEnabledJobs 加载所有启用的任务
func (d *CronDAO) LoadEnabledJobs(ctx context.Context) ([]*model.CronJob, error) {
	var jobs []*model.CronJob
	if err := d.db.WithContext(ctx).
		Where("enabled = ?", true).
		Where("next_run_at IS NULL OR next_run_at <= ?", time.Now().Add(10*time.Second)).
		Find(&jobs).Error; err != nil {
		return nil, fmt.Errorf("load enabled cron jobs failed: %w", err)
	}
	return jobs, nil
}
