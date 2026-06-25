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

package model

import (
	"time"

	"gorm.io/plugin/soft_delete"
)

// CronJob 定时任务
// 使用 JSON 字符串字段保存 schedule/payload/delivery/failure_alert/state，
// 避免前端协议演进时频繁修改数据库表结构。
type CronJob struct {
	ID               int                   `json:"id" gorm:"primaryKey;autoIncrement"`
	JobID            string                `json:"job_id" gorm:"type:varchar(64);uniqueIndex;not null;comment:逻辑任务ID"`
	Name             string                `json:"name" gorm:"type:varchar(200);not null;comment:任务名称"`
	Description      string                `json:"description" gorm:"type:varchar(500);comment:任务描述"`
	AgentID          string                `json:"agent_id" gorm:"type:varchar(100);comment:关联AgentID"`
	SessionKey       string                `json:"session_key" gorm:"type:varchar(200);comment:会话Key"`
	Enabled          bool                  `json:"enabled" gorm:"not null;default:true;comment:是否启用"`
	DeleteAfterRun   bool                  `json:"delete_after_run" gorm:"not null;default:false;comment:运行后是否删除"`
	ScheduleJSON     string                `json:"schedule_json" gorm:"type:text;comment:调度计划JSON"`
	SessionTarget    string                `json:"session_target" gorm:"type:varchar(50);comment:会话目标"`
	WakeMode         string                `json:"wake_mode" gorm:"type:varchar(50);comment:唤醒模式"`
	PayloadJSON      string                `json:"payload_json" gorm:"type:text;comment:执行负载JSON"`
	DeliveryJSON     string                `json:"delivery_json" gorm:"type:text;comment:投递配置JSON"`
	FailureAlertJSON string                `json:"failure_alert_json" gorm:"type:text;comment:失败告警JSON"`
	StateJSON        string                `json:"state_json" gorm:"type:text;comment:任务状态JSON"`
	NextRunAt        *time.Time            `json:"next_run_at" gorm:"index;comment:下次运行时间"`
	CreatedAt        time.Time             `json:"created_at"`
	UpdatedAt        time.Time             `json:"updated_at"`
	DeletedAt        soft_delete.DeletedAt `json:"deleted_at" gorm:"index"`
}

func (CronJob) TableName() string {
	return "cl_agent_cron_jobs"
}

// CronRun 定时任务运行日志
type CronRun struct {
	ID             int        `json:"id" gorm:"primaryKey;autoIncrement"`
	RunID          string     `json:"run_id" gorm:"type:varchar(64);uniqueIndex;not null;comment:运行ID"`
	JobID          string     `json:"job_id" gorm:"type:varchar(64);index;not null;comment:关联任务ID"`
	JobName        string     `json:"job_name" gorm:"type:varchar(200);comment:任务名称快照"`
	Action         string     `json:"action" gorm:"type:varchar(50);comment:动作"`
	Status         string     `json:"status" gorm:"type:varchar(20);index;comment:运行状态"`
	DurationMs     int64      `json:"duration_ms" gorm:"comment:耗时毫秒"`
	Error          string     `json:"error" gorm:"type:text;comment:错误信息"`
	Summary        string     `json:"summary" gorm:"type:text;comment:运行摘要"`
	Delivered      bool       `json:"delivered" gorm:"comment:是否已投递"`
	DeliveryStatus string     `json:"delivery_status" gorm:"type:varchar(30);index;comment:投递状态"`
	DeliveryError  string     `json:"delivery_error" gorm:"type:text;comment:投递错误"`
	SessionID      string     `json:"session_id" gorm:"type:varchar(200);comment:会话ID"`
	SessionKey     string     `json:"session_key" gorm:"type:varchar(200);comment:会话Key"`
	RunAt          time.Time  `json:"run_at" gorm:"comment:运行时间"`
	NextRunAt      *time.Time `json:"next_run_at" gorm:"comment:下次运行时间"`
	Model          string     `json:"model" gorm:"type:varchar(100);comment:使用模型"`
	Provider       string     `json:"provider" gorm:"type:varchar(100);comment:模型提供商"`
	UsageJSON      string     `json:"usage_json" gorm:"type:text;comment:Token用量JSON"`
	CreatedAt      time.Time  `json:"created_at"`
}

func (CronRun) TableName() string {
	return "cl_agent_cron_runs"
}
