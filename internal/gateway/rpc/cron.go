package rpc

import (
	"context"
	"encoding/json"
	"fmt"

	cronService "github.com/rizxfrog/VanPanelBackend/internal/cron/service"
	"github.com/rizxfrog/VanPanelBackend/internal/gateway"
)

var cronSvc *cronService.CronService

// SetCronService sets the CronService for gateway cron handlers.
func SetCronService(svc *cronService.CronService) {
	cronSvc = svc
}

func init() {
	gateway.RegisterMethod("cron.status", string(gateway.ScopeRead), handleCronStatus)
	gateway.RegisterMethod("cron.list", string(gateway.ScopeRead), handleCronList)
	gateway.RegisterMethod("cron.get", string(gateway.ScopeRead), handleCronGet)
	gateway.RegisterMethod("cron.add", string(gateway.ScopeRead), handleCronAdd)
	gateway.RegisterMethod("cron.update", string(gateway.ScopeRead), handleCronUpdate)
	gateway.RegisterMethod("cron.remove", string(gateway.ScopeRead), handleCronRemove)
	gateway.RegisterMethod("cron.run", string(gateway.ScopeRead), handleCronRun)
	gateway.RegisterMethod("cron.runs", string(gateway.ScopeRead), handleCronRuns)
}

type cronStatusReq struct{}

func handleCronStatus(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	if cronSvc == nil {
		return map[string]interface{}{"enabled": false, "jobs": 0, "nextWakeAtMs": nil}, nil
	}
	return cronSvc.Status(ctx)
}

type cronListReq struct {
	IncludeDisabled bool   `json:"includeDisabled"`
	Enabled         string `json:"enabled"`
	Limit           int    `json:"limit"`
	Offset          int    `json:"offset"`
	Query           string `json:"query"`
	ScheduleKind    string `json:"scheduleKind"`
	LastRunStatus   string `json:"lastRunStatus"`
	SortBy          string `json:"sortBy"`
	SortDir         string `json:"sortDir"`
}

func handleCronList(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	if cronSvc == nil {
		return map[string]interface{}{"jobs": []interface{}{}, "total": 0, "limit": 50, "offset": 0, "hasMore": false, "nextOffset": nil}, nil
	}
	var req cronListReq
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, fmt.Errorf("解析参数失败: %w", err)
	}
	if req.Limit <= 0 {
		req.Limit = 50
	}
	if req.SortBy == "" {
		req.SortBy = "nextRunAtMs"
	}
	if req.SortDir == "" {
		req.SortDir = "asc"
	}
	if req.Enabled == "" {
		req.Enabled = "all"
	}
	if req.ScheduleKind == "" {
		req.ScheduleKind = "all"
	}
	if req.LastRunStatus == "" {
		req.LastRunStatus = "all"
	}
	return cronSvc.ListJobs(ctx, cronService.ListJobsReq{
		IncludeDisabled: req.IncludeDisabled,
		Enabled:         req.Enabled,
		Query:           req.Query,
		ScheduleKind:    req.ScheduleKind,
		LastRunStatus:   req.LastRunStatus,
		SortBy:          req.SortBy,
		SortDir:         req.SortDir,
		Limit:           req.Limit,
		Offset:          req.Offset,
	})
}

type cronGetReq struct {
	ID string `json:"id"`
}

func handleCronGet(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	if cronSvc == nil {
		return map[string]interface{}{"job": nil}, nil
	}
	var req cronGetReq
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, fmt.Errorf("解析参数失败: %w", err)
	}
	job, err := cronSvc.GetJob(ctx, req.ID)
	if err != nil {
		return nil, fmt.Errorf("获取任务失败: %w", err)
	}
	return map[string]interface{}{"job": job}, nil
}

func handleCronAdd(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	if cronSvc == nil {
		return nil, fmt.Errorf("Cron 服务未初始化")
	}
	var req cronService.CreateJobReq
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, fmt.Errorf("解析参数失败: %w", err)
	}
	job, err := cronSvc.CreateJob(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("创建任务失败: %w", err)
	}
	return map[string]interface{}{"ok": true, "id": job.ID}, nil
}

type cronUpdateReq struct {
	ID    string                   `json:"id"`
	Patch cronService.CronJobPatch `json:"patch"`
}

func handleCronUpdate(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	if cronSvc == nil {
		return nil, fmt.Errorf("Cron 服务未初始化")
	}
	var req cronUpdateReq
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, fmt.Errorf("解析参数失败: %w", err)
	}
	if _, err := cronSvc.UpdateJob(ctx, cronService.UpdateJobReq{ID: req.ID, Patch: req.Patch}); err != nil {
		return nil, fmt.Errorf("更新任务失败: %w", err)
	}
	return map[string]interface{}{"ok": true}, nil
}

type cronRemoveReq struct {
	ID string `json:"id"`
}

func handleCronRemove(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	if cronSvc == nil {
		return nil, fmt.Errorf("Cron 服务未初始化")
	}
	var req cronRemoveReq
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, fmt.Errorf("解析参数失败: %w", err)
	}
	if err := cronSvc.DeleteJob(ctx, req.ID); err != nil {
		return nil, fmt.Errorf("删除任务失败: %w", err)
	}
	return map[string]interface{}{"ok": true}, nil
}

type cronRunReq struct {
	ID   string `json:"id"`
	Mode string `json:"mode"`
}

func handleCronRun(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	if cronSvc == nil {
		return nil, fmt.Errorf("Cron 服务未初始化")
	}
	var req cronRunReq
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, fmt.Errorf("解析参数失败: %w", err)
	}
	if err := cronSvc.RunJob(ctx, req.ID, req.Mode); err != nil {
		return nil, fmt.Errorf("运行任务失败: %w", err)
	}
	return map[string]interface{}{"ok": true}, nil
}

type cronRunsReq struct {
	Scope            string   `json:"scope"`
	ID               string   `json:"id"`
	Limit            int      `json:"limit"`
	Offset           int      `json:"offset"`
	Statuses         []string `json:"statuses"`
	Status           string   `json:"status"`
	DeliveryStatuses []string `json:"deliveryStatuses"`
	Query            string   `json:"query"`
	SortDir          string   `json:"sortDir"`
}

func handleCronRuns(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	if cronSvc == nil {
		return map[string]interface{}{"entries": []interface{}{}, "total": 0, "limit": 50, "offset": 0, "hasMore": false, "nextOffset": nil}, nil
	}
	var req cronRunsReq
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, fmt.Errorf("解析参数失败: %w", err)
	}
	if req.Limit <= 0 {
		req.Limit = 50
	}
	if req.Scope == "" {
		req.Scope = "all"
	}
	return cronSvc.ListRuns(ctx, cronService.ListRunsReq{
		Scope:            req.Scope,
		JobID:            req.ID,
		Limit:            req.Limit,
		Offset:           req.Offset,
		Statuses:         req.Statuses,
		Status:           req.Status,
		DeliveryStatuses: req.DeliveryStatuses,
		Query:            req.Query,
		SortDir:          req.SortDir,
	})
}
