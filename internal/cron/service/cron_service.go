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

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	agentService "github.com/rizxfrog/VanPanelBackend/internal/agent/service"
	"github.com/rizxfrog/VanPanelBackend/internal/cron/dao"
	"github.com/rizxfrog/VanPanelBackend/internal/model"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

// ==================== 类型定义（与前端协议对齐） ====================

type CronSchedule struct {
	Kind      string `json:"kind"` // "at" | "every" | "cron"
	At        string `json:"at,omitempty"`
	EveryMs   int64  `json:"everyMs,omitempty"`
	AnchorMs  int64  `json:"anchorMs,omitempty"`
	Expr      string `json:"expr,omitempty"`
	Tz        string `json:"tz,omitempty"`
	StaggerMs int64  `json:"staggerMs,omitempty"`
}

type CronPayload struct {
	Kind string `json:"kind"` // "systemEvent" | "agentTurn" | "command"
	// systemEvent
	Text string `json:"text,omitempty"`
	// agentTurn
	Message                    string   `json:"message,omitempty"`
	Model                      string   `json:"model,omitempty"`
	Fallbacks                  []string `json:"fallbacks,omitempty"`
	Thinking                   string   `json:"thinking,omitempty"`
	TimeoutSeconds             int      `json:"timeoutSeconds,omitempty"`
	AllowUnsafeExternalContent bool     `json:"allowUnsafeExternalContent,omitempty"`
	LightContext               bool     `json:"lightContext,omitempty"`
	Deliver                    bool     `json:"deliver,omitempty"`
	Channel                    string   `json:"channel,omitempty"`
	To                         string   `json:"to,omitempty"`
	BestEffortDeliver          bool     `json:"bestEffortDeliver,omitempty"`
	// command
	Argv                   []string          `json:"argv,omitempty"`
	Cwd                    string            `json:"cwd,omitempty"`
	Env                    map[string]string `json:"env,omitempty"`
	Input                  string            `json:"input,omitempty"`
	NoOutputTimeoutSeconds int               `json:"noOutputTimeoutSeconds,omitempty"`
	OutputMaxBytes         int               `json:"outputMaxBytes,omitempty"`
}

type CronDelivery struct {
	Mode       string `json:"mode"`
	Channel    string `json:"channel,omitempty"`
	To         string `json:"to,omitempty"`
	AccountID  string `json:"accountId,omitempty"`
	BestEffort bool   `json:"bestEffort,omitempty"`
}

type CronFailureAlert struct {
	After      int    `json:"after,omitempty"`
	Channel    string `json:"channel,omitempty"`
	To         string `json:"to,omitempty"`
	CooldownMs int64  `json:"cooldownMs,omitempty"`
	Mode       string `json:"mode,omitempty"`
	AccountID  string `json:"accountId,omitempty"`
}

type CronJobState struct {
	NextRunAtMs                           int64  `json:"nextRunAtMs,omitempty"`
	RunningAtMs                           int64  `json:"runningAtMs,omitempty"`
	LastRunAtMs                           int64  `json:"lastRunAtMs,omitempty"`
	LastRunStatus                         string `json:"lastRunStatus,omitempty"`
	LastStatus                            string `json:"lastStatus,omitempty"`
	LastError                             string `json:"lastError,omitempty"`
	LastErrorReason                       string `json:"lastErrorReason,omitempty"`
	LastDurationMs                        int64  `json:"lastDurationMs,omitempty"`
	ConsecutiveErrors                     int    `json:"consecutiveErrors,omitempty"`
	LastDelivered                         bool   `json:"lastDelivered,omitempty"`
	LastDeliveryStatus                    string `json:"lastDeliveryStatus,omitempty"`
	LastDeliveryError                     string `json:"lastDeliveryError,omitempty"`
	LastFailureNotificationDelivered      bool   `json:"lastFailureNotificationDelivered,omitempty"`
	LastFailureNotificationDeliveryStatus string `json:"lastFailureNotificationDeliveryStatus,omitempty"`
	LastFailureNotificationDeliveryError  string `json:"lastFailureNotificationDeliveryError,omitempty"`
	LastFailureAlertAtMs                  int64  `json:"lastFailureAlertAtMs,omitempty"`
}

type CronJob struct {
	ID             string        `json:"id"`
	Name           string        `json:"name"`
	Description    string        `json:"description,omitempty"`
	AgentID        string        `json:"agentId,omitempty"`
	SessionKey     string        `json:"sessionKey,omitempty"`
	Enabled        bool          `json:"enabled"`
	DeleteAfterRun bool          `json:"deleteAfterRun,omitempty"`
	Schedule       CronSchedule  `json:"schedule"`
	SessionTarget  string        `json:"sessionTarget"`
	WakeMode       string        `json:"wakeMode"`
	Payload        CronPayload   `json:"payload"`
	Delivery       *CronDelivery `json:"delivery,omitempty"`
	FailureAlert   interface{}   `json:"failureAlert,omitempty"`
	State          CronJobState  `json:"state,omitempty"`
	CreatedAtMs    int64         `json:"createdAtMs"`
	UpdatedAtMs    int64         `json:"updatedAtMs"`
}

type CronStatus struct {
	Enabled      bool   `json:"enabled"`
	Jobs         int64  `json:"jobs"`
	NextWakeAtMs *int64 `json:"nextWakeAtMs,omitempty"`
}

type CronRunLogEntry struct {
	Ts             int64                  `json:"ts"`
	JobID          string                 `json:"jobId"`
	Action         string                 `json:"action,omitempty"`
	Status         string                 `json:"status,omitempty"`
	DurationMs     int64                  `json:"durationMs,omitempty"`
	Error          string                 `json:"error,omitempty"`
	Summary        string                 `json:"summary,omitempty"`
	Delivered      bool                   `json:"delivered,omitempty"`
	DeliveryStatus string                 `json:"deliveryStatus,omitempty"`
	DeliveryError  string                 `json:"deliveryError,omitempty"`
	SessionID      string                 `json:"sessionId,omitempty"`
	SessionKey     string                 `json:"sessionKey,omitempty"`
	RunAtMs        int64                  `json:"runAtMs,omitempty"`
	NextRunAtMs    int64                  `json:"nextRunAtMs,omitempty"`
	Model          string                 `json:"model,omitempty"`
	Provider       string                 `json:"provider,omitempty"`
	Usage          map[string]interface{} `json:"usage,omitempty"`
	JobName        string                 `json:"jobName,omitempty"`
}

type ListJobsReq struct {
	IncludeDisabled bool
	Enabled         string // "all" | "enabled" | "disabled"
	Query           string
	ScheduleKind    string // "all" | "at" | "every" | "cron"
	LastRunStatus   string // "all" | "ok" | "error" | "skipped" | "unknown"
	SortBy          string // "nextRunAtMs" | "updatedAtMs" | "name"
	SortDir         string // "asc" | "desc"
	Limit           int
	Offset          int
}

type ListRunsReq struct {
	Scope            string // "all" | "job"
	JobID            string
	Statuses         []string
	Status           string // "all" | "ok" | "error" | "skipped"
	DeliveryStatuses []string
	Query            string
	SortDir          string // "asc" | "desc"
	Limit            int
	Offset           int
}

type CreateJobReq struct {
	CronJob
}

type UpdateJobReq struct {
	ID    string       `json:"id"`
	Patch CronJobPatch `json:"patch"`
}

type CronJobPatch struct {
	Name           string        `json:"name,omitempty"`
	Description    string        `json:"description,omitempty"`
	AgentID        *string       `json:"agentId,omitempty"`
	SessionKey     *string       `json:"sessionKey,omitempty"`
	Enabled        *bool         `json:"enabled,omitempty"`
	DeleteAfterRun *bool         `json:"deleteAfterRun,omitempty"`
	Schedule       *CronSchedule `json:"schedule,omitempty"`
	SessionTarget  string        `json:"sessionTarget,omitempty"`
	WakeMode       string        `json:"wakeMode,omitempty"`
	Payload        *CronPayload  `json:"payload,omitempty"`
	Delivery       *CronDelivery `json:"delivery,omitempty"`
	FailureAlert   interface{}   `json:"failureAlert,omitempty"`
}

type ListJobsResult struct {
	Jobs       []CronJob `json:"jobs"`
	Total      int64     `json:"total"`
	Limit      int       `json:"limit"`
	Offset     int       `json:"offset"`
	NextOffset *int      `json:"nextOffset"`
	HasMore    bool      `json:"hasMore"`
}

type ListRunsResult struct {
	Entries    []CronRunLogEntry `json:"entries"`
	Total      int64             `json:"total"`
	Limit      int               `json:"limit"`
	Offset     int               `json:"offset"`
	NextOffset *int              `json:"nextOffset"`
	HasMore    bool              `json:"hasMore"`
}

// ==================== CronService ====================

type CronService struct {
	dao         *dao.CronDAO
	agentSvc    agentService.AgentService
	logger      *zap.Logger
	asynqClient *asynq.Client
	manager     Manager
}

// Manager 是外部调度管理器的接口，用于在任务变更后触发重载
type Manager interface {
	Reload()
	NextWake() time.Time
	Enabled() bool
}

func NewCronService(dao *dao.CronDAO, agentSvc agentService.AgentService, logger *zap.Logger, asynqClient *asynq.Client) *CronService {
	return &CronService{
		dao:         dao,
		agentSvc:    agentSvc,
		logger:      logger,
		asynqClient: asynqClient,
	}
}

func (s *CronService) SetManager(m Manager) {
	s.manager = m
}

func (s *CronService) reload() {
	if s.manager != nil {
		s.manager.Reload()
	}
}

func (s *CronService) LoadDueJobs(ctx context.Context) ([]CronJob, error) {
	jobs, err := s.dao.LoadEnabledJobs(ctx)
	if err != nil {
		return nil, err
	}
	var result []CronJob
	now := time.Now()
	for _, j := range jobs {
		if j.NextRunAt != nil && j.NextRunAt.After(now) {
			continue
		}
		job, err := s.toCronJob(j)
		if err != nil {
			s.logger.Warn("解析 cron job 失败", zap.String("job_id", j.JobID), zap.Error(err))
			continue
		}
		result = append(result, job)
	}
	return result, nil
}

func (s *CronService) Status(ctx context.Context) (*CronStatus, error) {
	var status CronStatus
	status.Enabled = s.manager != nil && s.manager.Enabled()
	total, err := s.dao.CountJobs(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取任务数失败: %w", err)
	}
	status.Jobs = total
	if s.manager != nil && s.manager.Enabled() {
		next := s.manager.NextWake().UnixMilli()
		status.NextWakeAtMs = &next
	}
	return &status, nil
}

func (s *CronService) ListJobs(ctx context.Context, req ListJobsReq) (*ListJobsResult, error) {
	daoReq := dao.ListJobsReq{
		Query: req.Query,
	}
	if req.Enabled != "all" && req.Enabled != "" {
		enabled := req.Enabled == "enabled"
		daoReq.Enabled = &enabled
	}

	jobs, _, err := s.dao.ListJobs(ctx, daoReq)
	if err != nil {
		return nil, err
	}

	var result []CronJob
	for _, j := range jobs {
		job, err := s.toCronJob(j)
		if err != nil {
			s.logger.Warn("解析 cron job 失败", zap.String("job_id", j.JobID), zap.Error(err))
			continue
		}
		// 过滤 scheduleKind
		if req.ScheduleKind != "all" && req.ScheduleKind != "" {
			if job.Schedule.Kind != req.ScheduleKind {
				continue
			}
		}
		// 过滤 lastRunStatus
		if req.LastRunStatus != "all" && req.LastRunStatus != "" {
			status := resolveLastRunStatus(job.State.LastRunStatus)
			if status != req.LastRunStatus {
				continue
			}
		}
		result = append(result, job)
	}

	// 排序
	sortJobs(result, req.SortBy, req.SortDir)

	// 分页
	totalFiltered := int64(len(result))
	if req.Limit <= 0 {
		req.Limit = 50
	}
	if req.Offset < 0 {
		req.Offset = 0
	}
	end := req.Offset + req.Limit
	if end > len(result) {
		end = len(result)
	}
	page := result[req.Offset:end]
	hasMore := end < len(result)

	resp := &ListJobsResult{
		Jobs:    page,
		Total:   totalFiltered,
		Limit:   req.Limit,
		Offset:  req.Offset,
		HasMore: hasMore,
	}
	if hasMore {
		next := end
		resp.NextOffset = &next
	}
	return resp, nil
}

func resolveLastRunStatus(status string) string {
	if status == "" {
		return "unknown"
	}
	return status
}

func sortJobs(jobs []CronJob, sortBy, sortDir string) {
	asc := sortDir != "desc"
	switch sortBy {
	case "name":
		if asc {
			sort.Slice(jobs, func(i, j int) bool { return strings.ToLower(jobs[i].Name) < strings.ToLower(jobs[j].Name) })
		} else {
			sort.Slice(jobs, func(i, j int) bool { return strings.ToLower(jobs[i].Name) > strings.ToLower(jobs[j].Name) })
		}
	case "updatedAtMs":
		if asc {
			sort.Slice(jobs, func(i, j int) bool { return jobs[i].UpdatedAtMs < jobs[j].UpdatedAtMs })
		} else {
			sort.Slice(jobs, func(i, j int) bool { return jobs[i].UpdatedAtMs > jobs[j].UpdatedAtMs })
		}
	default: // nextRunAtMs
		if asc {
			sort.Slice(jobs, func(i, j int) bool {
				if jobs[i].State.NextRunAtMs == 0 && jobs[j].State.NextRunAtMs == 0 {
					return jobs[i].UpdatedAtMs < jobs[j].UpdatedAtMs
				}
				if jobs[i].State.NextRunAtMs == 0 {
					return false
				}
				if jobs[j].State.NextRunAtMs == 0 {
					return true
				}
				return jobs[i].State.NextRunAtMs < jobs[j].State.NextRunAtMs
			})
		} else {
			sort.Slice(jobs, func(i, j int) bool {
				if jobs[i].State.NextRunAtMs == 0 && jobs[j].State.NextRunAtMs == 0 {
					return jobs[i].UpdatedAtMs > jobs[j].UpdatedAtMs
				}
				if jobs[i].State.NextRunAtMs == 0 {
					return true
				}
				if jobs[j].State.NextRunAtMs == 0 {
					return false
				}
				return jobs[i].State.NextRunAtMs > jobs[j].State.NextRunAtMs
			})
		}
	}
}

func (s *CronService) GetJob(ctx context.Context, id string) (*CronJob, error) {
	job, err := s.dao.GetJob(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("获取任务失败: %w", err)
	}
	result, err := s.toCronJob(job)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *CronService) CreateJob(ctx context.Context, req CreateJobReq) (*CronJob, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("任务名称不能为空")
	}
	if err := validateSchedule(req.Schedule); err != nil {
		return nil, err
	}

	now := time.Now()
	nextRun, err := computeNextRun(req.Schedule, now)
	if err != nil {
		return nil, fmt.Errorf("计算下次运行时间失败: %w", err)
	}

	scheduleJSON, err := json.Marshal(req.Schedule)
	if err != nil {
		return nil, fmt.Errorf("序列化 schedule 失败: %w", err)
	}
	payloadJSON, err := json.Marshal(req.Payload)
	if err != nil {
		return nil, fmt.Errorf("序列化 payload 失败: %w", err)
	}
	var deliveryJSON, failureAlertJSON []byte
	if req.Delivery != nil {
		deliveryJSON, err = json.Marshal(req.Delivery)
		if err != nil {
			return nil, fmt.Errorf("序列化 delivery 失败: %w", err)
		}
	}
	if req.FailureAlert != nil {
		failureAlertJSON, err = json.Marshal(req.FailureAlert)
		if err != nil {
			return nil, fmt.Errorf("序列化 failureAlert 失败: %w", err)
		}
	}

	job := &model.CronJob{
		JobID:            uuid.New().String(),
		Name:             req.Name,
		Description:      req.Description,
		AgentID:          req.AgentID,
		SessionKey:       req.SessionKey,
		Enabled:          req.Enabled,
		DeleteAfterRun:   req.DeleteAfterRun,
		ScheduleJSON:     string(scheduleJSON),
		SessionTarget:    req.SessionTarget,
		WakeMode:         req.WakeMode,
		PayloadJSON:      string(payloadJSON),
		DeliveryJSON:     string(deliveryJSON),
		FailureAlertJSON: string(failureAlertJSON),
		NextRunAt:        nextRun,
	}
	state := CronJobState{NextRunAtMs: timeToMs(nextRun)}
	stateJSON, _ := json.Marshal(state)
	job.StateJSON = string(stateJSON)

	if err := s.dao.CreateJob(ctx, job); err != nil {
		return nil, fmt.Errorf("创建任务失败: %w", err)
	}

	s.reload()
	result, err := s.toCronJob(job)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *CronService) UpdateJob(ctx context.Context, req UpdateJobReq) (*CronJob, error) {
	if req.ID == "" {
		return nil, fmt.Errorf("任务ID不能为空")
	}
	job, err := s.dao.GetJob(ctx, req.ID)
	if err != nil {
		return nil, fmt.Errorf("获取任务失败: %w", err)
	}

	existing, err := s.toCronJob(job)
	if err != nil {
		return nil, fmt.Errorf("解析任务失败: %w", err)
	}

	scheduleChanged := false
	if req.Patch.Name != "" {
		job.Name = req.Patch.Name
	}
	if req.Patch.Description != "" {
		job.Description = req.Patch.Description
	}
	if req.Patch.AgentID != nil {
		job.AgentID = *req.Patch.AgentID
	}
	if req.Patch.SessionKey != nil {
		job.SessionKey = *req.Patch.SessionKey
	}
	if req.Patch.Enabled != nil {
		job.Enabled = *req.Patch.Enabled
	}
	if req.Patch.DeleteAfterRun != nil {
		job.DeleteAfterRun = *req.Patch.DeleteAfterRun
	}
	if req.Patch.SessionTarget != "" {
		job.SessionTarget = req.Patch.SessionTarget
	}
	if req.Patch.WakeMode != "" {
		job.WakeMode = req.Patch.WakeMode
	}
	if req.Patch.Schedule != nil {
		if err := validateSchedule(*req.Patch.Schedule); err != nil {
			return nil, err
		}
		scheduleJSON, err := json.Marshal(req.Patch.Schedule)
		if err != nil {
			return nil, fmt.Errorf("序列化 schedule 失败: %w", err)
		}
		job.ScheduleJSON = string(scheduleJSON)
		scheduleChanged = true
	}
	if req.Patch.Payload != nil {
		payloadJSON, err := json.Marshal(req.Patch.Payload)
		if err != nil {
			return nil, fmt.Errorf("序列化 payload 失败: %w", err)
		}
		job.PayloadJSON = string(payloadJSON)
	}
	if req.Patch.Delivery != nil {
		if req.Patch.Delivery.Mode == "none" {
			job.DeliveryJSON = ""
		} else {
			deliveryJSON, err := json.Marshal(req.Patch.Delivery)
			if err != nil {
				return nil, fmt.Errorf("序列化 delivery 失败: %w", err)
			}
			job.DeliveryJSON = string(deliveryJSON)
		}
	}
	if req.Patch.FailureAlert != nil {
		if v, ok := req.Patch.FailureAlert.(bool); ok && !v {
			job.FailureAlertJSON = ""
		} else {
			failureAlertJSON, err := json.Marshal(req.Patch.FailureAlert)
			if err != nil {
				return nil, fmt.Errorf("序列化 failureAlert 失败: %w", err)
			}
			job.FailureAlertJSON = string(failureAlertJSON)
		}
	}

	if scheduleChanged {
		var schedule CronSchedule
		if err := json.Unmarshal([]byte(job.ScheduleJSON), &schedule); err != nil {
			return nil, fmt.Errorf("解析 schedule 失败: %w", err)
		}
		nextRun, err := computeNextRun(schedule, time.Now())
		if err != nil {
			return nil, fmt.Errorf("计算下次运行时间失败: %w", err)
		}
		job.NextRunAt = nextRun
		existing.State.NextRunAtMs = timeToMs(nextRun)
		stateJSON, _ := json.Marshal(existing.State)
		job.StateJSON = string(stateJSON)
	}

	if err := s.dao.UpdateJob(ctx, job); err != nil {
		return nil, fmt.Errorf("更新任务失败: %w", err)
	}

	s.reload()
	result, err := s.toCronJob(job)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *CronService) DeleteJob(ctx context.Context, id string) error {
	if err := s.dao.DeleteJob(ctx, id); err != nil {
		return fmt.Errorf("删除任务失败: %w", err)
	}
	s.reload()
	return nil
}

func (s *CronService) RunJob(ctx context.Context, id string, mode string) error {
	job, err := s.dao.GetJob(ctx, id)
	if err != nil {
		return fmt.Errorf("获取任务失败: %w", err)
	}
	if !job.Enabled {
		return fmt.Errorf("任务未启用")
	}
	if s.asynqClient == nil {
		return fmt.Errorf("Asynq 客户端未初始化")
	}
	if err := s.enqueueExecuteTask(job.JobID, time.Now()); err != nil {
		return fmt.Errorf("触发任务执行失败: %w", err)
	}
	return nil
}

func (s *CronService) enqueueExecuteTask(jobID string, triggeredAt time.Time) error {
	payload := map[string]string{
		"job_id":       jobID,
		"triggered_at": triggeredAt.Format(time.RFC3339Nano),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	task := asynq.NewTask("cron:execute", data)
	_, err = s.asynqClient.Enqueue(task)
	return err
}

func (s *CronService) ListRuns(ctx context.Context, req ListRunsReq) (*ListRunsResult, error) {
	statuses := req.Statuses
	if req.Status != "" && req.Status != "all" && len(statuses) == 0 {
		statuses = []string{req.Status}
	}
	daoReq := dao.ListRunsReq{
		Scope:            req.Scope,
		JobID:            req.JobID,
		Statuses:         statuses,
		DeliveryStatuses: req.DeliveryStatuses,
		Query:            req.Query,
		Limit:            req.Limit,
		Offset:           req.Offset,
	}
	if daoReq.Limit <= 0 {
		daoReq.Limit = 50
	}

	runs, total, err := s.dao.ListRuns(ctx, daoReq)
	if err != nil {
		return nil, err
	}

	var entries []CronRunLogEntry
	for _, r := range runs {
		entries = append(entries, s.toRunEntry(r))
	}

	hasMore := len(entries) > 0 && int64(req.Offset+len(entries)) < total
	resp := &ListRunsResult{
		Entries: entries,
		Total:   total,
		Limit:   daoReq.Limit,
		Offset:  req.Offset,
		HasMore: hasMore,
	}
	if hasMore {
		next := req.Offset + len(entries)
		resp.NextOffset = &next
	}
	return resp, nil
}

func (s *CronService) ExecuteJob(ctx context.Context, jobID string, triggeredAt time.Time) error {
	start := time.Now()
	job, err := s.dao.GetJob(ctx, jobID)
	if err != nil {
		return fmt.Errorf("获取任务失败: %w", err)
	}
	if !job.Enabled {
		return nil
	}

	cronJob, err := s.toCronJob(job)
	if err != nil {
		return fmt.Errorf("解析任务失败: %w", err)
	}

	state := cronJob.State
	state.RunningAtMs = start.UnixMilli()
	state.LastRunAtMs = start.UnixMilli()
	state.LastRunStatus = "running"

	run := &model.CronRun{
		RunID:      uuid.New().String(),
		JobID:      jobID,
		JobName:    job.Name,
		Action:     "finished",
		Status:     "running",
		RunAt:      start,
		SessionKey: job.SessionKey,
	}
	if err := s.dao.CreateRun(ctx, run); err != nil {
		s.logger.Error("创建运行日志失败", zap.Error(err))
	}

	result := s.executePayload(ctx, cronJob)

	duration := time.Since(start).Milliseconds()
	status := result.Status
	state.LastRunStatus = status
	state.LastStatus = status
	state.LastDurationMs = duration
	state.LastError = result.Error
	state.LastErrorReason = result.Error
	state.RunningAtMs = 0
	state.LastDelivered = result.Delivered
	state.LastDeliveryStatus = result.DeliveryStatus
	state.LastDeliveryError = result.DeliveryError

	if status == "error" {
		state.ConsecutiveErrors++
	} else {
		state.ConsecutiveErrors = 0
	}

	// 重新计算下次运行时间
	nextRun, err := computeNextRun(cronJob.Schedule, time.Now())
	if err != nil {
		s.logger.Warn("计算下次运行时间失败", zap.String("job_id", jobID), zap.Error(err))
	}
	state.NextRunAtMs = timeToMs(nextRun)

	// 更新任务状态
	stateJSON, _ := json.Marshal(state)
	if err := s.dao.UpdateJobState(ctx, jobID, string(stateJSON)); err != nil {
		s.logger.Error("更新任务状态失败", zap.Error(err))
	}
	if err := s.dao.UpdateNextRun(ctx, jobID, nextRun); err != nil {
		s.logger.Error("更新下次运行时间失败", zap.Error(err))
	}

	// 更新运行日志
	run.Status = status
	run.DurationMs = duration
	run.Error = result.Error
	run.Summary = result.Summary
	run.Delivered = result.Delivered
	run.DeliveryStatus = result.DeliveryStatus
	run.DeliveryError = result.DeliveryError
	run.NextRunAt = nextRun
	run.Model = result.Model
	run.Provider = result.Provider
	if len(result.Usage) > 0 {
		usageJSON, _ := json.Marshal(result.Usage)
		run.UsageJSON = string(usageJSON)
	}
	if err := s.dao.UpdateRun(ctx, run); err != nil {
		s.logger.Error("更新运行日志失败", zap.Error(err))
	}

	// 运行后删除
	if cronJob.DeleteAfterRun {
		if err := s.dao.DeleteJob(ctx, jobID); err != nil {
			s.logger.Error("删除运行后任务失败", zap.Error(err))
		}
	}

	return nil
}

type executionResult struct {
	Status         string
	Error          string
	Summary        string
	Delivered      bool
	DeliveryStatus string
	DeliveryError  string
	Model          string
	Provider       string
	Usage          map[string]interface{}
}

func (s *CronService) executePayload(ctx context.Context, job CronJob) executionResult {
	res := executionResult{
		Status:         "ok",
		DeliveryStatus: "not-requested",
	}

	switch job.Payload.Kind {
	case "systemEvent":
		res.Summary = job.Payload.Text
	case "agentTurn":
		if s.agentSvc == nil {
			res.Status = "error"
			res.Error = "Agent 服务未初始化"
			return res
		}
		req := &model.AgentQueryReq{
			SessionID: job.SessionKey,
			Question:  job.Payload.Message,
		}
		resp, err := s.agentSvc.Query(ctx, req, 0)
		if err != nil {
			res.Status = "error"
			res.Error = err.Error()
			return res
		}
		res.Summary = resp.Answer
	case "command":
		res.Status = "skipped"
		res.Summary = "command 类型暂不支持"
	default:
		res.Status = "error"
		res.Error = fmt.Sprintf("未知 payload 类型: %s", job.Payload.Kind)
	}
	return res
}

// toCronJob 将 DB 模型转换为前端协议结构
func (s *CronService) toCronJob(job *model.CronJob) (CronJob, error) {
	var schedule CronSchedule
	if err := json.Unmarshal([]byte(job.ScheduleJSON), &schedule); err != nil {
		return CronJob{}, fmt.Errorf("解析 schedule 失败: %w", err)
	}
	var payload CronPayload
	if err := json.Unmarshal([]byte(job.PayloadJSON), &payload); err != nil {
		return CronJob{}, fmt.Errorf("解析 payload 失败: %w", err)
	}
	var delivery *CronDelivery
	if job.DeliveryJSON != "" {
		var d CronDelivery
		if err := json.Unmarshal([]byte(job.DeliveryJSON), &d); err != nil {
			return CronJob{}, fmt.Errorf("解析 delivery 失败: %w", err)
		}
		delivery = &d
	}
	var failureAlert interface{}
	if job.FailureAlertJSON != "" {
		var raw json.RawMessage
		if err := json.Unmarshal([]byte(job.FailureAlertJSON), &raw); err != nil {
			return CronJob{}, fmt.Errorf("解析 failureAlert 失败: %w", err)
		}
		failureAlert = raw
	}
	var state CronJobState
	if job.StateJSON != "" {
		if err := json.Unmarshal([]byte(job.StateJSON), &state); err != nil {
			// 状态解析失败不阻断
			s.logger.Warn("解析 job state 失败", zap.String("job_id", job.JobID), zap.Error(err))
		}
	}
	if job.NextRunAt != nil {
		state.NextRunAtMs = job.NextRunAt.UnixMilli()
	}

	return CronJob{
		ID:             job.JobID,
		Name:           job.Name,
		Description:    job.Description,
		AgentID:        job.AgentID,
		SessionKey:     job.SessionKey,
		Enabled:        job.Enabled,
		DeleteAfterRun: job.DeleteAfterRun,
		Schedule:       schedule,
		SessionTarget:  job.SessionTarget,
		WakeMode:       job.WakeMode,
		Payload:        payload,
		Delivery:       delivery,
		FailureAlert:   failureAlert,
		State:          state,
		CreatedAtMs:    job.CreatedAt.UnixMilli(),
		UpdatedAtMs:    job.UpdatedAt.UnixMilli(),
	}, nil
}

func (s *CronService) toRunEntry(run *model.CronRun) CronRunLogEntry {
	var usage map[string]interface{}
	if run.UsageJSON != "" {
		_ = json.Unmarshal([]byte(run.UsageJSON), &usage)
	}
	return CronRunLogEntry{
		Ts:             run.CreatedAt.UnixMilli(),
		JobID:          run.JobID,
		Action:         run.Action,
		Status:         run.Status,
		DurationMs:     run.DurationMs,
		Error:          run.Error,
		Summary:        run.Summary,
		Delivered:      run.Delivered,
		DeliveryStatus: run.DeliveryStatus,
		DeliveryError:  run.DeliveryError,
		SessionID:      run.SessionID,
		SessionKey:     run.SessionKey,
		RunAtMs:        run.RunAt.UnixMilli(),
		NextRunAtMs:    timeToMs(run.NextRunAt),
		Model:          run.Model,
		Provider:       run.Provider,
		Usage:          usage,
		JobName:        run.JobName,
	}
}

func timeToMs(t *time.Time) int64 {
	if t == nil {
		return 0
	}
	return t.UnixMilli()
}

func validateSchedule(s CronSchedule) error {
	switch s.Kind {
	case "at":
		if _, err := time.Parse(time.RFC3339Nano, s.At); err != nil {
			if _, err2 := time.Parse(time.RFC3339, s.At); err2 != nil {
				return fmt.Errorf("at 时间格式无效: %s", s.At)
			}
		}
	case "every":
		if s.EveryMs <= 0 {
			return fmt.Errorf("everyMs 必须大于 0")
		}
	case "cron":
		if s.Expr == "" {
			return fmt.Errorf("cron 表达式不能为空")
		}
	default:
		return fmt.Errorf("未知调度类型: %s", s.Kind)
	}
	return nil
}

func computeNextRun(s CronSchedule, now time.Time) (*time.Time, error) {
	switch s.Kind {
	case "at":
		t, err := time.Parse(time.RFC3339Nano, s.At)
		if err != nil {
			t, err = time.Parse(time.RFC3339, s.At)
			if err != nil {
				return nil, err
			}
		}
		if t.After(now) || t.Equal(now) {
			return &t, nil
		}
		// 一次性任务已过期，不再调度
		return nil, nil
	case "every":
		if s.EveryMs <= 0 {
			return nil, fmt.Errorf("everyMs 必须大于 0")
		}
		interval := time.Duration(s.EveryMs) * time.Millisecond
		anchor := now
		if s.AnchorMs > 0 {
			anchor = time.UnixMilli(s.AnchorMs)
		}
		elapsed := now.Sub(anchor)
		if elapsed < 0 {
			return &anchor, nil
		}
		next := anchor.Add(((elapsed / interval) + 1) * interval)
		return &next, nil
	case "cron":
		loc := time.Local
		if s.Tz != "" {
			l, err := time.LoadLocation(s.Tz)
			if err == nil {
				loc = l
			}
		}
		parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
		schedule, err := parser.Parse(s.Expr)
		if err != nil {
			return nil, fmt.Errorf("解析 cron 表达式失败: %w", err)
		}
		// robfig/cron 的 Next 接受本地时间，但返回的是按 loc 的下次时间
		localNow := now.In(loc)
		next := schedule.Next(localNow)
		if s.StaggerMs > 0 {
			stagger := time.Duration(rand.Int63n(s.StaggerMs)) * time.Millisecond
			next = next.Add(stagger)
		}
		return &next, nil
	default:
		return nil, fmt.Errorf("未知调度类型: %s", s.Kind)
	}
}
