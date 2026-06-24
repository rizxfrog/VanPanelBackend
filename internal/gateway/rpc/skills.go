package rpc

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/rizxfrog/VanPanelBackend/internal/agent/service"
	"github.com/rizxfrog/VanPanelBackend/internal/gateway"
)

var skillSvc *service.SkillService

// SetSkillService sets the SkillService for gateway skill handlers.
func SetSkillService(svc *service.SkillService) {
	skillSvc = svc
}

func init() {
	// Read methods
	gateway.RegisterMethod("skills.status", string(gateway.ScopeRead), handleSkillsStatus)
	gateway.RegisterMethod("skills.search", string(gateway.ScopeRead), handleSkillsSearch)
	gateway.RegisterMethod("skills.detail", string(gateway.ScopeRead), handleSkillsDetail)
	gateway.RegisterMethod("skills.securityVerdicts", string(gateway.ScopeRead), handleSkillsSecurityVerdicts)
	gateway.RegisterMethod("skills.skillCard", string(gateway.ScopeRead), handleSkillsSkillCard)
	gateway.RegisterMethod("skills.bins", string(gateway.ScopeRead), handleSkillsBins)
	gateway.RegisterMethod("skills.proposals.list", string(gateway.ScopeRead), handleSkillsProposalsList)
	gateway.RegisterMethod("skills.proposals.inspect", string(gateway.ScopeRead), handleSkillsProposalsInspect)

	// Admin methods
	gateway.RegisterMethod("skills.upload.begin", string(gateway.ScopeAdmin), handleSkillsUploadBegin)
	gateway.RegisterMethod("skills.upload.chunk", string(gateway.ScopeAdmin), handleSkillsUploadChunk)
	gateway.RegisterMethod("skills.upload.commit", string(gateway.ScopeAdmin), handleSkillsUploadCommit)
	gateway.RegisterMethod("skills.install", string(gateway.ScopeAdmin), handleSkillsInstall)
	gateway.RegisterMethod("skills.update", string(gateway.ScopeAdmin), handleSkillsUpdate)
	gateway.RegisterMethod("skills.proposals.create", string(gateway.ScopeAdmin), handleSkillsProposalsCreate)
	gateway.RegisterMethod("skills.proposals.update", string(gateway.ScopeAdmin), handleSkillsProposalsUpdate)
	gateway.RegisterMethod("skills.proposals.revise", string(gateway.ScopeAdmin), handleSkillsProposalsRevise)
	gateway.RegisterMethod("skills.proposals.requestRevision", string(gateway.ScopeAdmin), handleSkillsProposalsRequestRevision)
	gateway.RegisterMethod("skills.proposals.apply", string(gateway.ScopeAdmin), handleSkillsProposalsApply)
	gateway.RegisterMethod("skills.proposals.reject", string(gateway.ScopeAdmin), handleSkillsProposalsReject)
	gateway.RegisterMethod("skills.proposals.quarantine", string(gateway.ScopeAdmin), handleSkillsProposalsQuarantine)
}

type skillsStatusReq struct {
	AgentID string `json:"agentId,omitempty"`
}

func handleSkillsStatus(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	if skillSvc == nil {
		return map[string]interface{}{"status": "ok", "skills": []interface{}{}}, nil
	}
	var req skillsStatusReq
	_ = json.Unmarshal(params, &req)
	report, err := skillSvc.Status(ctx, req.AgentID)
	if err != nil {
		return nil, fmt.Errorf("获取技能状态失败: %w", err)
	}
	return report, nil
}

type skillsSearchReq struct {
	Query string `json:"query"`
	Limit int    `json:"limit"`
}

func handleSkillsSearch(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	if skillSvc == nil {
		return map[string]interface{}{"results": []interface{}{}}, nil
	}
	var req skillsSearchReq
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, fmt.Errorf("解析参数失败: %w", err)
	}
	if req.Limit <= 0 {
		req.Limit = 20
	}
	results, err := skillSvc.SearchClawHub(ctx, req.Query, req.Limit)
	if err != nil {
		return nil, fmt.Errorf("搜索 ClawHub 失败: %w", err)
	}
	return map[string]interface{}{"results": results}, nil
}

type skillsDetailReq struct {
	Slug string `json:"slug"`
}

func handleSkillsDetail(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	if skillSvc == nil {
		return map[string]interface{}{"skill": nil}, nil
	}
	var req skillsDetailReq
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, fmt.Errorf("解析参数失败: %w", err)
	}
	if req.Slug == "" {
		return nil, fmt.Errorf("slug 不能为空")
	}
	detail, err := skillSvc.GetClawHubDetail(ctx, req.Slug)
	if err != nil {
		return nil, fmt.Errorf("获取 ClawHub 详情失败: %w", err)
	}
	return detail, nil
}

func handleSkillsSecurityVerdicts(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	if skillSvc == nil {
		return map[string]interface{}{
			"schema": "agentops.skills.security-verdicts.v1",
			"items":  []interface{}{},
		}, nil
	}
	var req service.SecurityVerdictsRequest
	_ = json.Unmarshal(params, &req)
	verdicts, err := skillSvc.SecurityVerdicts(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("获取安全裁决失败: %w", err)
	}
	return verdicts, nil
}

type skillsSkillCardReq struct {
	SkillKey string `json:"skillKey"`
}

func handleSkillsSkillCard(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	if skillSvc == nil {
		return map[string]interface{}{"card": nil}, nil
	}
	var req skillsSkillCardReq
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, fmt.Errorf("解析参数失败: %w", err)
	}
	if req.SkillKey == "" {
		return nil, fmt.Errorf("skillKey 不能为空")
	}
	card, err := skillSvc.GetSkillCard(ctx, req.SkillKey)
	if err != nil {
		return nil, fmt.Errorf("获取 Skill Card 失败: %w", err)
	}
	return card, nil
}

func handleSkillsBins(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	if skillSvc == nil {
		return map[string]interface{}{}, nil
	}
	bins, err := skillSvc.Bins(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取二进制列表失败: %w", err)
	}
	return bins, nil
}

func handleSkillsUploadBegin(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]bool{"ok": true}, nil
}
func handleSkillsUploadChunk(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]bool{"ok": true}, nil
}
func handleSkillsUploadCommit(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]bool{"ok": true}, nil
}

func handleSkillsInstall(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	if skillSvc == nil {
		return map[string]bool{"ok": true}, nil
	}
	var req service.InstallSkillRequest
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, fmt.Errorf("解析参数失败: %w", err)
	}
	msg, err := skillSvc.InstallSkill(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("安装技能失败: %w", err)
	}
	return map[string]interface{}{"ok": true, "message": msg}, nil
}

func handleSkillsUpdate(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	if skillSvc == nil {
		return map[string]bool{"ok": true}, nil
	}
	var req service.UpdateSkillRequest
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, fmt.Errorf("解析参数失败: %w", err)
	}
	if req.SkillKey == "" {
		return nil, fmt.Errorf("skillKey 不能为空")
	}
	if err := skillSvc.UpdateSkill(ctx, req); err != nil {
		return nil, fmt.Errorf("更新技能失败: %w", err)
	}
	return map[string]bool{"ok": true}, nil
}

func handleSkillsProposalsList(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{
		"schema":    "agentops.skill-workshop.proposals-manifest.v1",
		"updatedAt": "",
		"proposals": []interface{}{},
	}, nil
}

type skillsProposalsInspectReq struct {
	ProposalID string `json:"proposalId"`
}

func handleSkillsProposalsInspect(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"proposal": nil}, nil
}

func handleSkillsProposalsCreate(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]bool{"ok": true}, nil
}
func handleSkillsProposalsUpdate(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]bool{"ok": true}, nil
}
func handleSkillsProposalsRevise(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]bool{"ok": true}, nil
}
func handleSkillsProposalsRequestRevision(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]bool{"ok": true}, nil
}
func handleSkillsProposalsApply(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]bool{"ok": true}, nil
}
func handleSkillsProposalsReject(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]bool{"ok": true}, nil
}
func handleSkillsProposalsQuarantine(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]bool{"ok": true}, nil
}
