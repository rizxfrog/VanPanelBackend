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

type CronJobStatus int8

const (
	CronJobStatusEnabled CronJobStatus = iota + 1
	CronJobStatusDisabled
	CronJobStatusRunning
	CronJobStatusError
)

type CronJobType int8

const (
	CronJobTypeSystem CronJobType = iota + 1
	CronJobTypeCommand
	CronJobTypeHTTP
	CronJobTypeScript
	CronJobTypeSSH
)

type CronJob struct {
	Model
	Name            string             `json:"name" gorm:"type:varchar(100);not null;uniqueIndex;comment:任务名称"`
	Description     string             `json:"description" gorm:"type:text;comment:任务描述"`
	JobType         CronJobType        `json:"job_type" gorm:"not null;default:2;index:idx_job_type;comment:任务类型 1系统任务 2命令行 3HTTP 4脚本 5SSH"`
	Status          CronJobStatus      `json:"status" gorm:"not null;default:1;index:idx_status;comment:任务状态 1启用 2禁用 3运行中 4错误"`
	IsBuiltIn       int8               `json:"is_built_in" gorm:"not null;default:0;index:idx_builtin;comment:是否为内置任务 1是 2否"`
	Schedule        string             `json:"schedule" gorm:"type:varchar(100);not null;comment:调度表达式"`
	Command         string             `json:"command" gorm:"type:text;comment:执行命令"`
	Args            StringList         `json:"args" gorm:"type:text;comment:命令参数"`
	WorkDir         string             `json:"work_dir" gorm:"type:varchar(500);comment:工作目录"`
	Environment     KeyValueList       `json:"environment" gorm:"type:text;comment:环境变量"`
	HTTPMethod      string             `json:"http_method" gorm:"type:varchar(10);comment:HTTP方法"`
	HTTPUrl         string             `json:"http_url" gorm:"type:varchar(500);comment:HTTP URL"`
	HTTPHeaders     KeyValueList       `json:"http_headers" gorm:"type:text;comment:HTTP请求头"`
	HTTPBody        string             `json:"http_body" gorm:"type:text;comment:HTTP请求体"`
	ScriptType      string             `json:"script_type" gorm:"type:varchar(20);comment:脚本类型"`
	ScriptContent   string             `json:"script_content" gorm:"type:text;comment:脚本内容"`
	SSHResourceID   *int               `json:"ssh_resource_id" gorm:"comment:SSH资源ID"`
	SSHResource     *TreeLocalResource `json:"ssh_resource,omitempty" gorm:"foreignKey:SSHResourceID"`
	SSHCommand      string             `json:"ssh_command" gorm:"type:text;comment:SSH执行命令"`
	SSHWorkDir      string             `json:"ssh_work_dir" gorm:"type:varchar(500);comment:SSH工作目录"`
	SSHEnvironment  KeyValueList       `json:"ssh_environment" gorm:"type:text;comment:SSH环境变量"`
	Timeout         int                `json:"timeout" gorm:"default:300;comment:超时时间秒"`
	MaxRetry        int                `json:"max_retry" gorm:"default:3;comment:最大重试次数"`
	NextRunTime     *time.Time         `json:"next_run_time" gorm:"comment:下次运行时间"`
	LastRunTime     *time.Time         `json:"last_run_time" gorm:"comment:上次运行时间"`
	LastRunStatus   int8               `json:"last_run_status" gorm:"default:0;comment:上次运行状态 0未执行 1成功 2失败"`
	LastRunDuration int                `json:"last_run_duration" gorm:"default:0;comment:上次运行时长毫秒"`
	LastRunError    string             `json:"last_run_error" gorm:"type:text;comment:上次运行错误"`
	LastRunOutput   string             `json:"last_run_output" gorm:"type:text;comment:上次运行输出"`
	RunCount        int                `json:"run_count" gorm:"default:0;comment:运行次数"`
	SuccessCount    int                `json:"success_count" gorm:"default:0;comment:成功次数"`
	FailureCount    int                `json:"failure_count" gorm:"default:0;comment:失败次数"`
	CreatedBy       int                `json:"created_by" gorm:"index:idx_created_by;comment:创建者ID"`
	CreatedByName   string             `json:"created_by_name" gorm:"type:varchar(100);comment:创建者名称"`
}

func (c *CronJob) TableName() string {
	return "cl_cron_jobs"
}

type GetCronJobListReq struct {
	ListReq
	Status    *CronJobStatus `json:"status" form:"status" binding:"omitempty,oneof=1 2 3 4"`
	JobType   *CronJobType   `json:"job_type" form:"job_type" binding:"omitempty,oneof=1 2 3 4 5"`
	IsBuiltIn *int8          `json:"is_built_in" form:"is_built_in" binding:"omitempty,oneof=0 1"`
}

type CreateCronJobReq struct {
	Name           string        `json:"name" binding:"required,min=1,max=100"`
	Description    string        `json:"description" binding:"omitempty,max=1000"`
	JobType        CronJobType   `json:"job_type" binding:"required,oneof=1 2 3 4 5"`
	Schedule       string        `json:"schedule" binding:"required"`
	Command        string        `json:"command"`
	Args           []string      `json:"args"`
	WorkDir        string        `json:"work_dir"`
	Environment    KeyValueList  `json:"environment"`
	HTTPMethod     string        `json:"http_method"`
	HTTPUrl        string        `json:"http_url"`
	HTTPHeaders    KeyValueList  `json:"http_headers"`
	HTTPBody       string        `json:"http_body"`
	ScriptType     string        `json:"script_type"`
	ScriptContent  string        `json:"script_content"`
	SSHResourceID  *int          `json:"ssh_resource_id"`
	SSHCommand     string        `json:"ssh_command"`
	SSHWorkDir     string        `json:"ssh_work_dir"`
	SSHEnvironment KeyValueList  `json:"ssh_environment"`
	Timeout        int           `json:"timeout" binding:"omitempty,min=1,max=86400"`
	MaxRetry       int           `json:"max_retry" binding:"omitempty,min=0,max=10"`
	Status         CronJobStatus `json:"status" binding:"omitempty,oneof=1 2 3 4"`
	CreatedBy      int           `json:"created_by"`
	CreatedByName  string        `json:"created_by_name"`
}

type UpdateCronJobReq struct {
	ID             int            `json:"id" binding:"required,min=1"`
	Name           string         `json:"name" binding:"omitempty,min=1,max=100"`
	Description    string         `json:"description" binding:"omitempty,max=1000"`
	JobType        CronJobType    `json:"job_type" binding:"omitempty,oneof=1 2 3 4 5"`
	Schedule       string         `json:"schedule"`
	Command        string         `json:"command"`
	Args           []string       `json:"args"`
	WorkDir        string         `json:"work_dir"`
	Environment    KeyValueList   `json:"environment"`
	HTTPMethod     string         `json:"http_method"`
	HTTPUrl        string         `json:"http_url"`
	HTTPHeaders    KeyValueList   `json:"http_headers"`
	HTTPBody       string         `json:"http_body"`
	ScriptType     string         `json:"script_type"`
	ScriptContent  string         `json:"script_content"`
	SSHResourceID  *int           `json:"ssh_resource_id"`
	SSHCommand     string         `json:"ssh_command"`
	SSHWorkDir     string         `json:"ssh_work_dir"`
	SSHEnvironment KeyValueList   `json:"ssh_environment"`
	Timeout        int            `json:"timeout" binding:"omitempty,min=1,max=86400"`
	MaxRetry       int            `json:"max_retry" binding:"omitempty,min=0,max=10"`
	Status         *CronJobStatus `json:"status" binding:"omitempty,oneof=1 2 3 4"`
}

type ValidateScheduleReq struct {
	Schedule string `json:"schedule" binding:"required"`
}

type ValidateScheduleResp struct {
	Valid        bool     `json:"valid"`
	NextRunTime  string   `json:"next_run_time,omitempty"`
	NextRunTimes []string `json:"next_run_times,omitempty"`
	Error        string   `json:"error,omitempty"`
	ErrorMessage string   `json:"error_message,omitempty"`
}
